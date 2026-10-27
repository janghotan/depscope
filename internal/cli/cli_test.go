package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/demo"
	"github.com/janghotan/depscope/internal/inventory"
	domain "github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
)

var cliTime = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

func runCLI(t *testing.T, service app.Service, env map[string]string, tty bool, args ...string) (int, string, string) {
	t.Helper()
	var out bytes.Buffer
	var errOut bytes.Buffer
	lookup := func(key string) string { return env[key] }
	code := Run(context.Background(), Options{
		Service: service,
		IO: IO{
			In:     strings.NewReader(""),
			Out:    &out,
			Err:    &errOut,
			InTTY:  tty,
			OutTTY: tty,
			ErrTTY: tty,
		},
		Args:      args,
		Version:   "0.4.0-dev",
		LookupEnv: lookup,
	})
	return code, out.String(), errOut.String()
}

func TestNoArgsNonInteractivePrintsHelp(t *testing.T) {
	code, out, errOut := runCLI(t, nil, nil, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut)
	}
	for _, text := range []string{"Usage:", "depscope scan", "interactive", "never installs"} {
		if !strings.Contains(out, text) {
			t.Fatalf("help output is missing %q:\n%s", text, out)
		}
	}
	if errOut != "" {
		t.Fatalf("unexpected stderr: %s", errOut)
	}
}

func TestHelpVersionAndFlagErrors(t *testing.T) {
	code, out, _ := runCLI(t, nil, nil, false, "--help")
	if code != 0 || !strings.Contains(out, "Usage:") {
		t.Fatalf("--help code = %d output = %q", code, out)
	}
	code, out, _ = runCLI(t, nil, nil, false, "--version")
	if code != 0 || !strings.Contains(out, "depscope 0.4.0-dev") {
		t.Fatalf("--version code = %d output = %q", code, out)
	}
	code, _, errOut := runCLI(t, nil, nil, false, "--color=nope", "scan")
	if code != 2 || !strings.Contains(errOut, "color mode") {
		t.Fatalf("invalid color code = %d stderr = %q", code, errOut)
	}
	code, _, errOut = runCLI(t, nil, nil, false, "--bogus")
	if code != 2 || !strings.Contains(errOut, "unknown flag") {
		t.Fatalf("unknown flag code = %d stderr = %q", code, errOut)
	}
	code, _, errOut = runCLI(t, nil, nil, false, "bogus")
	if code != 2 || !strings.Contains(errOut, "unknown command") {
		t.Fatalf("unknown command code = %d stderr = %q", code, errOut)
	}
}

func TestScanUsageErrors(t *testing.T) {
	code, _, errOut := runCLI(t, nil, nil, false, "scan")
	if code != 2 || !strings.Contains(errOut, "usage: depscope scan") {
		t.Fatalf("code = %d stderr = %q", code, errOut)
	}
	code, _, errOut = runCLI(t, nil, nil, false, "scan", "one", "two")
	if code != 2 || !strings.Contains(errOut, "usage: depscope scan") {
		t.Fatalf("code = %d stderr = %q", code, errOut)
	}
	code, _, errOut = runCLI(t, &fakeService{}, nil, false, "scan", "bad name")
	if code != 2 || !strings.Contains(errOut, "invalid package") {
		t.Fatalf("code = %d stderr = %q", code, errOut)
	}
}

func TestMCPCLICommandValidation(t *testing.T) {
	// 1. 无效选项
	code, _, errOut := runCLI(t, &fakeService{}, nil, false, "mcp", "--bogus")
	if code != 2 || !strings.Contains(errOut, "unknown flag") {
		t.Fatalf("code = %d stderr = %q", code, errOut)
	}

	// 2. 缺少 --allowed-path 参数值
	code, _, errOut = runCLI(t, &fakeService{}, nil, false, "mcp", "--allowed-path")
	if code != 2 || !strings.Contains(errOut, "requires a directory path") {
		t.Fatalf("code = %d stderr = %q", code, errOut)
	}
}

func TestScanPlainOutputDisablesColor(t *testing.T) {
	service := &fakeService{report: liveReport(t, domain.StatusFindings)}
	code, out, errOut := runCLI(t, service, nil, false, "--no-color", "scan", "npm:fixture-package")
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, errOut)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatal("plain scan output contains ANSI escapes")
	}
	for _, text := range []string{"PACKAGE REPORT", "npm:fixture-package", "FINDINGS", "Fixture advisory", "NEXT ACTIONS"} {
		if !strings.Contains(out, text) {
			t.Fatalf("scan output is missing %q:\n%s", text, out)
		}
	}
}

func TestColorAlwaysAndNOColor(t *testing.T) {
	service := &fakeService{report: liveReport(t, domain.StatusFindings)}
	code, out, _ := runCLI(t, service, nil, false, "--color=always", "scan", "npm:fixture-package")
	if code != 0 || !strings.Contains(out, "\x1b[") {
		t.Fatalf("--color=always code = %d color = %v", code, strings.Contains(out, "\x1b["))
	}
	code, out, _ = runCLI(t, service, map[string]string{"NO_COLOR": "1"}, false, "--color=always", "scan", "npm:fixture-package")
	if code != 0 || strings.Contains(out, "\x1b[") {
		t.Fatalf("NO_COLOR override code = %d ansi = %v", code, strings.Contains(out, "\x1b["))
	}
}

