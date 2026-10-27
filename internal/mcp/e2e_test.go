// Package mcp 的端到端真实子进程 stdio 协议测试。
// 使用官方 Go MCP SDK 的 CommandTransport 启动编译出的二进制并验证 JSON-RPC 协议流。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func findProjectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for dir := wd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
	}
	t.Fatal("could not find project root containing go.mod")
	return ""
}

func ensureBinaryBuilt(t *testing.T, relPath, mainPkg string) string {
	t.Helper()
	root := findProjectRoot(t)
	binPath := filepath.Join(root, relPath)
	if _, err := os.Stat(binPath); err == nil {
		return binPath
	}

	// 尝试编译
	cmd := exec.Command("go", "build", "-trimpath", "-o", binPath, mainPkg)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\noutput: %s", mainPkg, err, out)
	}
	return binPath
}

func TestEndToEndStdioProtocol_DepScopeCLI(t *testing.T) {
	binPath := ensureBinaryBuilt(t, "bin/depscope", "./cmd/depscope")
	root := findProjectRoot(t)

	// 创建带合法 npm package-lock 的临时受允许目录
	testDir := t.TempDir()
	packageJSON := `{"name":"e2e-project","version":"1.0.0"}`
	packageLockJSON := `{"lockfileVersion":3,"packages":{"":{"name":"e2e-project","version":"1.0.0"}}}`
	if err := os.WriteFile(filepath.Join(testDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "package-lock.json"), []byte(packageLockJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "mcp", "--allowed-path", testDir)
	cmd.Dir = root
	transport := &sdk.CommandTransport{Command: cmd}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := sdk.NewClient(&sdk.Implementation{Name: "e2e-test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("client.Connect over stdio subprocess: %v", err)
	}
	defer session.Close()

	// 1. 测试工具发现
	toolsRes, err := session.ListTools(ctx, &sdk.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(toolsRes.Tools) < 4 {
		t.Fatalf("expected at least 4 tools, got %d", len(toolsRes.Tools))
	}

	// 2. 测试 scan_project 在真实子进程中的执行
	scanProjRes, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_project",
		Arguments: map[string]any{
			"path": testDir,
		},
	})
	if err != nil {
		t.Fatalf("CallTool scan_project over stdio: %v", err)
	}
	if scanProjRes.IsError {
		t.Fatalf("scan_project returned error: %+v", scanProjRes)
	}

	var projOutput ScanProjectOutput
	data, _ := json.Marshal(scanProjRes.StructuredContent)
	if err := json.Unmarshal(data, &projOutput); err != nil {
		t.Fatalf("unmarshal scan_project: %v", err)
	}
	if projOutput.Root != testDir || projOutput.Status != "clean" {
		t.Errorf("unexpected projOutput: root=%q, status=%q", projOutput.Root, projOutput.Status)
	}

	// 3. 测试 get_scan_summary 查验上一步缓存
	summaryRes, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_scan_summary",
		Arguments: map[string]any{
			"scan_id": projOutput.ScanID,
		},
	})
	if err != nil {
		t.Fatalf("CallTool get_scan_summary over stdio: %v", err)
	}
	var summaryOutput GetScanSummaryOutput
	sumData, _ := json.Marshal(summaryRes.StructuredContent)
	if err := json.Unmarshal(sumData, &summaryOutput); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if summaryOutput.TargetName != testDir || summaryOutput.Totals.ConfirmedFindings != 0 {
		t.Errorf("unexpected summary: %+v", summaryOutput)
	}

	// 4. 测试安全边界：拒绝受允许范围之外的目录
	outsideDir := t.TempDir()
	deniedRes, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_project",
		Arguments: map[string]any{
			"path": outsideDir,
		},
	})
	if err != nil {
		t.Fatalf("expected tool-level refusal, got wire error: %v", err)
	}
	if !deniedRes.IsError {
		t.Errorf("expected IsError=true for unallowed directory %q", outsideDir)
	}

	// 5. 测试 get_finding_details 查询不存在的发现
	notFoundRes, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "get_finding_details",
		Arguments: map[string]any{
			"finding_id": "NONEXISTENT-FINDING-ID",
		},
	})
	if err != nil {
		t.Fatalf("expected tool error: %v", err)
	}
	if !notFoundRes.IsError {
		t.Errorf("expected IsError=true for nonexistent finding ID")
	}
}

func TestEndToEndStdioProtocol_StandaloneMCP(t *testing.T) {
	binPath := ensureBinaryBuilt(t, "bin/depscope-mcp", "./cmd/depscope-mcp")
	root := findProjectRoot(t)

	testDir := t.TempDir()
	reqTxt := "requests==2.31.0\n"
	if err := os.WriteFile(filepath.Join(testDir, "requirements.txt"), []byte(reqTxt), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "--allowed-path", testDir)
	cmd.Dir = root
	transport := &sdk.CommandTransport{Command: cmd}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := sdk.NewClient(&sdk.Implementation{Name: "standalone-e2e-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("client.Connect over stdio to standalone binary: %v", err)
	}
	defer session.Close()

	// 工具发现
	toolsRes, err := session.ListTools(ctx, &sdk.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	found := false
	for _, tool := range toolsRes.Tools {
		if tool.Name == "scan_project" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("scan_project tool missing on standalone binary")
	}

	// 路径遍历拒绝测试
	badPath := fmt.Sprintf("%s/../", testDir)
	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name: "scan_project",
		Arguments: map[string]any{
			"path": badPath,
		},
	})
	if err != nil {
		t.Fatalf("expected tool-level error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected path traversal attempt to be rejected with IsError=true")
	}
}
