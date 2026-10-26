package render_test

import (
	"strings"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/inventory"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
	"github.com/janghotan/depscope/internal/render"
	"github.com/janghotan/depscope/internal/termtext"
)

// testIcons 使用大写字母代替真实字形，便于断言图标与文字标签的相对位置。
func testIcons() render.IconSet {
	return render.IconSet{
		Demo:             "D",
		Confirmed:        "C",
		Heuristic:        "H",
		Critical:         "A",
		High:             "B",
		Medium:           "M",
		Low:              "L",
		Unknown:          "U",
		Clean:            "K",
		Warning:          "W",
		Error:            "E",
		CheckOK:          "O",
		CheckUnavailable: "X",
		CheckSkipped:     "S",
		CheckFailed:      "F",
		Source:           "Q",
		Dependency:       "Y",
		Action:           "Z",
	}
}

// TestIconSetAssociationsPreserveText 验证图标只出现在标签之外，
// [CRITICAL]、[CONFIRMED] 等文字标签保持完整，并且映射到正确的语义槽位。
func TestIconSetAssociationsPreserveText(t *testing.T) {
	report := scanDemo(t, "vulnerable")
	theme := render.Theme{Width: 120, Icons: testIcons()}
	overview := render.FromReport(report).Overview(theme, 0)
	for _, want := range []string{
		"D [DEMO DATA]",
		"W [FINDINGS]",
		"A [CRITICAL]",
		"C [CONFIRMED]",
		"Z NEXT ACTIONS",
	} {
		if !strings.Contains(overview, want) {
			t.Fatalf("overview missing %q:\n%s", want, overview)
		}
	}

	scripts := render.FromReport(scanDemo(t, "install-scripts"))
	scriptText := scripts.Overview(theme, 0)
	if !strings.Contains(scriptText, "H [HEURISTIC]") {
		t.Fatalf("heuristic findings lost their semantic icon:\n%s", scriptText)
	}

	clean := render.FromReport(scanDemo(t, "clean"))
	cleanText := clean.Overview(theme, 0)
	if !strings.Contains(cleanText, "K [CLEAN]") {
		t.Fatalf("clean status lost its semantic icon:\n%s", cleanText)
	}

	incompleteReport := scanDemo(t, "clean")
	incompleteReport.Status = model.StatusIncomplete
	if text := render.FromReport(incompleteReport).Overview(theme, 0); !strings.Contains(text, "W [INCOMPLETE]") {
		t.Fatalf("incomplete status lost its warning icon:\n%s", text)
	}

	failedReport := scanDemo(t, "clean")
	failedReport.Status = model.StatusFailed
	if text := render.FromReport(failedReport).Overview(theme, 0); !strings.Contains(text, "E [FAILED]") {
		t.Fatalf("failed status lost its error icon:\n%s", text)
	}
}

// TestCheckBadgeIcons 验证四种检查状态各自使用不同图标，并保留文字标签。
func TestCheckBadgeIcons(t *testing.T) {
	report := scanDemo(t, "clean")
	report.Status = model.StatusIncomplete
	report.Checks = append(report.Checks,
		model.CheckResult{ID: "unavailable", Name: "Unavailable", Status: model.CheckUnavailable, Message: "Unavailable.", SourceIDs: []string{}},
		model.CheckResult{ID: "skipped", Name: "Skipped", Status: model.CheckSkipped, Message: "Skipped.", SourceIDs: []string{}},
		model.CheckResult{ID: "failed", Name: "Failed", Status: model.CheckFailed, Message: "Failed.", SourceIDs: []string{}},
	)
	text := render.FromReport(report).ChecksSection(render.Theme{Width: 120, Icons: testIcons()})
	for _, want := range []string{"O [OK]", "X [UNAVAILABLE]", "S [SKIPPED]", "F [FAILED]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("checks section missing %q:\n%s", want, text)
		}
	}
}

