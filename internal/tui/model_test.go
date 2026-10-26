package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/demo"
	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/inventory"
	domain "github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
	"github.com/janghotan/depscope/internal/render"
)

func testOptions() Options {
	return Options{
		ColorMode:        "never",
		DefaultEcosystem: domain.EcosystemNPM,
		LookupEnv:        func(string) string { return "" },
		Context:          context.Background(),
	}
}

func newTestModel(t *testing.T, service app.Service) *Model {
	t.Helper()
	model := New(service, testOptions())
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	return model
}

func send(model *Model, msg tea.Msg) tea.Cmd {
	_, cmd := model.Update(msg)
	return cmd
}

func typeText(t *testing.T, model *Model, value string) {
	t.Helper()
	for _, r := range value {
		send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestMenuNavigationOpensRequiredFlows(t *testing.T) {
	model := newTestModel(t, &fakeService{})
	view := model.View()
	for _, title := range []string{
		"Check a single package",
		"Scan my project",
		"Compare two packages",
		"Check my global installs",
		"Run the offline demo",
		"Settings",
	} {
		if !strings.Contains(view, title) {
			t.Fatalf("menu is missing %q:\n%s", title, view)
		}
	}

	send(model, tea.KeyMsg{Type: tea.KeyDown})
	if model.menuIndex != 1 {
		t.Fatalf("menu index = %d, want 1", model.menuIndex)
	}
	send(model, tea.KeyMsg{Type: tea.KeyUp})
	if model.menuIndex != 0 {
		t.Fatalf("menu index = %d, want 0", model.menuIndex)
	}

	cases := []struct {
		key     string
		screen  screen
		content string
	}{
		{key: "1", screen: screenPackageForm, content: "PACKAGE"},
		{key: "2", screen: screenProjectForm, content: "PROJECT SCAN"},
		{key: "3", screen: screenCompareForm, content: "COMPARE PACKAGES"},
		{key: "4", screen: screenInventoryConfirm, content: "GLOBAL INVENTORY"},
		{key: "5", screen: screenDemoPicker, content: "OFFLINE DEMO"},
		{key: "6", screen: screenSettings, content: "SETTINGS"},
	}
	for _, test := range cases {
		send(model, tea.KeyMsg{Type: tea.KeyEsc})
		if model.screen != screenMenu {
			t.Fatalf("screen = %v, want menu", model.screen)
		}
		send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(test.key)})
		if model.screen != test.screen {
			t.Fatalf("key %s opened screen %v, want %v", test.key, model.screen, test.screen)
		}
		if view := model.View(); !strings.Contains(view, test.content) {
			t.Fatalf("key %s view is missing %q:\n%s", test.key, test.content, view)
		}
	}
}

