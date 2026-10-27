package render_test

import (
	"strings"
	"testing"

	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
	"github.com/janghotan/depscope/internal/render"
)

// dependencyContextFixture 在项目展示夹具上附加依赖上下文与分类标记。
func dependencyContextFixture() project.Result {
	result := projectFixture()
	result.Formats[0].Resolution = project.DependencyResolutionComplete
	result.Dependencies.Nodes[0].Scopes = []project.DependencyScope{project.DependencyScopeDevelopment}
	result.Dependencies.Nodes[1].Scopes = []project.DependencyScope{project.DependencyScopeUnknown}
	result.FindingContexts = map[string]project.DependencyContext{
		"LIFECYCLE-POSTINSTALL": {
			Relationship: model.DependencyDirect,
			Scopes: []project.DependencyScope{
				project.DependencyScopeProduction,
				project.DependencyScopeDevelopment,
			},
			Resolution: project.DependencyResolutionComplete,
			Evidence: []project.DependencyEvidence{
				{Path: "package.json", Field: "devDependencies"},
				{Path: "package-lock.json"},
			},
		},
	}
	return result
}

func TestProjectFindingDetailShowsDependencyContext(t *testing.T) {
	result := dependencyContextFixture()
	doc := render.FromProject(result, i18n.English)
	text := doc.FindingDetail(0, render.Theme{Width: 100, Language: i18n.English})
	for _, want := range []string{
		"DEPENDENCY CONTEXT",
		"Relationship",
		"direct",
		"Scope",
		"production, development",
		"Resolution",
		"complete",
		"Evidence",
		"package.json (devDependencies)",
		"package-lock.json",
		"does not determine",
		"code reachability",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("finding detail missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "does not record a production or development classification") {
		t.Fatalf("complete context unexpectedly shows the unknown-scope note:\n%s", text)
	}
	if strings.Contains(text, "resolution is partial") {
		t.Fatalf("complete context unexpectedly shows the partial-resolution note:\n%s", text)
	}

	overview := doc.Overview(render.Theme{Width: 100, Language: i18n.English}, 0)
	if !strings.Contains(overview, "· direct · production, development") {
		t.Fatalf("finding list lacks the compact context suffix:\n%s", overview)
	}
}

func TestProjectDependencyContextIsBilingual(t *testing.T) {
	result := dependencyContextFixture()
	catalog := i18n.For(i18n.SimplifiedChinese)
	doc := render.FromProject(result, i18n.SimplifiedChinese)
	text := doc.FindingDetail(0, render.Theme{Width: 100, Language: i18n.SimplifiedChinese})
	for _, want := range []string{
		catalog.Text(i18n.KeyReportHeadingDependencyContext),
		catalog.Text(i18n.KeyReportLabelRelationship),
		catalog.Text(i18n.KeyReportRelationshipDirect),
		catalog.Text(i18n.KeyReportLabelScope),
		catalog.Text(i18n.KeyReportScopeProduction) + ", " + catalog.Text(i18n.KeyReportScopeDevelopment),
		catalog.Text(i18n.KeyReportLabelResolution),
		catalog.Text(i18n.KeyReportResolutionComplete),
		catalog.Text(i18n.KeyReportLabelContextEvidence),
		catalog.Text(i18n.KeyReportContextDisclaimer),
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Chinese finding detail missing %q:\n%s", want, text)
		}
	}
	// 证据路径与字段是原始标识，界面只翻译标签，不改写证据。
	for _, want := range []string{"package.json (devDependencies)", "package-lock.json"} {
		if !strings.Contains(text, want) {
			t.Fatalf("Chinese finding detail rewrote evidence %q:\n%s", want, text)
		}
	}
	overview := doc.Overview(render.Theme{Width: 100, Language: i18n.SimplifiedChinese}, 0)
	if !strings.Contains(overview, "· "+catalog.Text(i18n.KeyReportRelationshipDirect)+" · "+
		catalog.Text(i18n.KeyReportScopeProduction)+", "+catalog.Text(i18n.KeyReportScopeDevelopment)) {
		t.Fatalf("Chinese finding list lacks the compact context suffix:\n%s", overview)
	}
}

func TestProjectDependencyContextUnknownAndPartialAreExplicit(t *testing.T) {
	result := dependencyContextFixture()
	result.FindingContexts["LIFECYCLE-POSTINSTALL"] = project.DependencyContext{
		Relationship: model.DependencyTransitive,
		Scopes:       []project.DependencyScope{project.DependencyScopeUnknown},
		Resolution:   project.DependencyResolutionPartial,
		Evidence:     []project.DependencyEvidence{{Path: "requirements.txt"}},
	}

	english := render.FromProject(result, i18n.English).FindingDetail(0, render.Theme{Width: 100, Language: i18n.English})
	for _, want := range []string{
		"transitive",
		"unknown",
		"partial",
		"does not record a production or development classification",
		"Dependency tree resolution is partial",
		"requirements.txt",
	} {
		if !strings.Contains(english, want) {
			t.Fatalf("English partial context missing %q:\n%s", want, english)
		}
	}

	catalog := i18n.For(i18n.SimplifiedChinese)
	chinese := render.FromProject(result, i18n.SimplifiedChinese).FindingDetail(0, render.Theme{Width: 100, Language: i18n.SimplifiedChinese})
	for _, want := range []string{
		catalog.Text(i18n.KeyReportRelationshipTransitive),
		catalog.Text(i18n.KeyReportScopeUnknown),
		catalog.Text(i18n.KeyReportResolutionPartial),
		catalog.Text(i18n.KeyReportScopeUnknownNote),
		catalog.Text(i18n.KeyReportResolutionPartialNote),
	} {
		if !strings.Contains(chinese, want) {
			t.Fatalf("Chinese partial context missing %q:\n%s", want, chinese)
		}
	}
}

