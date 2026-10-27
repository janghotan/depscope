// Package mcp 的综合集成测试：覆盖工具发现、参数校验、项目扫描、单包扫描、详情检索、汇总、超时与取消。
package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/inventory"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// decodeStructuredContent 辅助将 CallToolResult 的 StructuredContent 解码为目标结构体。
func decodeStructuredContent[T any](t *testing.T, res *sdk.CallToolResult) T {
	t.Helper()
	var out T
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal into target: %v", err)
	}
	return out
}

// fakeService 实现 app.Service 供测试使用。
type fakeService struct {
	scanPackageFunc func(ctx context.Context, request app.PackageRequest, progress func(app.Progress)) (model.ScanReport, error)
	scanProjectFunc func(ctx context.Context, request app.ProjectRequest, progress func(app.Progress)) (project.Result, error)
}

func (f *fakeService) ScanPackage(ctx context.Context, request app.PackageRequest, progress func(app.Progress)) (model.ScanReport, error) {
	if f.scanPackageFunc != nil {
		return f.scanPackageFunc(ctx, request, progress)
	}
	return model.ScanReport{}, nil
}

func (f *fakeService) ScanProject(ctx context.Context, request app.ProjectRequest, progress func(app.Progress)) (project.Result, error) {
	if f.scanProjectFunc != nil {
		return f.scanProjectFunc(ctx, request, progress)
	}
	return project.Result{}, nil
}

func (f *fakeService) ScanInventory(ctx context.Context, progress func(app.Progress)) (inventory.Result, error) {
	return inventory.Result{}, nil
}

func (f *fakeService) Compare(ctx context.Context, request app.CompareRequest, progress func(app.Progress)) (app.Comparison, error) {
	return app.Comparison{}, nil
}

func (f *fakeService) Demo(ctx context.Context, scenario string, progress func(app.Progress)) (model.ScanReport, error) {
	return model.ScanReport{}, nil
}

func (f *fakeService) DemoScenarios() []app.DemoScenario {
	return nil
}

// setupTestMCPClientServer 启动配对的内存客户端与服务端。
func setupTestMCPClientServer(t *testing.T, service app.Service, allowedPaths []string) (*sdk.ClientSession, *Server, func()) {
	t.Helper()
	ctx := context.Background()

	clientTransport, serverTransport := sdk.NewInMemoryTransports()

	server, err := NewServer(ServerOptions{
		Service:      service,
		AllowedPaths: allowedPaths,
		Version:      "1.0.0-test",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	serverSession, err := server.MCPServer().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}

	cleanup := func() {
		_ = clientSession.Close()
		_ = serverSession.Wait()
	}

	return clientSession, server, cleanup
}

func TestMCPToolDiscovery(t *testing.T) {
	service := &fakeService{}
	client, _, cleanup := setupTestMCPClientServer(t, service, nil)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	toolsResult, err := client.ListTools(ctx, &sdk.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	expectedTools := map[string]bool{
		"scan_project":        false,
		"scan_package":        false,
		"get_finding_details": false,
		"get_scan_summary":    false,
	}

	for _, tool := range toolsResult.Tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
			if tool.Description == "" {
				t.Errorf("tool %q description is empty", tool.Name)
			}
			if tool.InputSchema == nil {
				t.Errorf("tool %q input schema is nil", tool.Name)
			}
		}
	}

	for toolName, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q was not exposed by MCP server", toolName)
		}
	}
}

