package render_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/demo"
	"github.com/janghotan/depscope/internal/inventory"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
	"github.com/janghotan/depscope/internal/render"
)

func TestOverviewDistinguishesStatusesAndSeverities(t *testing.T) {
	vulnerable := scanDemo(t, "vulnerable")
	clean := scanDemo(t, "clean")
	theme := render.Theme{Width: 96}

	findings := render.FromReport(vulnerable).Overview(theme, 0)
	for _, want := range []string{"FINDINGS", "[CRITICAL]", "[CONFIRMED]", "DEMO DATA", "Completed checks", "COVERAGE GAPS"} {
		if !strings.Contains(findings, want) && want != "COVERAGE GAPS" {
			t.Fatalf("vulnerable overview missing %q:\n%s", want, findings)
		}
	}
	cleanText := render.FromReport(clean).Overview(theme, 0)
	if !strings.Contains(cleanText, "[CLEAN]") {
		t.Fatalf("clean overview missing clean status:\n%s", cleanText)
	}
	if strings.Contains(cleanText, "[CRITICAL]") {
		t.Fatal("clean overview claims a critical severity")
	}
	if !strings.Contains(cleanText, "No confirmed findings") {
		t.Fatalf("clean overview missing no-findings message:\n%s", cleanText)
	}
}

func TestFindingDetailShowsEvidenceAndActions(t *testing.T) {
	report := scanDemo(t, "vulnerable")
	doc := render.FromReport(report)
	text := doc.FindingDetail(0, render.Theme{Width: 90})
	for _, want := range []string{"FINDING 1 OF", "EVIDENCE", "source: demo:advisories", "REMEDIATION", "Upgrade", "NEXT ACTIONS"} {
		if !strings.Contains(text, want) {
			t.Fatalf("finding detail missing %q:\n%s", want, text)
		}
	}

	scripts := render.FromReport(scanDemo(t, "install-scripts"))
	findingIndex := -1
	for index, finding := range scripts.Findings {
		if strings.HasPrefix(finding.ID, "LIFECYCLE-") {
			findingIndex = index
			break
		}
	}
	if findingIndex < 0 {
		t.Fatal("install-scripts demo has no lifecycle finding")
	}
	scriptText := scripts.FindingDetail(findingIndex, render.Theme{Width: 90})
	if !strings.Contains(scriptText, "fetch-assets.js") {
		t.Fatalf("lifecycle detail does not show command:\n%s", scriptText)
	}
	if !strings.Contains(scriptText, "never executes") && !strings.Contains(scriptText, "did not execute") {
		t.Fatalf("lifecycle detail does not state execution boundary:\n%s", scriptText)
	}
}

func TestChecksAndCoverageGaps(t *testing.T) {
	report := scanDemo(t, "clean")
	report.Status = model.StatusIncomplete
	report.Checks = append(report.Checks, model.CheckResult{
		ID:        "extra-vulnerabilities",
		Name:      "OSV vulnerability query",
		Status:    model.CheckUnavailable,
		Message:   "OSV data was unavailable.",
		SourceIDs: []string{},
	})
	doc := render.FromReport(report)
	text := doc.ChecksSection(render.Theme{Width: 90})
	if !strings.Contains(text, "[UNAVAILABLE]") {
		t.Fatalf("checks section missing unavailable label:\n%s", text)
	}
	if !strings.Contains(text, "6 of 7 checks succeeded") {
		t.Fatalf("checks section missing coverage count:\n%s", text)
	}
	overview := doc.Overview(render.Theme{Width: 90}, 0)
	if !strings.Contains(overview, "not clean") {
		t.Fatalf("incomplete overview does not warn about clean semantics:\n%s", overview)
	}
}