// TestSeverityAndKindIcons 验证每个严重性与发现类型使用独立槽位。
func TestSeverityAndKindIcons(t *testing.T) {
	theme := render.Theme{Width: 120, Icons: testIcons()}
	cases := []struct {
		severity model.Severity
		kind     model.FindingKind
		want     string
	}{
		{model.SeverityCritical, model.FindingConfirmed, "A [CRITICAL] C [CONFIRMED]"},
		{model.SeverityHigh, model.FindingConfirmed, "B [HIGH] C [CONFIRMED]"},
		{model.SeverityMedium, model.FindingHeuristic, "M [MEDIUM] H [HEURISTIC]"},
		{model.SeverityLow, model.FindingHeuristic, "L [LOW] H [HEURISTIC]"},
		{model.SeverityUnknown, model.FindingConfirmed, "U [UNKNOWN] C [CONFIRMED]"},
	}
	for _, test := range cases {
		report := singleFindingReport(t, test.severity, test.kind)
		text := render.FromReport(report).Overview(theme, 0)
		if !strings.Contains(text, test.want) {
			t.Fatalf("%s finding missing %q:\n%s", test.severity, test.want, text)
		}
	}
}

// TestSectionIcons 验证依赖、来源、证据、限制与行动区块的图标位置。
func TestSectionIcons(t *testing.T) {
	theme := render.Theme{Width: 120, Icons: testIcons()}
	report := scanDemo(t, "vulnerable")
	report.Limitations = append(report.Limitations, model.Limitation{Code: "LIMIT-DEMO", Message: "Synthetic limitation."})
	doc := render.FromReport(report)

	if text := doc.DependenciesSection(theme); !strings.Contains(text, "Y DEPENDENCIES") {
		t.Fatalf("dependencies heading lost its icon:\n%s", text)
	}
	if text := doc.SourcesSection(theme); !strings.Contains(text, "Q SOURCES") || !strings.Contains(text, "  Q ") {
		t.Fatalf("sources section lost its icon:\n%s", text)
	}
	if text := doc.LimitationsSection(theme); !strings.Contains(text, "W LIMITATIONS") {
		t.Fatalf("limitations heading lost its warning icon:\n%s", text)
	}
	if text := doc.FindingDetail(0, theme); !strings.Contains(text, "Q EVIDENCE") || !strings.Contains(text, "Z NEXT ACTIONS") {
		t.Fatalf("finding detail lost semantic headings:\n%s", text)
	}
}

// TestCompactSummaryKeepsBracketLabels 验证摘要中的严重性与类型标签仍是完整方括号文本。
func TestCompactSummaryKeepsBracketLabels(t *testing.T) {
	report := scanDemo(t, "vulnerable")
	text := render.FromReport(report).CompactSummary(render.Theme{Width: 120, Icons: testIcons()})
	for _, want := range []string{"W FINDINGS", "A CRITICAL", "- A [CRITICAL] C [CONFIRMED] "} {
		if !strings.Contains(text, want) {
			t.Fatalf("compact summary missing %q:\n%s", want, text)
		}
	}
}

// TestComparisonIconsUseWarningForUnavailableCoverage 验证无法获得的对比数据
// 使用警告语义而不是错误语义，并且两侧标签仍然完整。
func TestComparisonIconsUseWarningForUnavailableCoverage(t *testing.T) {
	left := scanDemo(t, "vulnerable")
	theme := render.Theme{Width: 110, Icons: testIcons()}
	text := render.Comparison("npm:left", "npm:right", &left, nil, nil, nil, theme)
	for _, want := range []string{
		"W UNAVAILABLE",
		"K COMPLETE",
		"Q SOURCE COVERAGE",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("comparison missing %q:\n%s", want, text)
		}
	}

	partial := left
	partial.Checks = append(append([]model.CheckResult(nil), left.Checks...), model.CheckResult{
		ID: "icon:partial", Name: "Partial source", Status: model.CheckUnavailable,
		Message: "Source unavailable.", SourceIDs: []string{},
	})
	partialText := render.Comparison("npm:left", "npm:right", &partial, nil, nil, nil, theme)
	if !strings.Contains(partialText, "W PARTIAL") {
		t.Fatalf("partial data quality lost its warning icon:\n%s", partialText)
	}
}