func TestPackageFormValidationAndCancellation(t *testing.T) {
	service := &fakeService{}
	model := newTestModel(t, service)
	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	typeText(t, model, "bad name")
	send(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.screen != screenPackageForm {
		t.Fatalf("invalid input left the form: %v", model.screen)
	}
	if model.formError == "" {
		t.Fatal("invalid input produced no actionable error")
	}

	model.formFields[0].Clear()
	typeText(t, model, "npm:fixture@1.0.0")
	send(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.screen != screenLoading {
		t.Fatalf("screen = %v, want loading", model.screen)
	}
	if model.cancel == nil {
		t.Fatal("scan did not create a cancellation function")
	}
	if service.calls != 0 {
		t.Fatal("service ran before the scan command was executed")
	}
	model.cancel()
	message := scanResultMsg{id: model.scanID, kind: scanPackage, err: context.Canceled}
	send(model, message)
	if model.screen != screenMenu {
		t.Fatalf("screen = %v, want menu after cancellation", model.screen)
	}
	if !strings.Contains(model.noticeText(), "cancelled") {
		t.Fatalf("notice = %q, want cancellation message", model.noticeText())
	}
}

func TestCancellationPropagatesToService(t *testing.T) {
	service := &fakeService{
		blockPackage: true,
		started:      make(chan struct{}, 1),
	}
	model := New(service, testOptions())
	request, err := app.NewPackageRequest(domain.EcosystemNPM, "fixture-package", "1.0.0")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	command := model.beginPackageScan(request)
	if model.screen != screenLoading || model.cancel == nil {
		t.Fatal("scan did not enter the loading state")
	}
	// beginScan 现在返回批处理命令（扫描 + 加载指示），需要依次运行其子命令。
	done := make(chan tea.Msg, 1)
	go func() {
		done <- runScanCommand(command)
	}()
	select {
	case <-service.started:
	case <-time.After(time.Second):
		t.Fatal("service was not called")
	}
	model.cancel()
	select {
	case raw := <-done:
		message, ok := raw.(scanResultMsg)
		if !ok {
			t.Fatalf("message = %T, want scanResultMsg", raw)
		}
		if !errors.Is(message.err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", message.err)
		}
		send(model, message)
		if model.screen != screenMenu {
			t.Fatalf("screen = %v, want menu", model.screen)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not stop the scan command")
	}
}

// runScanCommand 执行批处理命令中的扫描子命令，忽略动画帧命令。
// tea.Batch 返回一个 tea.Cmd，其执行结果即批处理；这里展开批处理并优先运行
// 会产生扫描结果的那一个命令。
func runScanCommand(command tea.Cmd) tea.Msg {
	if command == nil {
		return nil
	}
	first := command()
	batch, ok := first.(tea.BatchMsg)
	if !ok {
		return first
	}
	var spinnerMsg tea.Msg
	for _, sub := range batch {
		if sub == nil {
			continue
		}
		message := sub()
		if _, isTick := message.(tickMsg); isTick {
			spinnerMsg = message
			continue
		}
		return message
	}
	return spinnerMsg
}

func TestProgressMessagesUpdateLoadingState(t *testing.T) {
	model := New(&fakeService{}, testOptions())
	request, err := app.NewPackageRequest(domain.EcosystemNPM, "fixture-package", "")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	model.beginPackageScan(request)
	first := app.Progress{Stage: "metadata", Message: "Reading registry metadata.", Completed: 1, Total: 4}
	command := send(model, progressMsg{id: model.scanID, progress: first})
	if command == nil {
		t.Fatal("progress did not re-subscribe to the progress channel")
	}
	if model.loading.Completed != 1 || model.loading.Total != 4 {
		t.Fatalf("loading = %+v", model.loading)
	}
	if len(model.progressLog) != 1 {
		t.Fatalf("progress log length = %d, want 1", len(model.progressLog))
	}
	send(model, progressMsg{id: model.scanID + 1, progress: app.Progress{Stage: "stale"}})
	if model.loading.Stage != "metadata" {
		t.Fatalf("stale progress changed loading stage to %q", model.loading.Stage)
	}
	model.cancel()
}

func TestScanResultOpensReportAndViews(t *testing.T) {
	report, err := demo.Scan(context.Background(), "vulnerable")
	if err != nil {
		t.Fatalf("demo: %v", err)
	}
	model := New(&fakeService{}, testOptions())
	model.scanID = 7
	model.screen = screenLoading
	send(model, scanResultMsg{id: 7, kind: scanDemo, report: report})
	if model.screen != screenReport {
		t.Fatalf("screen = %v, want report", model.screen)
	}
	if model.document == nil || model.document.DataMode != domain.DataModeDemo {
		t.Fatal("report did not preserve demo data mode")
	}
	view := model.View()
	if !strings.Contains(view, "ASSESSMENT") || !strings.Contains(view, "DEMO DATA") {
		t.Fatalf("report view is missing key sections:\n%s", view)
	}

	send(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.reportView != viewFinding {
		t.Fatalf("report view = %v, want finding detail", model.reportView)
	}
	if view := model.View(); !strings.Contains(view, "FINDING 1 OF") {
		t.Fatalf("finding view is missing detail header:\n%s", view)
	}

	checks := []struct {
		key  rune
		view reportView
		text string
	}{
		{key: 'c', view: viewChecks, text: "CHECKS"},
		{key: 'd', view: viewDependencies, text: "DEPENDENCIES"},
		{key: 'i', view: viewDetails, text: "DETAILS"},
		{key: 's', view: viewSummary, text: "DEPSCOPE SUMMARY"},
		{key: 'v', view: viewSources, text: "SOURCES"},
	}
	for _, test := range checks {
		send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{test.key}})
		if model.reportView != test.view {
			t.Fatalf("key %q set view %v, want %v", test.key, model.reportView, test.view)
		}
		if view := model.View(); !strings.Contains(view, test.text) {
			t.Fatalf("view for %q is missing %q:\n%s", test.key, test.text, view)
		}
	}
	send(model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.reportView != viewOverview {
		t.Fatalf("report view = %v, want overview", model.reportView)
	}
	send(model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.screen != screenMenu {
		t.Fatalf("screen = %v, want menu", model.screen)
	}
}

func TestReportFindingSelection(t *testing.T) {
	report, err := demo.Scan(context.Background(), "vulnerable")
	if err != nil {
		t.Fatalf("demo: %v", err)
	}
	if len(report.Findings) < 2 {
		t.Fatalf("demo findings = %d, want at least 2", len(report.Findings))
	}
	model := New(&fakeService{}, testOptions())
	model.scanID = 1
	model.screen = screenLoading
	send(model, scanResultMsg{id: 1, kind: scanDemo, report: report})
	if model.findingIndex != 0 {
		t.Fatalf("initial finding = %d", model.findingIndex)
	}
	send(model, tea.KeyMsg{Type: tea.KeyDown})
	if model.findingIndex != 1 {
		t.Fatalf("finding index = %d, want 1", model.findingIndex)
	}
	send(model, tea.KeyMsg{Type: tea.KeyUp})
	if model.findingIndex != 0 {
		t.Fatalf("finding index = %d, want 0", model.findingIndex)
	}
}

func TestComparisonShowsUnavailableSide(t *testing.T) {
	report, err := demo.Scan(context.Background(), "vulnerable")
	if err != nil {
		t.Fatalf("demo: %v", err)
	}
	left, _ := app.NewPackageRequest(domain.EcosystemNPM, "left-package", "")
	right, _ := app.NewPackageRequest(domain.EcosystemNPM, "right-package", "")
	comparison := app.Comparison{
		LeftRequest:  left,
		RightRequest: right,
		Left:         &report,
		RightErr:     errors.New("registry unavailable"),
	}
	model := New(&fakeService{}, testOptions())
	model.scanID = 2
	model.screen = screenLoading
	send(model, scanResultMsg{id: 2, kind: scanCompare, comparison: comparison})
	if model.screen != screenComparison {
		t.Fatalf("screen = %v, want comparison", model.screen)
	}
	view := model.View()
	if !strings.Contains(view, "PACKAGE COMPARISON") || !strings.Contains(view, "UNAVAILABLE") {
		t.Fatalf("comparison view is missing side status:\n%s", view)
	}
	full := model.comparisonView()
	for _, text := range []string{"registry unavailable", "FINDINGS — LEFT", "SOURCE COVERAGE"} {
		if !strings.Contains(full, text) {
			t.Fatalf("comparison is missing %q:\n%s", text, full)
		}
	}
	send(model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.screen != screenMenu {
		t.Fatalf("screen = %v, want menu", model.screen)
	}
}

func TestDemoPickerStartsSelectedScenario(t *testing.T) {
	model := newTestModel(t, &fakeService{})
	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	if model.screen != screenDemoPicker {
		t.Fatalf("screen = %v, want demo picker", model.screen)
	}
	if len(model.demos) < 3 {
		t.Fatalf("demo scenarios = %d, want at least 3", len(model.demos))
	}
	send(model, tea.KeyMsg{Type: tea.KeyDown})
	if model.demoAt != 1 {
		t.Fatalf("demo index = %d, want 1", model.demoAt)
	}
	send(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.screen != screenLoading || model.loadingName != i18n.For(i18n.English).Text(i18n.KeyLoadingBuildingDemo) {
		t.Fatalf("screen = %v loading = %q", model.screen, model.loadingName)
	}
	model.cancel()
}

func TestInventoryRequiresConfirmation(t *testing.T) {
	model := newTestModel(t, &fakeService{})
	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	if model.screen != screenInventoryConfirm {
		t.Fatalf("screen = %v, want inventory confirmation", model.screen)
	}
	if !strings.Contains(model.View(), "read-only") {
		t.Fatal("inventory confirmation does not explain the read-only behavior")
	}
	send(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.screen != screenLoading || model.loadingName != i18n.For(i18n.English).Text(i18n.KeyLoadingReadingInventory) {
		t.Fatalf("screen = %v loading = %q", model.screen, model.loadingName)
	}
	model.cancel()
}

func TestSettingsChangeDefaultsAndDisableColor(t *testing.T) {
	model := New(&fakeService{}, Options{
		ColorMode:        "auto",
		DefaultEcosystem: domain.EcosystemNPM,
		LookupEnv:        func(string) string { return "" },
	})
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("6")})
	if model.screen != screenSettings {
		t.Fatalf("screen = %v, want settings", model.screen)
	}
	send(model, tea.KeyMsg{Type: tea.KeyDown})
	send(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.ecosystem != domain.EcosystemPyPI {
		t.Fatalf("ecosystem = %q, want pypi", model.ecosystem)
	}
	send(model, tea.KeyMsg{Type: tea.KeyDown})
	send(model, tea.KeyMsg{Type: tea.KeyRight})
	if model.colorMode != render.ColorAlways {
		t.Fatalf("color mode = %q, want always", model.colorMode)
	}
	for model.colorMode != render.ColorNever {
		send(model, tea.KeyMsg{Type: tea.KeyRight})
	}
	if model.color {
		t.Fatal("color remained enabled after selecting never")
	}
	if view := model.View(); strings.Contains(view, "\x1b[") {
		t.Fatal("color-disabled view contains ANSI escapes")
	}
}

func TestNoColorEnvironmentIsRespected(t *testing.T) {
	model := New(&fakeService{}, Options{
		ColorMode: "always",
		LookupEnv: func(key string) string {
			if key == "NO_COLOR" {
				return "1"
			}
			return ""
		},
	})
	model.Update(tea.WindowSizeMsg{Width: 90, Height: 28})
	if model.color {
		t.Fatal("NO_COLOR did not disable color")
	}
	if view := model.View(); strings.Contains(view, "\x1b[") {
		t.Fatal("NO_COLOR view contains ANSI escapes")
	}
}

func TestSmallTerminalAndHelp(t *testing.T) {
	model := newTestModel(t, &fakeService{})
	model.Update(tea.WindowSizeMsg{Width: 30, Height: 10})
	view := model.View()
	if !strings.Contains(view, "Terminal too small") {
		t.Fatalf("small terminal view is missing guidance:\n%s", view)
	}
	if strings.Contains(view, "\x1b[") {
		t.Fatal("small terminal view contains ANSI escapes")
	}

	model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if model.screen != screenHelp {
		t.Fatalf("screen = %v, want help", model.screen)
	}
	if view := model.View(); !strings.Contains(view, "KEYBOARD") {
		t.Fatalf("help view is missing keyboard guidance:\n%s", view)
	}
	send(model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.screen != screenMenu {
		t.Fatalf("screen = %v, want menu", model.screen)
	}
}

func TestProjectAndInventoryResultsOpenReports(t *testing.T) {
	projectResult := project.Result{
		Root:      "/tmp/fixture",
		Status:    domain.StatusIncomplete,
		Summary:   domain.Summary{},
		Formats:   []project.FormatInfo{{Ecosystem: domain.EcosystemNPM, Format: project.FormatNPMPackageLock, ManifestPath: "package.json"}},
		Findings:  []domain.Finding{},
		Checks:    []domain.CheckResult{{ID: "project:detection", Name: "Project detection", Status: domain.CheckSucceeded, Message: "ok", SourceIDs: []string{}}},
		Sources:   []domain.SourceReference{},
		ScannedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	model := New(&fakeService{}, testOptions())
	model.scanID = 3
	model.screen = screenLoading
	send(model, scanResultMsg{id: 3, kind: scanProject, project: projectResult})
	if model.screen != screenReport {
		t.Fatalf("project screen = %v, want report", model.screen)
	}
	if view := model.View(); !strings.Contains(view, "PROJECT REPORT") {
		t.Fatalf("project report is missing its title:\n%s", view)
	}

	inventoryResult := inventory.Result{
		Status:    domain.StatusFindings,
		Summary:   domain.Summary{},
		Packages:  []inventory.InstalledPackage{{Package: mustIdentity(t, domain.EcosystemNPM, "@fixture/global"), Version: "1.0.0", Manager: "npm"}},
		Findings:  []domain.Finding{},
		Checks:    []domain.CheckResult{{ID: "inventory:npm", Name: "npm inventory", Status: domain.CheckSucceeded, Message: "ok", SourceIDs: []string{}}},
		Sources:   []domain.SourceReference{},
		ScannedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	model.scanID = 4
	model.screen = screenLoading
	send(model, scanResultMsg{id: 4, kind: scanInventory, inventory: inventoryResult})
	if model.screen != screenReport {
		t.Fatalf("inventory screen = %v, want report", model.screen)
	}
	if view := model.View(); !strings.Contains(view, "GLOBAL INVENTORY") {
		t.Fatalf("inventory report is missing its title:\n%s", view)
	}
	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	if model.reportView != viewDetails {
		t.Fatalf("report view = %v, want details", model.reportView)
	}
	if view := model.View(); !strings.Contains(view, "@fixture/global") {
		t.Fatalf("inventory details are missing installed packages:\n%s", view)
	}
}

type fakeService struct {
	calls        int
	blockPackage bool
	started      chan struct{}
	report       domain.ScanReport
	reportErr    error
}

func (f *fakeService) ScanPackage(ctx context.Context, request app.PackageRequest, progress func(app.Progress)) (domain.ScanReport, error) {
	f.calls++
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if progress != nil {
		progress(app.Progress{Stage: "scan", Message: "Querying " + request.String() + "."})
	}
	if f.blockPackage {
		<-ctx.Done()
		return domain.ScanReport{Status: domain.StatusCancelled}, ctx.Err()
	}
	if f.reportErr != nil {
		return domain.ScanReport{}, f.reportErr
	}
	if f.report.Status == "" {
		return demo.Scan(ctx, "vulnerable")
	}
	return f.report, nil
}

func (f *fakeService) ScanProject(ctx context.Context, request app.ProjectRequest, progress func(app.Progress)) (project.Result, error) {
	f.calls++
	return project.Result{}, errors.New("project scan not configured")
}

func (f *fakeService) ScanInventory(ctx context.Context, progress func(app.Progress)) (inventory.Result, error) {
	f.calls++
	return inventory.Result{}, errors.New("inventory scan not configured")
}

func (f *fakeService) Compare(ctx context.Context, request app.CompareRequest, progress func(app.Progress)) (app.Comparison, error) {
	f.calls++
	return app.Comparison{}, errors.New("comparison not configured")
}

func (f *fakeService) Demo(ctx context.Context, scenario string, progress func(app.Progress)) (domain.ScanReport, error) {
	f.calls++
	return demo.Scan(ctx, scenario)
}

func (f *fakeService) DemoScenarios() []app.DemoScenario {
	items := demo.Scenarios()
	result := make([]app.DemoScenario, 0, len(items))
	for _, item := range items {
		result = append(result, app.DemoScenario{ID: item.ID, Title: item.Title, Description: item.Description})
	}
	return result
}

func mustIdentity(t *testing.T, ecosystem domain.Ecosystem, name string) domain.PackageIdentity {
	t.Helper()
	identity, err := domain.NewPackageIdentity(ecosystem, name)
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	return identity
}
