package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/janghotan/depscope/internal/demo"
	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/inventory"
	"github.com/janghotan/depscope/internal/live"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
)

// LiveOptions 配置共享服务；未指定的扫描器会通过 live 包装配真实来源。
type LiveOptions struct {
	Scanner         *engine.Scanner
	NPM             inventory.NPMRootFinder
	Python          inventory.PythonLister
	ProjectLimits   project.Limits
	InventoryLimits inventory.Limits
	Clock           func() time.Time
}

// LiveService 将真实来源扫描、项目扫描、全局清单与离线演示统一到 Service。
type LiveService struct {
	scanner         *engine.Scanner
	npm             inventory.NPMRootFinder
	python          inventory.PythonLister
	projectLimits   project.Limits
	inventoryLimits inventory.Limits
	clock           func() time.Time
}

// NewLiveService 创建共享服务；构造过程不会访问网络或执行包管理器命令。
func NewLiveService(opts LiveOptions) (*LiveService, error) {
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	scanner := opts.Scanner
	if scanner == nil {
		built, err := live.NewScanner(live.Options{Clock: clock})
		if err != nil {
			return nil, fmt.Errorf("create live scanner: %w", err)
		}
		scanner = built
	}
	npm := opts.NPM
	if npm == nil {
		npm = inventory.System{}
	}
	python := opts.Python
	if python == nil {
		python = inventory.System{}
	}
	return &LiveService{
		scanner:         scanner,
		npm:             npm,
		python:          python,
		projectLimits:   opts.ProjectLimits,
		inventoryLimits: opts.InventoryLimits,
		clock:           clock,
	}, nil
}

// ScanPackage 使用共享引擎扫描单个包。
func (s *LiveService) ScanPackage(ctx context.Context, request PackageRequest, progress func(Progress)) (model.ScanReport, error) {
	target, err := request.Target()
	if err != nil {
		return model.ScanReport{}, err
	}
	notify(progress, Progress{
		Stage:   "scan",
		Message: "Querying registry metadata, package metadata, and vulnerability sources.",
	})
	report, err := s.scanner.Scan(ctx, engine.ScanRequest{Target: target, DataMode: model.DataModeLive})
	if err != nil {
		return report, err
	}
	notify(progress, Progress{
		Stage:   "assemble",
		Message: "Scan finished with status " + string(report.Status) + ".",
	})
	return report, nil
}

// ScanProject 扫描本地项目根目录中的受支持依赖文件。
func (s *LiveService) ScanProject(ctx context.Context, request ProjectRequest, progress func(Progress)) (project.Result, error) {
	if err := request.Validate(); err != nil {
		return project.Result{}, err
	}
	return project.Scan(ctx, request.Path, project.Options{
		Limits:   s.projectLimits,
		Scanner:  s.scanner,
		Clock:    s.clock,
		Progress: mapProjectProgress(progress),
	})
}

// ScanInventory 只读枚举本机 npm 与 Python 全局包并匹配漏洞。
func (s *LiveService) ScanInventory(ctx context.Context, progress func(Progress)) (inventory.Result, error) {
	return inventory.Scan(ctx, inventory.Options{
		Limits:   s.inventoryLimits,
		Scanner:  s.scanner,
		Clock:    s.clock,
		Progress: mapInventoryProgress(progress),
		NPM:      s.npm,
		Python:   s.python,
	})
}

// Compare 依次扫描两个包；单侧失败会作为独立字段保留。
func (s *LiveService) Compare(ctx context.Context, request CompareRequest, progress func(Progress)) (Comparison, error) {
	if err := request.Validate(); err != nil {
		return Comparison{}, err
	}
	result := Comparison{LeftRequest: request.Left, RightRequest: request.Right}

	notify(progress, Progress{Stage: "scan", Message: "Scanning left package " + request.Left.String() + "."})
	left, leftErr := s.ScanPackage(ctx, request.Left, prefixedProgress("Left", progress))
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, ctxErr
	}
	if leftErr != nil && !errors.Is(leftErr, context.Canceled) {
		result.LeftErr = leftErr
	} else {
		result.Left = &left
	}
	if leftErr != nil && errors.Is(leftErr, context.Canceled) {
		return result, leftErr
	}

	notify(progress, Progress{Stage: "scan", Message: "Scanning right package " + request.Right.String() + "."})
	right, rightErr := s.ScanPackage(ctx, request.Right, prefixedProgress("Right", progress))
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, ctxErr
	}
	if rightErr != nil {
		result.RightErr = rightErr
	} else {
		result.Right = &right
	}
	notify(progress, Progress{Stage: "assemble", Message: "Comparison finished."})
	return result, nil
}

// Demo 使用完全离线的共享引擎生成确定性演示报告。
func (s *LiveService) Demo(ctx context.Context, scenario string, progress func(Progress)) (model.ScanReport, error) {
	scenario = trimScenario(scenario)
	notify(progress, Progress{Stage: "assemble", Message: "Building deterministic demo report for scenario " + scenario + "."})
	return demo.Scan(ctx, scenario)
}

// DemoScenarios 返回固定顺序的离线演示场景。
func (s *LiveService) DemoScenarios() []DemoScenario {
	items := demo.Scenarios()
	result := make([]DemoScenario, 0, len(items))
	for _, item := range items {
		result = append(result, DemoScenario{ID: item.ID, Title: item.Title, Description: item.Description})
	}
	return result
}

func mapProjectProgress(report func(Progress)) func(project.Progress) {
	if report == nil {
		return nil
	}
	return func(item project.Progress) {
		report(Progress{Stage: string(item.Stage), Message: item.Message, Completed: item.Completed, Total: item.Total})
	}
}

func mapInventoryProgress(report func(Progress)) func(inventory.Progress) {
	if report == nil {
		return nil
	}
	return func(item inventory.Progress) {
		report(Progress{Stage: string(item.Stage), Message: item.Message, Completed: item.Completed, Total: item.Total})
	}
}

func prefixedProgress(prefix string, report func(Progress)) func(Progress) {
	if report == nil {
		return nil
	}
	return func(item Progress) {
		item.Message = prefix + ": " + item.Message
		report(item)
	}
}

func trimScenario(scenario string) string {
	if scenario == "" {
		return "vulnerable"
	}
	return scenario
}

func notify(progress func(Progress), event Progress) {
	if progress != nil {
		progress(event)
	}
}