func TestScanErrorExitCodeAndSanitization(t *testing.T) {
	service := &fakeService{scanErr: errors.New("registry unavailable \x1b[31mred\x1b[0m")}
	code, out, errOut := runCLI(t, service, nil, false, "scan", "npm:fixture-package")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if out != "" {
		t.Fatalf("unexpected stdout: %q", out)
	}
	if strings.Contains(errOut, "\x1b[") {
		t.Fatal("error output contains ANSI escapes")
	}
	if !strings.Contains(errOut, "registry unavailable") {
		t.Fatalf("error output is missing the message: %q", errOut)
	}
}

func TestDemoIsDeterministicAndOffline(t *testing.T) {
	service := &app.LiveService{}
	code, first, errOut := runCLI(t, service, nil, false, "--no-color", "demo", "clean")
	if code != 0 {
		t.Fatalf("demo exit code = %d (stderr: %s)", code, errOut)
	}
	code, second, _ := runCLI(t, service, nil, false, "--no-color", "demo", "clean")
	if code != 0 {
		t.Fatalf("second demo exit code = %d", code)
	}
	if first != second {
		t.Fatal("demo output is not deterministic")
	}
	for _, text := range []string{"DEMO DATA", "CLEAN", "demo:registry"} {
		if !strings.Contains(first, text) {
			t.Fatalf("demo output is missing %q:\n%s", text, first)
		}
	}
	code, out, _ := runCLI(t, service, nil, false, "--no-color", "demo")
	if code != 0 || !strings.Contains(out, "DEMO DATA") || !strings.Contains(out, "FINDINGS") {
		t.Fatalf("default demo code = %d output = %q", code, out)
	}
}

// TestCLIOutputStaysEnglish 确保非交互式输出不随 TUI 语言设置变化。
func TestCLIOutputStaysEnglish(t *testing.T) {
	service := &app.LiveService{}
	code, out, errOut := runCLI(t, service, nil, false, "--no-color", "demo", "vulnerable")
	if code != 0 {
		t.Fatalf("demo exit code = %d (stderr: %s)", code, errOut)
	}
	for _, text := range []string{"DEMO DATA", "ASSESSMENT", "FINDINGS", "SOURCES"} {
		if !strings.Contains(out, text) {
			t.Fatalf("English CLI output is missing %q:\n%s", text, out)
		}
	}
	for _, r := range out {
		if unicode.Is(unicode.Han, r) {
			t.Fatalf("English CLI output contains Chinese text %q:\n%s", r, out)
		}
	}
}

func TestDemoListAndUnknownScenario(t *testing.T) {
	service := &app.LiveService{}
	code, out, _ := runCLI(t, service, nil, false, "demo", "list")
	if code != 0 {
		t.Fatalf("demo list code = %d", code)
	}
	for _, id := range []string{"vulnerable", "install-scripts", "clean"} {
		if !strings.Contains(out, id) {
			t.Fatalf("demo list is missing %q:\n%s", id, out)
		}
	}
	code, _, errOut := runCLI(t, service, nil, false, "demo", "nope")
	if code != 2 || !strings.Contains(errOut, "unknown demo scenario") {
		t.Fatalf("unknown scenario code = %d stderr = %q", code, errOut)
	}
}

func TestCompareKeepsUnavailableSide(t *testing.T) {
	report := liveReport(t, domain.StatusFindings)
	left, _ := app.NewPackageRequest(domain.EcosystemNPM, "left-package", "")
	right, _ := app.NewPackageRequest(domain.EcosystemNPM, "right-package", "")
	service := &fakeService{comparison: app.Comparison{
		LeftRequest:  left,
		RightRequest: right,
		Left:         &report,
		RightErr:     errors.New("registry unavailable"),
	}}
	code, out, errOut := runCLI(t, service, nil, false, "--no-color", "compare", "npm:left-package", "npm:right-package")
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, errOut)
	}
	for _, text := range []string{"PACKAGE COMPARISON", "UNAVAILABLE", "registry unavailable", "FINDINGS — LEFT"} {
		if !strings.Contains(out, text) {
			t.Fatalf("comparison is missing %q:\n%s", text, out)
		}
	}
}

func TestCompareBothUnavailableReturnsFailure(t *testing.T) {
	left, _ := app.NewPackageRequest(domain.EcosystemNPM, "left-package", "")
	right, _ := app.NewPackageRequest(domain.EcosystemNPM, "right-package", "")
	service := &fakeService{comparison: app.Comparison{
		LeftRequest:  left,
		RightRequest: right,
		LeftErr:      errors.New("left failed"),
		RightErr:     errors.New("right failed"),
	}}
	code, out, _ := runCLI(t, service, nil, false, "--no-color", "compare", "npm:left-package", "npm:right-package")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out, "UNAVAILABLE") {
		t.Fatalf("comparison did not explain unavailable sides:\n%s", out)
	}
}

