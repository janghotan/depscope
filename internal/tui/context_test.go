package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/i18n"
	domain "github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
	"github.com/janghotan/depscope/internal/render"
)

func projectContextResult(t *testing.T) project.Result {
	t.Helper()
	version := "1.2.0"
	pkg, err := domain.NewPackageIdentity(domain.EcosystemNPM, "dev-child")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	findingID := "VULN-TEST-2026-0250@npm:dev-child@1.2.0"
	return project.Result{
		Root:      "/tmp/demo-project",
		ScannedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		Status:    domain.StatusFindings,
		Summary: domain.Summary{
			ConfirmedFindings: 1,
			HighestSeverity:   severityPointer(domain.SeverityHigh),
		},
		Formats: []project.FormatInfo{{
			Ecosystem: domain.EcosystemNPM, Format: project.FormatNPMPackageLock,
			ManifestPath: "package.json", LockfilePath: "package-lock.json",
			ResolvedPackages: 1, Resolution: project.DependencyResolutionComplete,
		}},
		Dependencies: project.Graph{
			RootID: "project",
			Nodes: []project.Node{{
				DependencyNode: domain.DependencyNode{
					ID: "npm:dev-child", Package: pkg, Version: &version,
					Relationship:   domain.DependencyTransitive,
					DependencyPath: []string{"project", "npm:dev-tool", "npm:dev-child"},
				},
				Scopes: []project.DependencyScope{project.DependencyScopeDevelopment},
			}},
		},
		Findings: []domain.Finding{{
			ID: findingID, Kind: domain.FindingConfirmed, Title: "Synthetic advisory",
			Description: "Synthetic advisory used by deterministic tests.",
			Severity:    domain.SeverityHigh, Package: pkg, Version: &version,
		}},
		Checks: []domain.CheckResult{
			{ID: "project:detection", Name: "Project format detection", Status: domain.CheckSucceeded, Message: "Detected one format.", SourceIDs: []string{}},
			{ID: "npm:dependency-resolution", Name: "npm dependency resolution", Status: domain.CheckSucceeded, Message: "Resolved one package.", SourceIDs: []string{}},
		},
		Sources: []domain.SourceReference{{
			ID: "project:npm", Provider: domain.SourceProviderNPM, Identifier: "package-lock.json",
			RetrievedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		}},
		FindingContexts: map[string]project.DependencyContext{
			findingID: {
				Relationship: domain.DependencyTransitive,
				Scopes:       []project.DependencyScope{project.DependencyScopeDevelopment},
				Resolution:   project.DependencyResolutionComplete,
				Evidence:     []project.DependencyEvidence{{Path: "package-lock.json", Field: "dev"}},
			},
		},
	}
}

func severityPointer(value domain.Severity) *domain.Severity {
	return &value
}

// TestProjectDependencyContextAppearsInFindingView 确认 TUI 通过共享渲染器展示
// 项目依赖上下文，并在切换语言后保留原始证据标识。
func TestProjectDependencyContextAppearsInFindingView(t *testing.T) {
	result := projectContextResult(t)
	model := sizedLanguageModel(t, i18n.English, 110, 40, false)
	model.documentSource = func() render.Document {
		return render.FromProject(result, model.language)
	}
	model.showDocument(model.documentSource(), screenMenu)
	model.reportView = viewFinding

	english := model.View()
	for _, want := range []string{"DEPENDENCY CONTEXT", "Relationship", "transitive", "development", "package-lock.json (dev)"} {
		if !strings.Contains(english, want) {
			t.Fatalf("English TUI finding view missing %q:\n%s", want, english)
		}
	}

	model.applyLanguage(i18n.SimplifiedChinese)
	chinese := model.View()
	catalog := i18n.For(i18n.SimplifiedChinese)
	for _, want := range []string{
		catalog.Text(i18n.KeyReportHeadingDependencyContext),
		catalog.Text(i18n.KeyReportRelationshipTransitive),
		catalog.Text(i18n.KeyReportScopeDevelopment),
		"package-lock.json (dev)",
	} {
		if !strings.Contains(compactText(chinese), compactText(want)) {
			t.Fatalf("Chinese TUI finding view missing %q:\n%s", want, chinese)
		}
	}
}