// TestHighSeverityUsesDangerColor 验证严重与高危都使用红色语义，
// 但通过不同字形和文字标签区分，颜色不是唯一线索。
func TestHighSeverityUsesDangerColor(t *testing.T) {
	theme := render.Theme{Color: true, Width: 120, Icons: testIcons()}
	critical := render.FromReport(singleFindingReport(t, model.SeverityCritical, model.FindingConfirmed)).Overview(theme, 0)
	if !strings.Contains(critical, "\x1b[1;38;5;203mA [CRITICAL]\x1b[0m") {
		t.Fatalf("critical badge is not painted with the danger color:\n%q", critical)
	}
	high := render.FromReport(singleFindingReport(t, model.SeverityHigh, model.FindingConfirmed)).Overview(theme, 0)
	if !strings.Contains(high, "\x1b[38;5;203mB [HIGH]\x1b[0m") {
		t.Fatalf("high badge is not painted with red:\n%q", high)
	}
	medium := render.FromReport(singleFindingReport(t, model.SeverityMedium, model.FindingHeuristic)).Overview(theme, 0)
	if !strings.Contains(medium, "\x1b[38;5;221mM [MEDIUM]\x1b[0m") {
		t.Fatalf("medium badge is not painted with the warning color:\n%q", medium)
	}
}

// TestIconRenderingFitsNarrowTerminals 验证启用图标后，中英文报告与对比在
// 所有支持宽度下都不越过内容宽度。
func TestIconRenderingFitsNarrowTerminals(t *testing.T) {
	projectResult := dependencyProjectResult(t)
	inventoryResult := inventoryResultFixture(t)
	for _, language := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		for _, width := range []int{32, 46, 60, 80, 100} {
			theme := render.Theme{Width: width, Language: language, Icons: testIcons()}
			documents := []render.Document{
				render.FromReport(scanDemo(t, "vulnerable"), language),
				render.FromProject(projectResult, language),
				render.FromInventory(inventoryResult, language),
			}
			for documentIndex, doc := range documents {
				outputs := []string{
					doc.Overview(theme, 0),
					doc.Full(theme),
					doc.ChecksSection(theme),
					doc.DependenciesSection(theme),
					doc.DetailsSection(theme),
					doc.SourcesSection(theme),
					doc.LimitationsSection(theme),
					doc.CompactSummary(theme),
				}
				for outputIndex, output := range outputs {
					assertWithinWidth(t, output, theme.WidthForContent(), language, width, documentIndex, outputIndex)
				}
			}
			left := scanDemo(t, "vulnerable")
			assertWithinWidth(t, render.Comparison("npm:left", "npm:right", &left, nil, nil, nil, theme), theme.WidthForContent(), language, width, 9, 9)
		}
	}
}

// singleFindingReport 构造只含一条发现的最小报告，用于精确断言徽标输出。
func singleFindingReport(t *testing.T, severity model.Severity, kind model.FindingKind) model.ScanReport {
	t.Helper()
	identity, err := model.NewPackageIdentity(model.EcosystemNPM, "icon-fixture")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	version := "1.0.0"
	limit := 0
	return model.ScanReport{
		SchemaVersion: model.SchemaVersion,
		DataMode:      model.DataModeLive,
		ScannedAt:     time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Status:        model.StatusFindings,
		Target:        model.ScanTarget{Package: identity, ResolvedVersion: &version},
		Summary: model.Summary{
			ConfirmedFindings: limit,
			HighestSeverity:   severityPointer(severity),
		},
		Findings: []model.Finding{{
			ID: "VULN-ICON-FIXTURE", Kind: kind, Title: "Icon fixture finding",
			Description: "Fixture description.", Severity: severity, Package: identity, Version: &version,
		}},
		Checks: []model.CheckResult{{
			ID: "icon:check", Name: "Icon check", Status: model.CheckSucceeded,
			Message: "Check succeeded.", SourceIDs: []string{},
		}},
		Sources: []model.SourceReference{{
			ID: "osv:fixture", Provider: model.SourceProviderOSV, Identifier: "icon-fixture",
			RetrievedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		}},
	}
}

