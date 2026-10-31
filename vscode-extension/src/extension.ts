import * as vscode from "vscode";
import {
  getApiBaseUrl,
  hasWorkspaceManifests,
  startApiProcess,
  stopApiProcess,
} from "./apiProcess";
import { clearDiagnostics, disposeDiagnostics, updateDiagnostics } from "./diagnostics";
import { ScanReport } from "./generated/report-types";
import { ReportWebviewManager } from "./reportWebview";
import { isReportClean, isReportIncomplete, ScanTreeProvider } from "./scanTreeProvider";
import { ScanWizardManager } from "./scanWizard";
import { scanWorkspaceDependencies } from "./workspaceScanner";

let statusBarItem: vscode.StatusBarItem | null = null;
let treeProvider: ScanTreeProvider | null = null;
let isScanning = false;

/**
 * 更新状态栏项文本、提示及图标样式。
 */
function updateStatusBar(
  status: "ready" | "scanning" | "clean" | "findings" | "incomplete" | "error",
  count?: number
): void {
  if (!statusBarItem) {
    return;
  }

  switch (status) {
    case "scanning":
      statusBarItem.text = "$(sync~spin) DepScope: Scanning...";
      statusBarItem.tooltip = "DepScope is scanning workspace dependencies";
      statusBarItem.backgroundColor = undefined;
      break;
    case "clean":
      statusBarItem.text = "$(pass) DepScope: Clean";
      statusBarItem.tooltip = "All dependencies are clean with verified coverage. Click to open scan wizard.";
      statusBarItem.backgroundColor = undefined;
      break;
    case "findings":
      statusBarItem.text = `$(error) DepScope: ${count || 0} Vuln(s)`;
      statusBarItem.tooltip = `DepScope detected ${count || 0} vulnerable packages. Click to view or rescan.`;
      statusBarItem.backgroundColor = new vscode.ThemeColor("statusBarItem.errorBackground");
      break;
    case "incomplete":
      statusBarItem.text = `$(warning) DepScope: ${count || 0} Incomplete`;
      statusBarItem.tooltip = `DepScope detected ${count || 0} package(s) with incomplete coverage. Click to view or rescan.`;
      statusBarItem.backgroundColor = new vscode.ThemeColor("statusBarItem.warningBackground");
      break;
    case "error":
      statusBarItem.text = "$(warning) DepScope: Error";
      statusBarItem.tooltip = "DepScope API or scan error. Click to open scan wizard and retry.";
      statusBarItem.backgroundColor = new vscode.ThemeColor("statusBarItem.warningBackground");
      break;
    case "ready":
    default:
      statusBarItem.text = "$(shield) DepScope: Ready";
      statusBarItem.tooltip = "DepScope: Click to open scan wizard";
      statusBarItem.backgroundColor = undefined;
      break;
  }
  statusBarItem.show();
}

/**
 * 执行工作区依赖扫描流程：
 * 1. 确保本地 depscope-api 进程正在运行并就绪；
 * 2. 收集工作区清单文件并并发调用 API 扫描；
 * 3. 刷新侧边栏四级分类树视图；
 * 4. 更新清单文件中问题行诊断 (Problems 面板)；
 * 5. 更新状态栏状态。
 */
