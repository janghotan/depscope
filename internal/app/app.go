// Package app 为交互界面与非交互命令提供统一的扫描服务抽象。
// 所有风险判断、来源查询与报告组装都委托给共享引擎及既有的项目、清单扫描器。
package app

import (
	"context"
	"fmt"
	"strings"

	"depscope/internal/inventory"
	"depscope/internal/model"
	"depscope/internal/project"
)

// PackageRequest 描述一次单包扫描请求；Version 为空时解析最新版本。
type PackageRequest struct {
	Ecosystem model.Ecosystem
	Name      string
	Version   string
}

// NewPackageRequest 规范化包名并创建请求。
func NewPackageRequest(ecosystem model.Ecosystem, name, version string) (PackageRequest, error) {
	identity, err := model.NewPackageIdentity(ecosystem, name)
	if err != nil {
		return PackageRequest{}, err
	}
	version = strings.TrimSpace(version)
	if version != "" {
		if err := validateVersion(version); err != nil {
			return PackageRequest{}, err
		}
	}
	return PackageRequest{Ecosystem: identity.Ecosystem, Name: identity.Name, Version: version}, nil
}

// Validate 检查请求字段是否有效且已规范化。
func (r PackageRequest) Validate() error {
	if !r.Ecosystem.IsValid() {
		return fmt.Errorf("ecosystem %q is not supported", r.Ecosystem)
	}
	identity, err := model.NewPackageIdentity(r.Ecosystem, r.Name)
	if err != nil {
		return err
	}
	if identity.Name != r.Name {
		return fmt.Errorf("package name %q is not canonical; use %q", r.Name, identity.Name)
	}
	if r.Version != strings.TrimSpace(r.Version) {
		return fmt.Errorf("version must not contain leading or trailing whitespace")
	}
	if r.Version != "" {
		return validateVersion(r.Version)
	}
	return nil
}

// Target 将请求转换为共享引擎的扫描目标。
func (r PackageRequest) Target() (model.ScanTarget, error) {
	if err := r.Validate(); err != nil {
		return model.ScanTarget{}, err
	}
	identity, _ := model.NewPackageIdentity(r.Ecosystem, r.Name)
	target := model.ScanTarget{Package: identity}
	if r.Version != "" {
		version := r.Version
		target.ResolvedVersion = &version
	}
	return target, nil
}

// String 返回稳定的请求标签，例如 npm:pkg@1.2.3。
func (r PackageRequest) String() string {
	value := string(r.Ecosystem) + ":" + r.Name
	if r.Version != "" {
		value += "@" + r.Version
	}
	return value
}

// ProjectRequest 描述一次本地项目扫描请求。
type ProjectRequest struct {
	Path string
}

// Validate 检查项目路径非空。
func (r ProjectRequest) Validate() error {
	if strings.TrimSpace(r.Path) == "" {
		return fmt.Errorf("project path must not be empty")
	}
	return nil
}

// CompareRequest 描述两个包的对比请求。
type CompareRequest struct {
	Left  PackageRequest
	Right PackageRequest
}

// Validate 检查两侧请求。
func (r CompareRequest) Validate() error {
	if err := r.Left.Validate(); err != nil {
		return fmt.Errorf("left: %w", err)
	}
	if err := r.Right.Validate(); err != nil {
		return fmt.Errorf("right: %w", err)
	}
	return nil
}

// Progress 表示扫描过程中真实发生的阶段与计数。
type Progress struct {
	Stage     string
	Message   string
	Completed int
	Total     int
}

// DemoScenario 描述一个可运行的离线演示场景。
type DemoScenario struct {
	ID          string
	Title       string
	Description string
}

// Comparison 保存两侧独立的扫描结果；一侧失败不会抹掉另一侧的数据。
type Comparison struct {
	LeftRequest  PackageRequest
	RightRequest PackageRequest
	Left         *model.ScanReport
	LeftErr      error
	Right        *model.ScanReport
	RightErr     error
}

// Service 是交互界面与命令共用的扫描服务契约。
type Service interface {
	ScanPackage(ctx context.Context, request PackageRequest, progress func(Progress)) (model.ScanReport, error)
	ScanProject(ctx context.Context, request ProjectRequest, progress func(Progress)) (project.Result, error)
	ScanInventory(ctx context.Context, progress func(Progress)) (inventory.Result, error)
	Compare(ctx context.Context, request CompareRequest, progress func(Progress)) (Comparison, error)
	Demo(ctx context.Context, scenario string, progress func(Progress)) (model.ScanReport, error)
	DemoScenarios() []DemoScenario
}

func validateVersion(version string) error {
	for _, r := range version {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return fmt.Errorf("version %q must not contain whitespace", version)
		}
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("version %q must not contain control characters", version)
		}
	}
	return nil
}