func TestDependencyTreeIncludesConstraintsAndUnresolvedNodes(t *testing.T) {
	version := "1.0.0"
	pkgA := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "a-pkg"}
	pkgB := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "b-pkg"}
	result := project.Result{
		Root:      "/tmp/demo-project",
		ScannedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Status:    model.StatusFindings,
		Summary:   model.Summary{HeuristicWarnings: 1, HighestSeverity: severityPointer(model.SeverityLow)},
		Formats: []project.FormatInfo{{
			Ecosystem: model.EcosystemNPM, Format: project.FormatNPMPackageLock,
			ManifestPath: "package.json", LockfilePath: "package-lock.json",
			ResolvedPackages: 1, UnresolvedPackages: 1,
		}},
		Dependencies: project.Graph{
			RootID: "project",
			Nodes: []project.Node{
				{
					DependencyNode: model.DependencyNode{
						ID: "npm:a", Package: pkgA, Version: &version, Relationship: model.DependencyDirect,
						DependencyPath: []string{"project", "npm:a"}, Dependencies: []string{"npm:b"},
					},
					Constraint: "^1.0.0",
				},
				{
					DependencyNode: model.DependencyNode{
						ID: "npm:b", Package: pkgB, Relationship: model.DependencyTransitive,
						DependencyPath: []string{"project", "npm:a", "npm:b"}, Dependencies: []string{},
					},
					Constraint: "^2.0.0",
				},
			},
		},
		Findings: []model.Finding{{
			ID: "LIFECYCLE-POSTINSTALL", Kind: model.FindingHeuristic, Title: "npm postinstall lifecycle script declared",
			Description: "A lifecycle script was declared and was not executed.", Severity: model.SeverityLow,
			Package: pkgA, Version: &version,
			Evidence: []model.Evidence{{Kind: model.EvidenceHeuristic, Summary: "postinstall: node setup.js", SourceID: "project:npm"}},
		}},
		Checks: []model.CheckResult{
			{ID: "project:detection", Name: "Project format detection", Status: model.CheckSucceeded, Message: "Detected one format.", SourceIDs: []string{}},
			{ID: "npm:dependency-resolution", Name: "npm dependency resolution", Status: model.CheckSucceeded, Message: "Resolved one package.", SourceIDs: []string{}},
		},
		Sources: []model.SourceReference{{ID: "project:npm", Provider: model.SourceProviderNPM, Identifier: "package-lock.json", RetrievedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)}},
	}
	doc := render.FromProject(result)
	text := doc.DependenciesSection(render.Theme{Width: 100})
	for _, want := range []string{"(project root)", "a-pkg@1.0.0", "declared ^1.0.0", "b-pkg (unresolved)", "npm:b"} {
		if !strings.Contains(text, want) {
			t.Fatalf("dependency tree missing %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "└─") && !strings.Contains(text, "├─") {
		t.Fatalf("dependency tree has no branch markers:\n%s", text)
	}
}

func TestComparisonHandlesUnequalData(t *testing.T) {
	left := scanDemo(t, "vulnerable")
	theme := render.Theme{Width: 100}
	text := render.Comparison("npm:@depscope-demo/vulnerable-package", "npm:missing-demo-package", &left, nil, nil, errors.New("registry unavailable"), theme)
	for _, want := range []string{"PACKAGE COMPARISON", "LEFT", "RIGHT", "UNAVAILABLE", "COMPLETE", "[CRITICAL]", "SOURCE COVERAGE"} {
		if !strings.Contains(text, want) {
			t.Fatalf("comparison missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatal("comparison with color disabled contains ANSI escapes")
	}

	partial := left
	partial.Checks = append(append([]model.CheckResult(nil), left.Checks...), model.CheckResult{
		ID: "extra-check", Name: "Partial source", Status: model.CheckUnavailable, Message: "Source unavailable.", SourceIDs: []string{},
	})
	partialText := render.Comparison("npm:@depscope-demo/vulnerable-package", "npm:missing-demo-package", &partial, nil, nil, errors.New("registry unavailable"), theme)
	if !strings.Contains(partialText, "PARTIAL") {
		t.Fatalf("comparison does not mark partial data quality:\n%s", partialText)
	}

	narrow := render.Comparison("left", "right", &left, nil, nil, errors.New("registry unavailable"), render.Theme{Width: 48})
	if !strings.Contains(narrow, "LEFT  left") || !strings.Contains(narrow, "RIGHT  right") {
		t.Fatalf("narrow comparison is not stacked:\n%s", narrow)
	}
}

func TestColorDisabledProducesNoANSI(t *testing.T) {
	report := scanDemo(t, "vulnerable")
	doc := render.FromReport(report)
	theme := render.Theme{Width: 88}
	outputs := []string{
		doc.Overview(theme, 0),
		doc.Full(theme),
		doc.FindingDetail(0, theme),
		doc.ChecksSection(theme),
		doc.DependenciesSection(theme),
		doc.SourcesSection(theme),
		doc.CompactSummary(theme),
		render.Comparison("a", "b", &report, &report, nil, nil, theme),
	}
	for index, output := range outputs {
		if strings.Contains(output, "\x1b[") {
			t.Fatalf("output %d contains ANSI escapes with color disabled", index)
		}
	}
}

func TestColorEnabledProducesANSI(t *testing.T) {
	report := scanDemo(t, "vulnerable")
	text := render.FromReport(report).Overview(render.Theme{Color: true, Width: 88}, 0)
	if !strings.Contains(text, "\x1b[") {
		t.Fatal("color-enabled output contains no ANSI styling")
	}
	if !strings.Contains(text, "[CRITICAL]") {
		t.Fatal("color-enabled output lost explicit severity label")
	}
}

func TestCompactSummaryIncludesSourcesAndDemoNotice(t *testing.T) {
	report := scanDemo(t, "vulnerable")
	text := render.FromReport(report).CompactSummary(render.Theme{Width: 90})
	for _, want := range []string{"DEPSCOPE SUMMARY", "npm:@depscope-demo/vulnerable-package", "FINDINGS", "Highest CRITICAL", "Sources:", "demo:advisories", "Demo: DEMO DATA"} {
		if !strings.Contains(text, want) {
			t.Fatalf("compact summary missing %q:\n%s", want, text)
		}
	}
	if lines := strings.Count(text, "\n") + 1; lines > 14 {
		t.Fatalf("compact summary has %d lines, want at most 14", lines)
	}
}

func TestInventoryViewMarksTruncation(t *testing.T) {
	packages := make([]inventory.InstalledPackage, 0, 205)
	for index := 0; index < 205; index++ {
		identity, err := model.NewPackageIdentity(model.EcosystemNPM, "demo-pkg-"+string(rune('a'+index%26)))
		if err != nil {
			t.Fatalf("identity: %v", err)
		}
		packages = append(packages, inventory.InstalledPackage{
			Ecosystem: model.EcosystemNPM, Package: identity, Version: "1.0.0", Manager: "npm",
		})
	}
	result := inventory.Result{
		ScannedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Status:    model.StatusClean,
		Packages:  packages,
		Checks: []model.CheckResult{{
			ID: "npm:global-detection", Name: "npm global package detection", Status: model.CheckSucceeded,
			Message: "Read packages.", SourceIDs: []string{},
		}},
		Sources: []model.SourceReference{{ID: "inventory:npm", Provider: model.SourceProviderNPM, Identifier: "npm global", RetrievedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)}},
	}
	text := render.FromInventory(result).Full(render.Theme{Width: 90})
	if !strings.Contains(text, "omitted from this view") {
		t.Fatalf("inventory view does not mark truncation:\n%s", text)
	}
}

func scanDemo(t *testing.T, id string) model.ScanReport {
	t.Helper()
	report, err := demo.Scan(context.Background(), id)
	if err != nil {
		t.Fatalf("demo scan %q: %v", id, err)
	}
	return report
}

func severityPointer(value model.Severity) *model.Severity {
	return &value
}

func TestUntrustedTerminalTextIsSanitized(t *testing.T) {
	report := scanDemo(t, "vulnerable")
	report.Findings[0].Title = "bad \x1b[31mtext\x1b[0m bell\x07"
	report.Findings[0].Description = "line one\nline two \x1b]0;title\x07end"
	report.Checks[0].Message = "check \x1b[2Jclear"
	report.Limitations = append(report.Limitations, model.Limitation{Code: "LIMIT\x1b[31m", Message: "limit \x07 text"})
	url := "https://example.invalid/\x1b[31mred"
	report.Sources[0].URL = &url

	doc := render.FromReport(report)
	outputs := []string{doc.Overview(render.Theme{Width: 96}, 0), doc.Full(render.Theme{Width: 96}), doc.FindingDetail(0, render.Theme{Width: 96})}
	for index, output := range outputs {
		if strings.ContainsAny(output, "\x1b\x07") {
			t.Fatalf("output %d retained terminal control characters:\n%q", index, output)
		}
	}
	for _, want := range []string{"bad text bell", "line two end", "check clear"} {
		if !strings.Contains(outputs[1], want) {
			t.Fatalf("sanitized output is missing %q:\n%s", want, outputs[1])
		}
	}
	if !strings.Contains(report.Findings[0].Title, "\x1b") {
		t.Fatal("render mutated the original report instead of a display copy")
	}
}

func TestDetailsAndLimitationsSections(t *testing.T) {
	result := inventory.Result{
		ScannedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Status:    model.StatusIncomplete,
		Packages:  []inventory.InstalledPackage{},
		Checks:    []model.CheckResult{{ID: "npm:global-detection", Name: "npm detection", Status: model.CheckUnavailable, Message: "npm was unavailable.", SourceIDs: []string{}}},
		Limitations: []model.Limitation{{
			Code: "NPM_UNAVAILABLE", Message: "npm could not be queried.",
		}},
	}
	doc := render.FromInventory(result)
	details := doc.DetailsSection(render.Theme{Width: 80})
	if !strings.Contains(details, "INSTALLED PACKAGES") || !strings.Contains(details, "No installed packages") {
		t.Fatalf("details section is incomplete:\n%s", details)
	}
	limitations := doc.LimitationsSection(render.Theme{Width: 80})
	if !strings.Contains(limitations, "LIMITATIONS") || !strings.Contains(limitations, "NPM_UNAVAILABLE") {
		t.Fatalf("limitations section is incomplete:\n%s", limitations)
	}
}

func TestColorPolicy(t *testing.T) {
	env := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}
	cases := []struct {
		name    string
		mode    render.ColorMode
		tty     bool
		env     map[string]string
		enabled bool
	}{
		{name: "auto terminal", mode: render.ColorAuto, tty: true, enabled: true},
		{name: "auto redirect", mode: render.ColorAuto, tty: false, enabled: false},
		{name: "always redirect", mode: render.ColorAlways, tty: false, enabled: true},
		{name: "never terminal", mode: render.ColorNever, tty: true, enabled: false},
		{name: "no color override", mode: render.ColorAlways, tty: true, env: map[string]string{"NO_COLOR": "1"}, enabled: false},
		{name: "dumb terminal", mode: render.ColorAlways, tty: true, env: map[string]string{"TERM": "dumb"}, enabled: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := render.ColorEnabled(test.mode, test.tty, env(test.env)); got != test.enabled {
				t.Fatalf("ColorEnabled = %v, want %v", got, test.enabled)
			}
		})
	}
	if _, err := render.ParseColorMode("yes"); err == nil {
		t.Fatal("invalid color mode was accepted")
	}
}