func TestProjectDependencyTreeTagsNonProductionScopes(t *testing.T) {
	result := dependencyContextFixture()
	text := render.FromProject(result, i18n.English).DependenciesSection(render.Theme{Width: 100, Language: i18n.English})
	for _, want := range []string{
		"npm:demo-project-dependency@1.0.0 · declared ^1.0.0 [development]",
		"npm:demo-unresolved-dependency (unresolved) · declared ^2.0.0 [unknown]",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("dependency tree missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "[production]") {
		t.Fatalf("production-only nodes should not carry a scope tag:\n%s", text)
	}

	production := dependencyContextFixture()
	production.Dependencies.Nodes[0].Scopes = []project.DependencyScope{project.DependencyScopeProduction}
	production.Dependencies.Nodes[1].Scopes = []project.DependencyScope{project.DependencyScopeProduction}
	plain := render.FromProject(production, i18n.English).DependenciesSection(render.Theme{Width: 100, Language: i18n.English})
	if strings.Contains(plain, "[production]") {
		t.Fatalf("production-only tree gained a redundant scope tag:\n%s", plain)
	}
}

func TestProjectDetailsReportFormatResolution(t *testing.T) {
	complete := dependencyContextFixture()
	english := render.FromProject(complete, i18n.English).DetailsSection(render.Theme{Width: 100, Language: i18n.English})
	if !strings.Contains(english, "resolution: complete") {
		t.Fatalf("English details missing complete resolution:\n%s", english)
	}
	chinese := render.FromProject(complete, i18n.SimplifiedChinese).DetailsSection(render.Theme{Width: 100, Language: i18n.SimplifiedChinese})
	catalog := i18n.For(i18n.SimplifiedChinese)
	if !strings.Contains(chinese, catalog.Format(i18n.KeyReportFormatResolutionLine, catalog.Text(i18n.KeyReportResolutionComplete))) {
		t.Fatalf("Chinese details missing complete resolution:\n%s", chinese)
	}

	partial := dependencyContextFixture()
	partial.Formats[0].Resolution = project.DependencyResolutionPartial
	english = render.FromProject(partial, i18n.English).DetailsSection(render.Theme{Width: 100, Language: i18n.English})
	if !strings.Contains(english, "resolution: partial") {
		t.Fatalf("English details missing partial resolution:\n%s", english)
	}
}

func TestPackageReportsDoNotShowDependencyContext(t *testing.T) {
	report := scanDemo(t, "vulnerable")
	text := render.FromReport(report).FindingDetail(0, render.Theme{Width: 100})
	if strings.Contains(text, "DEPENDENCY CONTEXT") {
		t.Fatalf("package report claims project dependency context:\n%s", text)
	}
}

// TestDependencyContextSurvivesFindingIDSanitization 确认不可信 ID 被清理后
// 仍能匹配上下文，且不把转义序列带入终端。
func TestDependencyContextSurvivesFindingIDSanitization(t *testing.T) {
	result := dependencyContextFixture()
	rawID := "LIFECYCLE-\x1b[31mPOSTINSTALL"
	result.Findings[0].ID = rawID
	result.FindingContexts = map[string]project.DependencyContext{
		rawID: result.FindingContexts["LIFECYCLE-POSTINSTALL"],
	}
	text := render.FromProject(result).FindingDetail(0, render.Theme{Width: 100})
	if strings.Contains(text, "\x1b") {
		t.Fatalf("finding detail retained a terminal escape sequence:\n%q", text)
	}
	if !strings.Contains(text, "DEPENDENCY CONTEXT") {
		t.Fatalf("sanitized finding ID lost its dependency context:\n%s", text)
	}
}

func TestProjectDependencyContextStaysWithinWidth(t *testing.T) {
	result := dependencyContextFixture()
	for _, language := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		for _, width := range []int{42, 56, 76, 96} {
			doc := render.FromProject(result, language)
			theme := render.Theme{Width: width, Language: language}
			outputs := map[string]string{
				"overview":     doc.Overview(theme, 0),
				"full":         doc.Full(theme),
				"finding":      doc.FindingDetail(0, theme),
				"dependencies": doc.DependenciesSection(theme),
				"details":      doc.DetailsSection(theme),
			}
			for name, text := range outputs {
				assertLinesFit(t, string(language)+"/"+name, text, width)
			}
		}
	}
}