// dependencyProjectResult 构造带约束与未解析节点的项目结果，覆盖依赖树图标与宽度。
func dependencyProjectResult(t *testing.T) project.Result {
	t.Helper()
	version := "1.0.0"
	pkgA, err := model.NewPackageIdentity(model.EcosystemNPM, "a-pkg")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	pkgB, err := model.NewPackageIdentity(model.EcosystemNPM, "b-pkg")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	return project.Result{
		Root:      "/tmp/icon-project",
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
				{DependencyNode: model.DependencyNode{
					ID: "npm:a", Package: pkgA, Version: &version, Relationship: model.DependencyDirect,
					DependencyPath: []string{"project", "npm:a"}, Dependencies: []string{"npm:b"},
				}, Constraint: "^1.0.0"},
				{DependencyNode: model.DependencyNode{
					ID: "npm:b", Package: pkgB, Relationship: model.DependencyTransitive,
					DependencyPath: []string{"project", "npm:a", "npm:b"}, Dependencies: []string{},
				}, Constraint: "^2.0.0"},
			},
		},
		Checks: []model.CheckResult{{
			ID: "project:detection", Name: "Project format detection", Status: model.CheckSucceeded,
			Message: "Detected one format.", SourceIDs: []string{},
		}},
		Sources: []model.SourceReference{{
			ID: "project:npm", Provider: model.SourceProviderNPM, Identifier: "package-lock.json",
			RetrievedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		}},
	}
}

// inventoryResultFixture 构造带分类计数的全局清单结果，覆盖首屏总览与长包名。
func inventoryResultFixture(t *testing.T) inventory.Result {
	t.Helper()
	identity, err := model.NewPackageIdentity(model.EcosystemPyPI, "urllib3")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	version := "1.26.5"
	return inventory.Result{
		ScannedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Status:    model.StatusFindings,
		Packages: []inventory.InstalledPackage{{
			Ecosystem: model.EcosystemPyPI, Package: identity, Version: version, Manager: "pip",
		}},
		Findings: []model.Finding{{
			ID: "VULN-ICON-FIXTURE", Kind: model.FindingConfirmed, Title: "Synthetic advisory",
			Severity: model.SeverityHigh, Package: identity, Version: &version,
		}},
		Overview: inventory.InventoryOverview{
			InstalledPackages: 1, AffectedPackages: 1, AdvisoryMatches: 1,
			FindingsBySeverity: map[model.Severity]int{model.SeverityHigh: 1},
		},
		Checks: []model.CheckResult{{
			ID: "pip:global-detection", Name: "pip global package detection", Status: model.CheckSucceeded,
			Message: "Read packages.", SourceIDs: []string{},
		}},
		Sources: []model.SourceReference{{
			ID: "inventory:pip", Provider: model.SourceProviderPyPI, Identifier: "pip global",
			RetrievedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		}},
	}
}

func assertWithinWidth(t *testing.T, output string, width int, language i18n.Language, terminalWidth, documentIndex, outputIndex int) {
	t.Helper()
	for lineIndex, line := range strings.Split(output, "\n") {
		if got := termtext.Width(line); got > width {
			t.Fatalf("%s %d columns: document %d output %d line %d is %d wide, want <= %d: %q",
				language, terminalWidth, documentIndex, outputIndex, lineIndex, got, width, line)
		}
	}
}