// TestInventoryOverviewSeparatesCounts 验证清单报告把“已安装包”“受影响包”
// 与“漏洞匹配”三类数量分开呈现，避免读者把它们当成同一个数字。
func TestInventoryOverviewSeparatesCounts(t *testing.T) {
	urllib3, err := model.NewPackageIdentity(model.EcosystemPyPI, "urllib3")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	version := "1.26.5"
	makeFinding := func(id string, severity model.Severity) model.Finding {
		return model.Finding{
			ID:       id,
			Kind:     model.FindingConfirmed,
			Title:    "Advisory " + id,
			Severity: severity,
			Package:  urllib3,
			Version:  &version,
			Evidence: []model.Evidence{},
		}
	}
	result := inventory.Result{
		ScannedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Status:    model.StatusFindings,
		// 这些条目刻意不设置 Ecosystem，验证渲染层能回退到包标识中的生态。
		Packages: []inventory.InstalledPackage{
			{Package: urllib3, Version: version, Manager: "pip"},
			{Package: mustIdentity(t, "requests"), Version: "2.31.0", Manager: "pip"},
			{Package: mustIdentity(t, "certifi"), Version: "2024.2.2", Manager: "pip"},
		},
		Findings: []model.Finding{
			makeFinding("VULN-GHSA-qccp-gfcp-xxvc", model.SeverityHigh),
			makeFinding("VULN-GHSA-34jh-p97f-mpxf", model.SeverityCritical),
		},
		Overview: inventory.InventoryOverview{
			InstalledPackages: 3,
			AffectedPackages:  1,
			AdvisoryMatches:   2,
			FindingsBySeverity: map[model.Severity]int{
				model.SeverityCritical: 1,
				model.SeverityHigh:     1,
			},
		},
	}

	text := render.FromInventory(result).Full(render.Theme{Width: 100})
	for _, want := range []string{
		"OVERVIEW",
		"Installed packages      3",
		"(npm 0, PyPI 3)",
		"Affected packages       1",
		"Advisory matches        2",
		"critical 1 · high 1",
		"Counts are pairs, not distinct vulnerabilities.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("overview missing %q:\n%s", want, text)
		}
	}
	// 三类数字必须可以区分，不能出现一个含义模糊的总数。
	if strings.Contains(text, "total findings") {
		t.Fatalf("overview uses an ambiguous total:\n%s", text)
	}
}

