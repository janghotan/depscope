import * as assert from "assert";
import * as vscode from "vscode";
import { getApiBaseUrl, startApiProcess, stopApiProcess } from "../../apiProcess";
import { ReportWebviewManager } from "../../reportWebview";
import { GroupNode, isReportClean, isReportIncomplete, ScanTreeProvider } from "../../scanTreeProvider";
import { scanWorkspaceDependencies } from "../../workspaceScanner";

suite("DepScope Extension Integration Test Suite", () => {
  suiteSetup(async () => {
    // 确保 API 进程已就绪
    await startApiProcess();
  });

  suiteTeardown(() => {
    stopApiProcess();
  });

  test("API process is spawned on assigned port and reports active", () => {
    const url = getApiBaseUrl();
    assert.ok(url, "API base URL should be defined");
    assert.match(url!, /^http:\/\/127\.0\.0\.1:\d+$/, "Base URL format should be valid");
  });

  test("Extension commands are registered", async () => {
    const commands = await vscode.commands.getCommands(true);
    assert.ok(
      commands.includes("depscope.openScanWizard"),
      "depscope.openScanWizard should be registered"
    );
    assert.ok(
      commands.includes("depscope.scanWorkspace"),
      "depscope.scanWorkspace should be registered"
    );
    assert.ok(
      commands.includes("depscope.showReport"),
      "depscope.showReport should be registered"
    );
  });

  test("Scans npm-context fixture workspace and groups failed checks under Incomplete coverage", async () => {
    const baseUrl = getApiBaseUrl();
    assert.ok(baseUrl, "API base URL must exist");

    const reports = await scanWorkspaceDependencies(baseUrl!);
    assert.ok(reports.length > 0, "Should have scanned dependencies in the workspace");

    const devToolReport = reports.find((r) => r.target.package.name === "dev-tool");
    assert.ok(devToolReport, "dev-tool should be present in scanned reports");
    assert.strictEqual(
      isReportIncomplete(devToolReport!),
      true,
      "dev-tool should be classified as incomplete because registry check failed"
    );
    assert.strictEqual(
      isReportClean(devToolReport!),
      false,
      "dev-tool must not be classified as clean"
    );

    const sharedDevReport = reports.find((r) => r.target.package.name === "shared-dev");
    assert.ok(sharedDevReport, "shared-dev should be present in scanned reports");
    assert.strictEqual(
      isReportIncomplete(sharedDevReport!),
      true,
      "shared-dev should be classified as incomplete because registry check failed"
    );
    assert.strictEqual(
      isReportClean(sharedDevReport!),
      false,
      "shared-dev must not be classified as clean"
    );

    // 验证 TreeProvider 4 个组节点与包归属
    const provider = new ScanTreeProvider();
    provider.setReports(reports);

    const roots = (await provider.getChildren()) as GroupNode[];
    assert.strictEqual(roots.length, 4, "Should have 4 groups: Confirmed, Heuristic, Incomplete, Clean");

    const incompleteGroup = roots.find((g) => g.category === "incomplete");
    assert.ok(incompleteGroup, "Incomplete coverage group must exist");
    assert.strictEqual(incompleteGroup!.label, "Incomplete coverage");

    const incompleteNames = incompleteGroup!.reports.map((r) => r.target.package.name);
    assert.ok(incompleteNames.includes("dev-tool"), "dev-tool must be in Incomplete coverage group");
    assert.ok(incompleteNames.includes("shared-dev"), "shared-dev must be in Incomplete coverage group");

    const cleanGroup = roots.find((g) => g.category === "clean");
    assert.ok(cleanGroup, "Clean group must exist");
    const cleanNames = cleanGroup!.reports.map((r) => r.target.package.name);
    assert.strictEqual(
      cleanNames.includes("dev-tool"),
      false,
      "dev-tool must never appear in Clean group"
    );
    assert.strictEqual(
      cleanNames.includes("shared-dev"),
      false,
      "shared-dev must never appear in Clean group"
    );

    // 验证 Webview 报告 HTML 不再出现矛盾的 CLEAN 徽标，而是显示 INCOMPLETE COVERAGE (amber)
    const sharedDevHtml = ReportWebviewManager.renderHtml(sharedDevReport!);
    assert.ok(
      sharedDevHtml.includes("INCOMPLETE COVERAGE"),
      "shared-dev report HTML must show INCOMPLETE COVERAGE badge"
    );
    assert.ok(
      sharedDevHtml.includes("#e3b341"),
      "shared-dev report HTML must use amber style for incomplete badge"
    );
    assert.strictEqual(
      sharedDevHtml.includes('>CLEAN<'),
      false,
      "shared-dev report HTML must not show CLEAN badge"
    );

    const devToolHtml = ReportWebviewManager.renderHtml(devToolReport!);
    assert.ok(
      devToolHtml.includes("INCOMPLETE COVERAGE"),
      "dev-tool report HTML must show INCOMPLETE COVERAGE badge"
    );
    assert.ok(
      devToolHtml.includes("#e3b341"),
      "dev-tool report HTML must use amber style for incomplete badge"
    );
    assert.strictEqual(
      devToolHtml.includes('>CLEAN<'),
      false,
      "dev-tool report HTML must not show CLEAN badge"
    );

    // 触发 depscope.scanWorkspace 命令，验证整体命令流无异常
    await vscode.commands.executeCommand("depscope.scanWorkspace");
  });

  test("Verifies lodash and minimist finding details in /tmp/depscope-test", async () => {
    const baseUrl = getApiBaseUrl();
    assert.ok(baseUrl, "API base URL must exist");

    // 扫描 /tmp/depscope-test 中的清单
    const manifestPath = "/tmp/depscope-test/package.json";
    const reports = await scanWorkspaceDependencies(baseUrl!, [manifestPath]);
    assert.ok(reports.length >= 2, "Should scan both lodash and minimist");

    const lodashReport = reports.find((r) => r.target.package.name === "lodash");
    assert.ok(lodashReport, "lodash report must exist");
    assert.ok(lodashReport!.findings.length > 0, "lodash must have findings");

    const lodashHtml = ReportWebviewManager.renderHtml(lodashReport!);
    // 1. 不应包含原始的 "## " markdown 语法，而是转为 md-heading
    assert.strictEqual(
      lodashHtml.includes("## Recommendation"),
      false,
      "Raw markdown header '## Recommendation' must not be printed literally"
    );
    assert.ok(
      lodashHtml.includes('<h4 class="md-heading">Recommendation</h4>'),
      "Markdown header must be converted to h4.md-heading"
    );

    // 2. References 列表应收敛为结构化折叠列表，而非长字符串堆砌
    assert.ok(
      lodashHtml.includes('class="references-box"'),
      "Should contain structured references-box"
    );
    assert.ok(
      lodashHtml.includes('class="references-details"'),
      "Should contain collapsible references-details"
    );
    assert.ok(
      lodashHtml.includes("Show "),
      "Should contain 'Show N more references' summary"
    );

    // 3. 验证排版样式：换行防溢出、最大行宽、代码标识
    assert.ok(
      lodashHtml.includes("word-break: break-word"),
      "Must have word-break styling"
    );
    assert.ok(
      lodashHtml.includes("max-width: 860px"),
      "Must have max-width constraint for readable line length"
    );
    assert.ok(
      lodashHtml.includes("advisory-link"),
      "Must have advisory-link class"
    );

    // 4. minimist 验证与 Webview 打开
    const minimistReport = reports.find((r) => r.target.package.name === "minimist");
    assert.ok(minimistReport, "minimist report must exist");
    const minimistHtml = ReportWebviewManager.renderHtml(minimistReport!);
    assert.ok(
      minimistHtml.includes('class="advisory-link"'),
      "minimist must render advisory ID in monospace link"
    );

    // 调用 showReport 命令触发 Webview 实际展示
    await vscode.commands.executeCommand("depscope.showReport", lodashReport);
    await vscode.commands.executeCommand("depscope.showReport", minimistReport);
  });

  test("Opening scan wizard executes without error", async () => {
    await vscode.commands.executeCommand("depscope.openScanWizard");
  });
});