func TestProjectAndInventoryCommands(t *testing.T) {
	service := &fakeService{
		projectResult: project.Result{
			Root:      "/tmp/fixture",
			Status:    domain.StatusIncomplete,
			Formats:   []project.FormatInfo{},
			Findings:  []domain.Finding{},
			Checks:    []domain.CheckResult{},
			Sources:   []domain.SourceReference{},
			Summary:   domain.Summary{},
			ScannedAt: cliTime,
		},
		inventoryResult: inventory.Result{
			Status:    domain.StatusClean,
			Findings:  []domain.Finding{},
			Checks:    []domain.CheckResult{},
			Sources:   []domain.SourceReference{},
			Summary:   domain.Summary{},
			ScannedAt: cliTime,
		},
	}
	code, out, errOut := runCLI(t, service, nil, false, "--no-color", "project", "/tmp/fixture")
	if code != 0 {
		t.Fatalf("project code = %d (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "PROJECT REPORT") || !strings.Contains(out, "INCOMPLETE") {
		t.Fatalf("project output is missing status:\n%s", out)
	}
	code, out, errOut = runCLI(t, service, nil, false, "--no-color", "inventory")
	if code != 0 {
		t.Fatalf("inventory code = %d (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "GLOBAL INVENTORY") {
		t.Fatalf("inventory output is missing title:\n%s", out)
	}
}

func TestDumbTerminalFallsBackToHelp(t *testing.T) {
	code, out, _ := runCLI(t, nil, map[string]string{"TERM": "dumb"}, true)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("dumb terminal did not print help:\n%s", out)
	}
}

type fakeService struct {
	report          domain.ScanReport
	scanErr         error
	comparison      app.Comparison
	compareErr      error
	projectResult   project.Result
	projectErr      error
	inventoryResult inventory.Result
	inventoryErr    error
}

func (f *fakeService) ScanPackage(context.Context, app.PackageRequest, func(app.Progress)) (domain.ScanReport, error) {
	return f.report, f.scanErr
}

func (f *fakeService) ScanProject(context.Context, app.ProjectRequest, func(app.Progress)) (project.Result, error) {
	return f.projectResult, f.projectErr
}

func (f *fakeService) ScanInventory(context.Context, func(app.Progress)) (inventory.Result, error) {
	return f.inventoryResult, f.inventoryErr
}

func (f *fakeService) Compare(context.Context, app.CompareRequest, func(app.Progress)) (app.Comparison, error) {
	return f.comparison, f.compareErr
}

func (f *fakeService) Demo(ctx context.Context, scenario string, _ func(app.Progress)) (domain.ScanReport, error) {
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

func liveReport(t *testing.T, status domain.ScanStatus) domain.ScanReport {
	t.Helper()
	version := "1.2.3"
	identity, err := domain.NewPackageIdentity(domain.EcosystemNPM, "fixture-package")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	severity := domain.SeverityHigh
	remediation := "Upgrade to 1.2.4 or later."
	findings := []domain.Finding{}
	if status == domain.StatusFindings {
		findings = append(findings, domain.Finding{
			ID:          "FIXTURE-2026-0001",
			Kind:        domain.FindingConfirmed,
			Title:       "Fixture advisory",
			Description: "A synthetic advisory used only by CLI formatting tests.",
			Severity:    severity,
			Package:     identity,
			Version:     &version,
			Evidence: []domain.Evidence{{
				Kind:      domain.EvidenceAdvisory,
				Summary:   "Synthetic evidence.",
				SourceID:  "fixture:npm",
				Reference: "FIXTURE-2026-0001",
			}},
			Remediation: &remediation,
		})
	}
	return domain.ScanReport{
		SchemaVersion: domain.SchemaVersion,
		DataMode:      domain.DataModeLive,
		ScannedAt:     cliTime,
		Status:        status,
		Target: domain.ScanTarget{
			Package:         identity,
			ResolvedVersion: &version,
		},
		Summary: domain.Summary{
			ConfirmedFindings: len(findings),
			HeuristicWarnings: 0,
			HighestSeverity:   &severity,
		},
		Findings: findings,
		Checks: []domain.CheckResult{{
			ID:        "npm:metadata",
			Name:      "npm metadata",
			Status:    domain.CheckSucceeded,
			Message:   "Registry metadata was available.",
			SourceIDs: []string{"fixture:npm"},
		}},
		Dependencies: domain.DependencyGraph{
			RootNodeID: "root",
			Nodes: []domain.DependencyNode{{
				ID:             "root",
				Package:        identity,
				Version:        &version,
				Relationship:   domain.DependencyRoot,
				DependencyPath: []string{"fixture-package"},
				Dependencies:   []string{},
			}},
		},
		Sources: []domain.SourceReference{{
			ID:          "fixture:npm",
			Provider:    domain.SourceProviderNPM,
			Identifier:  "fixture-registry",
			RetrievedAt: cliTime,
		}},
		Limitations: []domain.Limitation{},
	}
}
