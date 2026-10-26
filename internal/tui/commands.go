package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/inventory"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
)

type scanKind int

const (
	scanPackage scanKind = iota
	scanProject
	scanInventory
	scanCompare
	scanDemo
)

// progressMsg 携带真实扫描阶段；界面不会自行生成进度。
type progressMsg struct {
	id       int
	progress app.Progress
}

// scanResultMsg 汇总一次扫描的最终结果；kind 决定如何呈现。
type scanResultMsg struct {
	id         int
	kind       scanKind
	report     model.ScanReport
	project    project.Result
	inventory  inventory.Result
	comparison app.Comparison
	err        error
}

func (m *Model) beginScan(kind scanKind, titleKey i18n.Key, run func(context.Context, func(app.Progress)) scanResultMsg) tea.Cmd {
	m.scanID++
	id := m.scanID
	ctx, cancel := context.WithCancel(m.baseCtx)
	m.cancel = cancel
	m.screen = screenLoading
	m.loadingName = m.text(titleKey)
	m.loading = app.Progress{Stage: "start", Message: m.text(i18n.KeyLoadingPreparing)}
	m.progressLog = nil
	m.cancelling = false
	m.formError = ""
	m.errorText = ""
	// 新扫描尚未产出文档；清除旧重建闭包，避免取消或失败后按新语言重建过期报告。
	m.documentSource = nil
	m.scroll = 0
	// 加载页开始时启动不确定进度指示；真实进度仍由 app.Progress 提供。
	spinner := m.startSpinner()
	channel := m.progressCh
	runCmd := func() tea.Msg {
		result := run(ctx, func(item app.Progress) {
			select {
			case channel <- progressMsg{id: id, progress: item}:
			default:
			}
		})
		result.id = id
		result.kind = kind
		return result
	}
	return tea.Batch(runCmd, spinner)
}

func (m *Model) beginPackageScan(request app.PackageRequest) tea.Cmd {
	return m.beginScan(scanPackage, i18n.KeyLoadingCheckingPackage, func(ctx context.Context, progress func(app.Progress)) scanResultMsg {
		report, err := m.service.ScanPackage(ctx, request, progress)
		return scanResultMsg{report: report, err: err}
	})
}

func (m *Model) beginProjectScan(request app.ProjectRequest) tea.Cmd {
	return m.beginScan(scanProject, i18n.KeyLoadingScanningProject, func(ctx context.Context, progress func(app.Progress)) scanResultMsg {
		result, err := m.service.ScanProject(ctx, request, progress)
		return scanResultMsg{project: result, err: err}
	})
}

func (m *Model) beginInventoryScan() tea.Cmd {
	return m.beginScan(scanInventory, i18n.KeyLoadingReadingInventory, func(ctx context.Context, progress func(app.Progress)) scanResultMsg {
		result, err := m.service.ScanInventory(ctx, progress)
		return scanResultMsg{inventory: result, err: err}
	})
}

func (m *Model) beginCompareScan(request app.CompareRequest) tea.Cmd {
	return m.beginScan(scanCompare, i18n.KeyLoadingComparing, func(ctx context.Context, progress func(app.Progress)) scanResultMsg {
		comparison, err := m.service.Compare(ctx, request, progress)
		return scanResultMsg{comparison: comparison, err: err}
	})
}

func (m *Model) beginDemoScan(scenario string) tea.Cmd {
	return m.beginScan(scanDemo, i18n.KeyLoadingBuildingDemo, func(ctx context.Context, progress func(app.Progress)) scanResultMsg {
		report, err := m.service.Demo(ctx, scenario, progress)
		return scanResultMsg{report: report, err: err}
	})
}

// waitProgress 等待下一条真实进度消息；界面只在加载页重新订阅。
func (m *Model) waitProgress(id int) tea.Cmd {
	channel := m.progressCh
	return func() tea.Msg {
		return <-channel
	}
}
