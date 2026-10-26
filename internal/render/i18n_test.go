package render_test

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/inventory"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
	"github.com/janghotan/depscope/internal/render"
)

// compactText 去掉换行与空白，便于断言被自动换行的句子或证据。
func compactText(value string) string {
	return strings.Join(strings.Fields(value), "")
}

func assertLinesFit(t *testing.T, name string, text string, width int) {
	t.Helper()
	for index, line := range strings.Split(text, "\n") {
		if got := ansi.StringWidth(line); got > width {
			t.Fatalf("%s line %d is %d columns, want <= %d: %q", name, index, got, width, line)
		}
	}
}

func bilingualOutputs(t *testing.T, language i18n.Language, width int, color bool) map[string]string {
	t.Helper()
	report := scanDemo(t, "vulnerable")
	theme := render.Theme{Width: width, Language: language, Color: color}
	document := render.FromReport(report, language)
	projectResult := projectFixture()
	projectDocument := render.FromProject(projectResult, language)
	inventoryResult := inventoryFixture()
	inventoryDocument := render.FromInventory(inventoryResult, language)
	return map[string]string{
		"overview":     document.Overview(theme, 0),
		"full":         document.Full(theme),
		"finding":      document.FindingDetail(0, theme),
		"checks":       document.ChecksSection(theme),
		"dependencies": projectDocument.DependenciesSection(theme),
		"details":      projectDocument.DetailsSection(theme),
		"inventory":    inventoryDocument.Full(theme),
		"sources":      document.SourcesSection(theme),
		"limitations":  document.LimitationsSection(theme),
		"summary":      document.CompactSummary(theme),
		"comparison":   render.Comparison("npm:left@1.0.0", "npm:right@1.0.0", &report, &report, nil, nil, theme),
	}
}

func projectFixture() project.Result {
	version := "1.0.0"
	pkg := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "demo-project-dependency"}
	unresolved := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "demo-unresolved-dependency"}
	return project.Result{
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
						ID: "npm:demo", Package: pkg, Version: &version, Relationship: model.DependencyDirect,
						DependencyPath: []string{"project", "npm:demo"}, Dependencies: []string{"npm:unresolved"},
					},
					Constraint: "^1.0.0",
				},
				{
					DependencyNode: model.DependencyNode{
						ID: "npm:unresolved", Package: unresolved, Relationship: model.DependencyTransitive,
						DependencyPath: []string{"project", "npm:demo", "npm:unresolved"}, Dependencies: []string{},
					},
					Constraint: "^2.0.0",
				},
			},
		},
		Findings: []model.Finding{{
			ID: "LIFECYCLE-POSTINSTALL", Kind: model.FindingHeuristic,
			Title: "npm postinstall lifecycle script declared", Description: "A lifecycle script was declared and was not executed.",
			Severity: model.SeverityLow, Package: pkg, Version: &version,
			Evidence: []model.Evidence{{Kind: model.EvidenceHeuristic, Summary: "postinstall: node setup.js", SourceID: "project:npm"}},
		}},
		Checks: []model.CheckResult{
			{ID: "project:detection", Name: "Project format detection", Status: model.CheckSucceeded, Message: "Detected one format.", SourceIDs: []string{}},
			{ID: "npm:dependency-resolution", Name: "npm dependency resolution", Status: model.CheckSucceeded, Message: "Resolved one package.", SourceIDs: []string{}},
		},
		Sources: []model.SourceReference{{
			ID: "project:npm", Provider: model.SourceProviderNPM, Identifier: "package-lock.json",
			RetrievedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		}},
	}
}

func inventoryFixture() inventory.Result {
	identity, _ := model.NewPackageIdentity(model.EcosystemNPM, "demo-installed-package")
	python, _ := model.NewPackageIdentity(model.EcosystemPyPI, "demo-python-package")
	return inventory.Result{
		ScannedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Status:    model.StatusFindings,
		Packages: []inventory.InstalledPackage{
			{Ecosystem: model.EcosystemNPM, Package: identity, Version: "1.0.0", Manager: "npm"},
			{Ecosystem: model.EcosystemPyPI, Package: python, Version: "2.0.0", Manager: "pip"},
		},
		Overview: inventory.InventoryOverview{InstalledPackages: 2, AffectedPackages: 1, AdvisoryMatches: 1},
		Findings: []model.Finding{{
			ID: "DEMO-INVENTORY-1", Kind: model.FindingConfirmed, Title: "Demo advisory for an installed package",
			Description: "Synthetic advisory used to verify inventory rendering.", Severity: model.SeverityHigh,
			Package: identity, Version: stringPointer("1.0.0"),
		}},
		Checks: []model.CheckResult{{ID: "inventory:npm", Name: "npm global detection", Status: model.CheckSucceeded, Message: "Read packages.", SourceIDs: []string{}}},
		Sources: []model.SourceReference{{
			ID: "inventory:npm", Provider: model.SourceProviderNPM, Identifier: "npm global",
			RetrievedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		}},
	}
}