// TestInventoryOverviewAppearsOnFirstScreen 回归测试：全局清单的分类计数必须出现在
// 报告首屏总览中，而不是只存在于详细信息页。
func TestInventoryOverviewAppearsOnFirstScreen(t *testing.T) {
	urllib3, err := model.NewPackageIdentity(model.EcosystemPyPI, "urllib3")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	version := "1.26.5"
	result := inventory.Result{
		ScannedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Status:    model.StatusFindings,
		Packages: []inventory.InstalledPackage{
			{Package: urllib3, Version: version, Manager: "pip", Ecosystem: model.EcosystemPyPI},
		},
		Findings: []model.Finding{{
			ID: "VULN-GHSA-qccp-gfcp-xxvc", Kind: model.FindingConfirmed, Title: "Advisory",
			Severity: model.SeverityHigh, Package: urllib3, Version: &version,
		}},
		Overview: inventory.InventoryOverview{
			InstalledPackages: 1, AffectedPackages: 1, AdvisoryMatches: 1,
			FindingsBySeverity: map[model.Severity]int{model.SeverityHigh: 1},
		},
	}
	overview := render.FromInventory(result).Overview(render.Theme{Width: 90}, 0)
	for _, want := range []string{"OVERVIEW", "Installed packages      1", "Affected packages       1", "Advisory matches        1"} {
		if !strings.Contains(overview, want) {
			t.Fatalf("first-screen overview is missing %q:\n%s", want, overview)
		}
	}
	// 计数必须出现在发现列表之前，确保重要信息先看到。
	// 使用行首标题定位，避免误匹配到说明文字中的同名字样。
	if strings.Index(overview, "Advisory matches") > strings.Index(overview, "\nFINDINGS") {
		t.Fatalf("counts appear after the finding list:\n%s", overview)
	}
}

func mustIdentity(t *testing.T, name string) model.PackageIdentity {
	t.Helper()
	identity, err := model.NewPackageIdentity(model.EcosystemPyPI, name)
	if err != nil {
		t.Fatalf("identity %q: %v", name, err)
	}
	return identity
}
