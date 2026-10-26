package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"depscope/internal/app"
	"depscope/internal/inventory"
	"depscope/internal/model"
	"depscope/internal/project"
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

func (m *Model) beginScan(kind scanKind, title string, run func(context.Context, func(app.Progress)) scanResultMsg) tea.Cmd {
	m.scanID++
	id := m.scanID
	ctx, cancel := context.WithCancel(m.baseCtx)
	m.cancel = cancel
	m.screen = screenLoading
	m.loadingName = title
	m.loading = app.Progress{Stage: "start", Message: "Preparing read-only requests."}
	m.progressLog = nil
	m.cancelling = false
	m.formError = ""
	m.errorText = ""
	m.scroll = 0
	channel := m.progressCh
	return func() tea.Msg {
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
}

func (m *Model) beginPackageScan(request app.PackageRequest) tea.Cmd {
	return m.beginScan(scanPackage, "CHECKING PACKAGE", func(ctx context.Context, progress func(app.Progress)) scanResultMsg {
		report, err := m.service.ScanPackage(ctx, request, progress)
		return scanResultMsg{report: report, err: err}
	})
}

func (m *Model) beginProjectScan(request app.ProjectRequest) tea.Cmd {
	return m.beginScan(scanProject, "SCANNING PROJECT", func(ctx context.Context, progress func(app.Progress)) scanResultMsg {
		result, err := m.service.ScanProject(ctx, request, progress)
		return scanResultMsg{project: result, err: err}
	})
}

func (m *Model) beginInventoryScan() tea.Cmd {
	return m.beginScan(scanInventory, "READING GLOBAL INVENTORY", func(ctx context.Context, progress func(app.Progress)) scanResultMsg {
		result, err := m.service.ScanInventory(ctx, progress)
		return scanResultMsg{inventory: result, err: err}
	})
}

func (m *Model) beginCompareScan(request app.CompareRequest) tea.Cmd {
	return m.beginScan(scanCompare, "COMPARING PACKAGES", func(ctx context.Context, progress func(app.Progress)) scanResultMsg {
		comparison, err := m.service.Compare(ctx, request, progress)
		return scanResultMsg{comparison: comparison, err: err}
	})
}

func (m *Model) beginDemoScan(scenario string) tea.Cmd {
	return m.beginScan(scanDemo, "BUILDING OFFLINE DEMO", func(ctx context.Context, progress func(app.Progress)) scanResultMsg {
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