func TestMCPScanPackage(t *testing.T) {
	version := "4.17.21"
	service := &fakeService{
		scanPackageFunc: func(ctx context.Context, request app.PackageRequest, progress func(app.Progress)) (model.ScanReport, error) {
			if request.Name != "lodash" || request.Version != "4.17.21" || request.Ecosystem != model.EcosystemNPM {
				t.Errorf("unexpected request: %+v", request)
			}
			return model.ScanReport{
				SchemaVersion: model.SchemaVersion,
				DataMode:      model.DataModeLive,
				ScannedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Status:        model.StatusFindings,
				Target: model.ScanTarget{
					Package:         model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "lodash"},
					ResolvedVersion: &version,
				},
				Summary: model.Summary{
					ConfirmedFindings: 1,
					HighestSeverity:   ptrSeverity(model.SeverityHigh),
				},
				Findings: []model.Finding{
					{
						ID:          "GHSA-35jh-9r23-6j32",
						Kind:        model.FindingConfirmed,
						Title:       "Prototype Pollution in lodash",
						Description: "Regular expression denial of service / prototype pollution vulnerability",
						Severity:    model.SeverityHigh,
						Package:     model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "lodash"},
						Version:     &version,
						Risk: &model.RiskAssessment{
							Score:     7.5,
							Rationale: "Known CVE advisory in widely used utility package.",
						},
						Evidence: []model.Evidence{
							{
								Kind:      model.EvidenceAdvisory,
								Summary:   "OSV GHSA-35jh-9r23-6j32",
								SourceID:  "osv:npm:lodash@4.17.21",
								Reference: "GHSA-35jh-9r23-6j32",
							},
						},
					},
				},
				Sources: []model.SourceReference{
					{
						ID:          "osv:npm:lodash@4.17.21",
						Provider:    model.SourceProviderOSV,
						Identifier:  "GHSA-35jh-9r23-6j32",
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

	// 1. 成功调用 scan_package
	callRes, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_package",
		Arguments: map[string]any{
			"package":   "lodash",
			"version":   "4.17.21",
			"ecosystem": "npm",
		},
	})
	if err != nil {
		t.Fatalf("CallTool scan_package failed: %v", err)
	}

	pkgOutput := decodeStructuredContent[ScanPackageOutput](t, callRes)

	if pkgOutput.Target.Package.Name != "lodash" || pkgOutput.Target.ResolvedVersion == nil || *pkgOutput.Target.ResolvedVersion != "4.17.21" {
		t.Errorf("unexpected output target: %+v", pkgOutput.Target)
	}
	if len(pkgOutput.Findings) != 1 || pkgOutput.Findings[0].ID != "GHSA-35jh-9r23-6j32" {
		t.Errorf("unexpected findings: %+v", pkgOutput.Findings)
	}
	if pkgOutput.Summary.ConfirmedFindings != 1 {
		t.Errorf("confirmed findings = %d, want 1", pkgOutput.Summary.ConfirmedFindings)
	}

	// 2. 接着测试 get_finding_details 查询该发现
	detailsRes, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_finding_details",
		Arguments: map[string]any{
			"finding_id": "GHSA-35jh-9r23-6j32",
		},
	})
	if err != nil {
		t.Fatalf("CallTool get_finding_details failed: %v", err)
	}

	detailsOutput := decodeStructuredContent[GetFindingDetailsOutput](t, detailsRes)
	if detailsOutput.FindingID != "GHSA-35jh-9r23-6j32" || detailsOutput.Risk == nil || detailsOutput.Risk.Score != 7.5 {
		t.Errorf("unexpected details output: %+v", detailsOutput)
	}

	// 3. 测试 get_scan_summary
	summaryRes, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_scan_summary",
		Arguments: map[string]any{
			"scan_id": pkgOutput.ScanID,
		},
	})
	if err != nil {
		t.Fatalf("CallTool get_scan_summary failed: %v", err)
	}
	summaryOutput := decodeStructuredContent[GetScanSummaryOutput](t, summaryRes)
	if summaryOutput.Totals.ConfirmedFindings != 1 || summaryOutput.Totals.HighestSeverity == nil || *summaryOutput.Totals.HighestSeverity != "high" {
		t.Errorf("unexpected summary totals: %+v", summaryOutput.Totals)
	}

	// 4. 无效参数测试：空包名
	invalidCallRes, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_package",
		Arguments: map[string]any{
			"package": "",
		},
	})
	if err != nil {
		t.Fatalf("expected tool-level error handling, got wire error: %v", err)
	}
	if !invalidCallRes.IsError {
		t.Errorf("expected IsError=true for empty package name, got false")
	}

	// 5. 不支持的生态
	invalidEcoRes, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_package",
		Arguments: map[string]any{
			"package":   "cargo-pkg",
			"ecosystem": "cargo",
		},
	})
	if err != nil {
		t.Fatalf("expected tool-level error, got %v", err)
	}
	if !invalidEcoRes.IsError {
		t.Errorf("expected IsError=true for unsupported ecosystem 'cargo'")
	}
}

