// Package mcp 的综合边界用例与回归测试。
package mcp

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPEmptyFindingsAndCleanState(t *testing.T) {
	service := &fakeService{
		scanPackageFunc: func(ctx context.Context, request app.PackageRequest, progress func(app.Progress)) (model.ScanReport, error) {
			version := "1.0.0"
			return model.ScanReport{
				SchemaVersion: model.SchemaVersion,
				DataMode:      model.DataModeLive,
				ScannedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Status:        model.StatusClean,
				Target: model.ScanTarget{
					Package:         model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "clean-package"},
					ResolvedVersion: &version,
				},
				Summary: model.Summary{
					ConfirmedFindings: 0,
					HeuristicWarnings: 0,
					HighestSeverity:   nil,
				},
				Findings: []model.Finding{},
				Sources: []model.SourceReference{
					{
						ID:          "npm:clean-package@1.0.0",
						Provider:    model.SourceProviderNPM,
						Identifier:  "clean-package",
						RetrievedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			}, nil
		},
	}

	client, _, cleanup := setupTestMCPClientServer(t, service, nil)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_package",
		Arguments: map[string]any{
			"package": "clean-package",
		},
	})
	if err != nil {
		t.Fatalf("scan_package failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected successful call, got IsError=true")
	}

	output := decodeStructuredContent[ScanPackageOutput](t, res)
	if output.Status != string(model.StatusClean) {
		t.Errorf("status = %q, want clean", output.Status)
	}
	if len(output.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(output.Findings))
	}
	if output.Summary.ConfirmedFindings != 0 || output.Summary.HighestSeverity != nil {
		t.Errorf("unexpected summary: %+v", output.Summary)
	}
	// 验证未误导性声称绝对安全
	if output.Message == "" {
		t.Errorf("expected informative status message")
	}
}

func TestMCPPartialResolutionAndUnsupportedFormat(t *testing.T) {
	tempDir := t.TempDir()
	service := &fakeService{
		scanProjectFunc: func(ctx context.Context, request app.ProjectRequest, progress func(app.Progress)) (project.Result, error) {
			return project.Result{
				Root:      request.Path,
				ScannedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Status:    model.StatusIncomplete,
				Formats: []project.FormatInfo{
					{
						Ecosystem:          model.EcosystemPyPI,
						Format:             project.FormatPythonRequirements,
						ManifestPath:       filepath.Join(request.Path, "requirements.txt"),
						ResolvedPackages:   2,
						UnresolvedPackages: 3,
						Resolution:         project.DependencyResolutionPartial,
					},
				},
				Limitations: []model.Limitation{
					{
						Code:    "PARTIAL_RESOLUTION",
						Message: "Some dependencies were not pinned to exact versions.",
					},
					{
						Code:    "UNSUPPORTED_FORMAT",
						Message: "Detected unsupported dependency file: poetry.lock.",
					},
				},
				Summary: model.Summary{
					ConfirmedFindings: 0,
					HighestSeverity:   nil,
				},
				Findings: []model.Finding{},
			}, nil
		},
	}

	client, _, cleanup := setupTestMCPClientServer(t, service, []string{tempDir})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_project",
		Arguments: map[string]any{
			"path": tempDir,
		},
	})
	if err != nil {
		t.Fatalf("scan_project failed: %v", err)
	}

	output := decodeStructuredContent[ScanProjectOutput](t, res)
	if output.Status != string(model.StatusIncomplete) {
		t.Errorf("status = %q, want incomplete", output.Status)
	}
	if len(output.Formats) != 1 || output.Formats[0].Resolution != string(project.DependencyResolutionPartial) {
		t.Errorf("format resolution = %q, want partial", output.Formats[0].Resolution)
	}
	if len(output.Limitations) != 2 {
		t.Errorf("limitations count = %d, want 2", len(output.Limitations))
	}

	// 测试 get_scan_summary 对该 incomplete 项目的反馈
	summaryRes, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_scan_summary",
		Arguments: map[string]any{
			"scan_id": output.ScanID,
		},
	})
	if err != nil {
		t.Fatalf("get_scan_summary failed: %v", err)
	}
	summaryOut := decodeStructuredContent[GetScanSummaryOutput](t, summaryRes)
	if !summaryOut.HasUnresolvedDependencies {
		t.Errorf("has_unresolved_dependencies = false, want true")
	}
	if summaryOut.Resolution != string(project.DependencyResolutionPartial) {
		t.Errorf("resolution = %q, want partial", summaryOut.Resolution)
	}
}

func TestMCPGetScanSummaryDirectScan(t *testing.T) {
	tempDir := t.TempDir()
	service := &fakeService{
		scanProjectFunc: func(ctx context.Context, request app.ProjectRequest, progress func(app.Progress)) (project.Result, error) {
			return project.Result{
				Root:      request.Path,
				ScannedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Status:    model.StatusClean,
				Formats: []project.FormatInfo{
					{
						Ecosystem:        model.EcosystemNPM,
						Format:           project.FormatNPMPackageLock,
						ManifestPath:     filepath.Join(request.Path, "package.json"),
						ResolvedPackages: 5,
						Resolution:       project.DependencyResolutionComplete,
					},
				},
				Summary: model.Summary{
					ConfirmedFindings: 0,
				},
			}, nil
		},
	}

	client, _, cleanup := setupTestMCPClientServer(t, service, []string{tempDir})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// get_scan_summary 在未提供 scan_id 时通过 path 参数即时扫描并汇总
	res, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_scan_summary",
		Arguments: map[string]any{
			"path": tempDir,
		},
	})
	if err != nil {
		t.Fatalf("get_scan_summary with path failed: %v", err)
	}
	summaryOut := decodeStructuredContent[GetScanSummaryOutput](t, res)
	if summaryOut.TargetType != "project" || summaryOut.TargetName != tempDir {
		t.Errorf("unexpected target: %s / %s", summaryOut.TargetType, summaryOut.TargetName)
	}
	if summaryOut.Status != string(model.StatusClean) {
		t.Errorf("status = %q, want clean", summaryOut.Status)
	}
}