async function runWorkspaceScan(): Promise<void> {
  if (isScanning) {
    void vscode.window.showInformationMessage("DepScope scan is already in progress.");
    return;
  }

  isScanning = true;
  updateStatusBar("scanning");

  try {
    // 确保 API 进程已就绪
    let baseUrl = getApiBaseUrl();
    if (!baseUrl) {
      baseUrl = await startApiProcess();
    }

    await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: "DepScope: Scanning workspace dependencies...",
        cancellable: true,
      },
      async (progress, token) => {
        const stepMap = (completed: number, total: number, currentPackage: string) => {
          progress.report({
            message: `${completed}/${total}: ${currentPackage}`,
            increment: total > 0 ? 100 / total : 0,
          });
        };

        const reports = await scanWorkspaceDependencies(baseUrl!, undefined, stepMap, token);

        if (token.isCancellationRequested) {
          updateStatusBar("ready");
          return;
        }

        // 更新侧边栏树视图
        if (treeProvider) {
          treeProvider.setReports(reports);
        }

        // 更新清单文件问题诊断
        await updateDiagnostics(reports);

        // 统计发现情况并更新状态栏
        const vulnerableCount = reports.filter((r) => r.findings && r.findings.length > 0).length;
        const incompleteCount = reports.filter(
          (r) => (!r.findings || r.findings.length === 0) && isReportIncomplete(r)
        ).length;
        const cleanCount = reports.filter((r) => isReportClean(r)).length;

        if (vulnerableCount > 0) {
          updateStatusBar("findings", vulnerableCount);
          void vscode.window.showWarningMessage(
            `DepScope: Scan complete. Found ${vulnerableCount} package(s) with security findings.`
          );
        } else if (incompleteCount > 0) {
          updateStatusBar("incomplete", incompleteCount);
          void vscode.window.showWarningMessage(
            `DepScope: Scan complete. ${incompleteCount} package(s) have incomplete coverage (not clean).`
          );
        } else if (cleanCount > 0) {
          updateStatusBar("clean");
          void vscode.window.showInformationMessage(
            `DepScope: Scan complete. All ${cleanCount} package(s) are clean.`
          );
        } else {
          updateStatusBar("ready");
          void vscode.window.showInformationMessage(
            "DepScope: No package.json or requirements.txt dependencies found to scan."
          );
        }
      }
    );
  } catch (err: unknown) {
    updateStatusBar("error");
    const message = err instanceof Error ? err.message : String(err);
    void vscode.window.showErrorMessage(`DepScope scan failed: ${message}`);
  } finally {
    isScanning = false;
  }
}

/**
 * 插件激活入口。
 */
export async function activate(context: vscode.ExtensionContext): Promise<void> {
  // 1. 注册侧边栏 TreeDataProvider
  treeProvider = new ScanTreeProvider();
  const treeView = vscode.window.registerTreeDataProvider("depscope.views.scanTree", treeProvider);
  context.subscriptions.push(treeView);

  // 2. 注册状态栏项（点击直接打开可视化扫描向导）
  statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
  statusBarItem.command = "depscope.openScanWizard";
  updateStatusBar("ready");
  context.subscriptions.push(statusBarItem);

  // 3. 注册命令: depscope.openScanWizard
  const openWizardCmd = vscode.commands.registerCommand("depscope.openScanWizard", async () => {
    if (treeProvider) {
      await ScanWizardManager.show(treeProvider, (status, count) => updateStatusBar(status, count));
    }
  });
  context.subscriptions.push(openWizardCmd);

  // 4. 注册命令: depscope.scanWorkspace
  const scanCmd = vscode.commands.registerCommand("depscope.scanWorkspace", async () => {
    await runWorkspaceScan();
  });
  context.subscriptions.push(scanCmd);

  // 5. 注册命令: depscope.showReport
  const showReportCmd = vscode.commands.registerCommand("depscope.showReport", (report: ScanReport) => {
    if (report) {
      ReportWebviewManager.show(report);
    }
  });
  context.subscriptions.push(showReportCmd);

  // 6. 检查工作区是否包含 package.json 或 requirements.txt
  if (hasWorkspaceManifests()) {
    // 异步启动 API 进程并就绪监听
    startApiProcess()
      .then(() => {
        updateStatusBar("ready");
      })
      .catch((err: unknown) => {
        const message = err instanceof Error ? err.message : String(err);
        console.warn("[DepScope] Initial API process spawn error:", message);
        updateStatusBar("error");
      });
  }
}

/**
 * 插件停用入口，清理子进程与诊断资源。
 */
export function deactivate(): void {
  stopApiProcess();
  clearDiagnostics();
  disposeDiagnostics();
  if (statusBarItem) {
    statusBarItem.dispose();
    statusBarItem = null;
  }
  treeProvider = null;
}