func stringPointer(value string) *string {
	return &value
}

func TestBilingualRenderStaysWithinWidth(t *testing.T) {
	for _, language := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		for _, color := range []bool{false, true} {
			for _, width := range []int{42, 56, 76, 96, 136} {
				for name, text := range bilingualOutputs(t, language, width, color) {
					assertLinesFit(t, string(language)+"/"+name, text, width)
				}
			}
		}
	}
}

func TestBilingualRenderLocalizesChromeAndKeepsEvidence(t *testing.T) {
	report := scanDemo(t, "vulnerable")
	finding := report.Findings[0]

	englishTheme := render.Theme{Width: 96, Language: i18n.English}
	chineseTheme := render.Theme{Width: 96, Language: i18n.SimplifiedChinese}
	english := render.FromReport(report, i18n.English)
	chinese := render.FromReport(report, i18n.SimplifiedChinese)

	englishText := english.Overview(englishTheme, 0)
	chineseText := chinese.Overview(chineseTheme, 0)
	if !strings.Contains(englishText, "ASSESSMENT") || strings.Contains(englishText, "评估") {
		t.Fatalf("English overview is not English:\n%s", englishText)
	}
	zh := i18n.For(i18n.SimplifiedChinese)
	for _, want := range []string{
		zh.Text(i18n.KeyReportHeadingAssessment),
		zh.Format(i18n.KeyReportHeadingFindings, len(report.Findings)),
		zh.Text(i18n.KeyReportLabelStatus),
		zh.Text(i18n.KeyReportHeaderSources),
	} {
		if !strings.Contains(compactText(chineseText), compactText(want)) {
			t.Fatalf("Chinese overview is missing %q:\n%s", want, chineseText)
		}
	}
	if strings.Contains(chineseText, "ASSESSMENT") || strings.Contains(chineseText, "Completed checks") {
		t.Fatalf("Chinese overview retained English chrome:\n%s", chineseText)
	}

	englishFull := english.Full(englishTheme)
	chineseFull := chinese.Full(chineseTheme)
	if !strings.Contains(compactText(chineseFull), compactText(finding.Title)) {
		t.Fatalf("Chinese report rewrote the advisory title:\n%s", chineseFull)
	}
	if !strings.Contains(compactText(chineseFull), compactText(finding.ID)) {
		t.Fatalf("Chinese report rewrote the finding ID:\n%s", chineseFull)
	}
	for _, evidence := range finding.Evidence {
		if !strings.Contains(compactText(chineseFull), compactText(evidence.Summary)) {
			t.Fatalf("Chinese report rewrote evidence %q", evidence.Summary)
		}
	}
	if !strings.Contains(compactText(englishFull), compactText(finding.Title)) {
		t.Fatalf("English report lost the advisory title:\n%s", englishFull)
	}

	comparison := render.Comparison("npm:left@1.0.0", "npm:right@1.0.0", &report, &report, nil, nil, chineseTheme)
	if !strings.Contains(comparison, zh.Text(i18n.KeyComparisonTitle)) {
		t.Fatalf("Chinese comparison is missing its localized title:\n%s", comparison)
	}
	if !strings.Contains(comparison, finding.ID) {
		t.Fatalf("Chinese comparison rewrote a finding ID:\n%s", comparison)
	}
}

func TestRenderLanguageDefaultsAndFallback(t *testing.T) {
	report := scanDemo(t, "vulnerable")
	theme := render.Theme{Width: 90}
	base := render.FromReport(report).Full(theme)
	for _, language := range []i18n.Language{"", i18n.English, "fr"} {
		if got := render.FromReport(report, language).Full(theme); got != base {
			t.Fatalf("language %q changed the default English output", language)
		}
	}
	if !strings.Contains(base, "ASSESSMENT") || strings.Contains(base, "评估") {
		t.Fatalf("default render output is not English:\n%s", base)
	}
}