func TestMCPScanProjectAndPathRestrictions(t *testing.T) {
	tempDir := t.TempDir()
	allowedProject := filepath.Join(tempDir, "allowed-project")
	outsideProject := filepath.Join(tempDir, "outside-project")

	if err := os.Mkdir(allowedProject, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outsideProject, 0755); err != nil {
		t.Fatal(err)
	}

	version := "1.0.0"
	service := &fakeService{
		scanProjectFunc: func(ctx context.Context, request app.ProjectRequest, progress func(app.Progress)) (project.Result, error) {
			return project.Result{
				Root:      request.Path,
				ScannedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Status:    model.StatusFindings,
				Formats: []project.FormatInfo{
					{
						Ecosystem:          model.EcosystemNPM,
						Format:             project.FormatNPMPackageLock,
						ManifestPath:       filepath.Join(request.Path, "package.json"),
						LockfilePath:       filepath.Join(request.Path, "package-lock.json"),
						ResolvedPackages:   10,
						UnresolvedPackages: 0,
						Resolution:         project.DependencyResolutionComplete,
					},
				},
				Summary: model.Summary{
					ConfirmedFindings: 1,
					HighestSeverity:   ptrSeverity(model.SeverityCritical),
				},
				Findings: []model.Finding{
					{
						ID:       "PROJ-FINDING-1",
						Kind:     model.FindingConfirmed,
						Title:    "Remote Code Execution",
						Severity: model.SeverityCritical,
						Package:  model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "vuln-pkg"},
						Version:  &version,
						Evidence: []model.Evidence{
							{
								Kind:     model.EvidenceDependencyPath,
								Summary:  "project -> parent-pkg -> vuln-pkg",
								SourceID: "project:npm",
							},
						},
					},
				},
				FindingContexts: map[string]project.DependencyContext{
					"PROJ-FINDING-1": {
						Relationship: model.DependencyTransitive,
						Scopes:       []project.DependencyScope{project.DependencyScopeProduction},
						Resolution:   project.DependencyResolutionComplete,
						Evidence: []project.DependencyEvidence{
							{Path: filepath.Join(request.Path, "package.json"), Field: "dependencies"},
						},
					},
				},
			}, nil
		},
	}

	client, _, cleanup := setupTestMCPClientServer(t, service, []string{allowedProject})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. 扫描受允许的项目目录：成功
	res, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_project",
		Arguments: map[string]any{
			"path": allowedProject,
		},
	})
	if err != nil {
		t.Fatalf("scan_project permitted path failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("scan_project returned error: %+v", res)
	}

	projOutput := decodeStructuredContent[ScanProjectOutput](t, res)
	if projOutput.Root != allowedProject || len(projOutput.Findings) != 1 {
		t.Errorf("unexpected project output: %+v", projOutput)
	}
	if projOutput.Findings[0].Relationship != "transitive" || len(projOutput.Findings[0].Scopes) == 0 || projOutput.Findings[0].Scopes[0] != "production" {
		t.Errorf("unexpected finding context in brief: %+v", projOutput.Findings[0])
	}

	// 2. 查询该项目发现的完整详情 (包含 representative paths 与 evidence)
	detailsRes, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_finding_details",
		Arguments: map[string]any{
			"finding_id": "PROJ-FINDING-1",
		},
	})
	if err != nil {
		t.Fatalf("get_finding_details failed: %v", err)
	}
	detailsOutput := decodeStructuredContent[GetFindingDetailsOutput](t, detailsRes)
	if detailsOutput.DependencyContext == nil {
		t.Fatal("expected dependency context to be present, got nil")
	}
	if len(detailsOutput.DependencyContext.RepresentativePaths) == 0 {
		t.Errorf("expected representative paths in dependency context")
	}

	// 3. 扫描未受允许的目录：应被安全边界拒绝
	deniedRes, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_project",
		Arguments: map[string]any{
			"path": outsideProject,
		},
	})
	if err != nil {
		t.Fatalf("expected tool-level denial, got wire error: %v", err)
	}
	if !deniedRes.IsError {
		t.Errorf("expected IsError=true for path outside permitted roots")
	}

	// 4. 空路径输入
	emptyRes, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_project",
		Arguments: map[string]any{
			"path": "",
		},
	})
	if err != nil {
		t.Fatalf("expected tool-level error: %v", err)
	}
	if !emptyRes.IsError {
		t.Errorf("expected IsError=true for empty project path")
	}
}

func TestMCPCancellationAndTimeout(t *testing.T) {
	service := &fakeService{
		scanPackageFunc: func(ctx context.Context, request app.PackageRequest, progress func(app.Progress)) (model.ScanReport, error) {
			<-ctx.Done()
			return model.ScanReport{Status: model.StatusCancelled}, ctx.Err()
		},
	}

	client, _, cleanup := setupTestMCPClientServer(t, service, nil)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 预先取消

	res, err := client.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_package",
		Arguments: map[string]any{
			"package": "lodash",
		},
	})
	// 针对取消的上下文，SDK 或工具处理应反馈取消或错误
	if err == nil && (res == nil || !res.IsError) {
		t.Errorf("expected cancellation error, got res=%+v, err=nil", res)
	}
}

func ptrSeverity(s model.Severity) *model.Severity {
	return &s
}
