import * as vscode from "vscode";
import { getApiBaseUrl, startApiProcess } from "./apiProcess";
import { updateDiagnostics } from "./diagnostics";
import { ScanReport } from "./generated/report-types";
import { isReportClean, isReportIncomplete, ScanTreeProvider } from "./scanTreeProvider";
import { DiscoveredManifest, getDiscoveredManifests, scanWorkspaceDependencies } from "./workspaceScanner";

/**
 * ScanWizardManager 管理可视化扫描向导 Webview 的展示与多步骤状态交互。
 */
export class ScanWizardManager {
  private static currentPanel: vscode.WebviewPanel | null = null;

  /**
   * 打开或唤起扫描向导面板。
   */
  public static async show(
    treeProvider: ScanTreeProvider,
    onStatusChange?: (
      status: "ready" | "scanning" | "clean" | "findings" | "incomplete" | "error",
      count?: number
    ) => void
  ): Promise<void> {
    const title = "DepScope: Scan Wizard";

    if (ScanWizardManager.currentPanel) {
      ScanWizardManager.currentPanel.reveal(vscode.ViewColumn.One);
      return;
    }

    const panel = vscode.window.createWebviewPanel(
      "depscopeScanWizard",
      title,
      vscode.ViewColumn.One,
      {
        enableScripts: true,
        retainContextWhenHidden: true,
      }
    );

    ScanWizardManager.currentPanel = panel;

    panel.onDidDispose(() => {
      ScanWizardManager.currentPanel = null;
    });

    // 探测当前工作区清单
    const manifests = await getDiscoveredManifests();

    panel.webview.html = ScanWizardManager.getInitialHtml(manifests);

    let isScanning = false;

    panel.webview.onDidReceiveMessage(async (message) => {
      if (message.command === "startScan") {
        if (isScanning) {
          return;
        }
        isScanning = true;
        onStatusChange?.("scanning");

        const selectedPaths: string[] = message.selectedPaths || [];

        // 渲染进度页面
        panel.webview.postMessage({
          type: "stateChange",
          state: "scanning",
          total: 0,
          completed: 0,
          currentPackage: "Starting DepScope API...",
        });

        try {
          let baseUrl = getApiBaseUrl();
          if (!baseUrl) {
            baseUrl = await startApiProcess();
          }

          const reports: ScanReport[] = await scanWorkspaceDependencies(
            baseUrl,
            selectedPaths,
            (completed, total, currentPackage) => {
              panel.webview.postMessage({
                type: "progress",
                completed,
                total,
                currentPackage,
              });
            }
          );

          // 更新侧边栏树视图与代码问题诊断
          treeProvider.setReports(reports);
          await updateDiagnostics(reports);

          const vulnerableCount = reports.filter((r) => r.findings && r.findings.length > 0).length;
          const incompleteCount = reports.filter(
            (r) => (!r.findings || r.findings.length === 0) && isReportIncomplete(r)
          ).length;
          const cleanCount = reports.filter((r) => isReportClean(r)).length;

          if (vulnerableCount > 0) {
            onStatusChange?.("findings", vulnerableCount);
          } else if (incompleteCount > 0) {
            onStatusChange?.("incomplete", incompleteCount);
          } else if (cleanCount > 0) {
            onStatusChange?.("clean");
          } else {
            onStatusChange?.("ready");
          }

          // 自动显示完成状态
          panel.webview.postMessage({
            type: "stateChange",
            state: "completed",
            total: reports.length,
            vulnerableCount,
            incompleteCount,
            cleanCount,
          });

          // 自动聚焦或展开 DepScope 树视图
          void vscode.commands.executeCommand("depscope.views.scanTree.focus");
        } catch (err: unknown) {
          onStatusChange?.("error");
          const errorMsg = err instanceof Error ? err.message : String(err);
          panel.webview.postMessage({
            type: "stateChange",
            state: "error",
            error: errorMsg,
          });
        } finally {
          isScanning = false;
        }
      }
    });
  }

  /**
   * 生成向导 HTML 模板。
   */
  private static getInitialHtml(manifests: DiscoveredManifest[]): string {
    const manifestRows =
      manifests.length === 0
        ? `<div class="empty-state">No package.json or requirements.txt detected in current workspace.</div>`
        : manifests
            .map(
              (m) => `
          <label class="manifest-card">
            <input type="checkbox" name="manifest" value="${escapeHtml(m.path)}" checked />
            <div class="manifest-info">
              <div class="manifest-name">
                <span class="badge ${m.type === "npm" ? "badge-npm" : "badge-pypi"}">${escapeHtml(
                m.type.toUpperCase()
              )}</span>
                <strong>${escapeHtml(m.relativePath)}</strong>
              </div>
              <div class="manifest-meta">${m.dependencyCount} declared dependencies</div>
            </div>
          </label>
        `
            )
            .join("");

    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>DepScope Scan Wizard</title>
  <style>
    :root {
      --color-brand: #7ee787;
      --color-brand-dim: #4f9d5c;
      --color-danger: #ff7b72;
      --color-warn: #e3b341;
      --color-muted: #8b949e;
      --border-color: var(--vscode-widget-border, #30363d);
    }
    body {
      font-family: var(--vscode-font-family, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif);
      font-size: var(--vscode-font-size, 13px);
      color: var(--vscode-editor-foreground, #c9d1d9);
      background-color: var(--vscode-editor-background, #0d1117);
      padding: 32px 40px;
      margin: 0;
      max-width: 720px;
      margin: 0 auto;
    }
    h1 {
      font-size: 1.8em;
      margin-top: 0;
      margin-bottom: 8px;
      display: flex;
      align-items: center;
      gap: 12px;
      color: var(--vscode-editor-foreground, #f0f6fc);
    }
    .subtitle {
      color: var(--color-muted);
      margin-bottom: 28px;
      font-size: 1.05em;
    }
    .step-box {
      background: var(--vscode-editorWidget-background, #161b22);
      border: 1px solid var(--border-color);
      border-radius: 8px;
      padding: 24px;
      margin-bottom: 24px;
    }
    .manifest-list {
      display: flex;
      flex-direction: column;
      gap: 10px;
      margin: 16px 0;
    }
    .manifest-card {
      display: flex;
      align-items: center;
      gap: 14px;
      padding: 12px 16px;
      background: rgba(255, 255, 255, 0.02);
      border: 1px solid var(--border-color);
      border-radius: 6px;
      cursor: pointer;
      transition: background 0.15s ease;
    }
    .manifest-card:hover {
      background: rgba(255, 255, 255, 0.05);
    }
    .manifest-card input[type="checkbox"] {
      cursor: pointer;
      width: 16px;
      height: 16px;
    }
    .manifest-info {
      flex: 1;
    }
    .manifest-name {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 1.05em;
    }
    .manifest-meta {
      color: var(--color-muted);
      font-size: 0.85em;
      margin-top: 2px;
    }
    .badge {
      display: inline-block;
      padding: 2px 6px;
      border-radius: 4px;
      font-size: 0.75em;
      font-weight: 600;
      letter-spacing: 0.5px;
    }
    .badge-npm { background: rgba(203, 56, 55, 0.2); color: #ff7b72; border: 1px solid rgba(203, 56, 55, 0.3); }
    .badge-pypi { background: rgba(55, 117, 169, 0.2); color: #79c0ff; border: 1px solid rgba(55, 117, 169, 0.3); }

    .btn-primary {
      background-color: var(--color-brand);
      color: #0d1519;
      font-weight: 600;
      padding: 10px 24px;
      border: none;
      border-radius: 6px;
      cursor: pointer;
      font-size: 1em;
      display: inline-flex;
      align-items: center;
      gap: 8px;
      transition: opacity 0.15s ease;
    }
    .btn-primary:hover {
      opacity: 0.9;
    }
    .btn-primary:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    /* 扫描中动画与进度条 */
    .spinner {
      border: 3px solid rgba(255, 255, 255, 0.1);
      border-left-color: var(--color-brand);
      border-radius: 50%;
      width: 24px;
      height: 24px;
      animation: spin 1s linear infinite;
    }
    @keyframes spin {
      0% { transform: rotate(0deg); }
      100% { transform: rotate(360deg); }
    }
    .progress-bar-bg {
      background: rgba(255, 255, 255, 0.08);
      border-radius: 4px;
      height: 8px;
      width: 100%;
      overflow: hidden;
      margin: 16px 0;
    }
    .progress-bar-fill {
      background: var(--color-brand);
      height: 100%;
      width: 0%;
      transition: width 0.2s ease;
    }
    .error-box {
      background: rgba(255, 123, 114, 0.1);
      border: 1px solid var(--color-danger);
      border-radius: 6px;
      padding: 16px;
      color: #f0f6fc;
      margin-top: 16px;
      white-space: pre-wrap;
      font-family: monospace;
      font-size: 0.9em;
    }
    .completed-box {
      background: rgba(126, 231, 135, 0.1);
      border: 1px solid var(--color-brand);
      border-radius: 6px;
      padding: 16px;
      color: #f0f6fc;
      margin-top: 16px;
    }
    .empty-state {
      padding: 24px;
      text-align: center;
      color: var(--color-muted);
    }
  </style>
</head>
<body>
  <h1>
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor">
      <circle cx="12" cy="12" r="9.5" stroke="#7ee787" stroke-width="2"/>
      <path d="M12 2.5v2.5M12 19v2.5M2.5 12h2.5M19 12h2.5" stroke="#7ee787" stroke-width="2" stroke-linecap="round"/>
      <rect x="9.2" y="9.2" width="5.6" height="5.6" stroke="#c9d1d9" stroke-width="1.8"/>
      <path d="M12 12v4.8" stroke="#ff7b72" stroke-width="2" stroke-linecap="round"/>
    </svg>
    DepScope Workspace Scanner
  </h1>
  <div class="subtitle">Inspect dependencies, detect known vulnerabilities, and analyze supply-chain risk.</div>

  <div id="step-select" class="step-box">
    <h3>Step 1: Confirm Detected Manifests</h3>
    <p>Select which manifests in your workspace you would like to inspect:</p>
    <div class="manifest-list">
      ${manifestRows}
    </div>
    <div style="margin-top: 20px;">
      <button id="btn-scan" class="btn-primary" ${manifests.length === 0 ? "disabled" : ""}>
        Scan Workspace
      </button>
    </div>
  </div>

  <div id="step-scanning" class="step-box" style="display: none;">
    <div style="display: flex; align-items: center; gap: 12px;">
      <div class="spinner"></div>
      <h3 style="margin: 0;" id="scan-heading">Scanning dependencies...</h3>
    </div>
    <div class="progress-bar-bg">
      <div id="progress-fill" class="progress-bar-fill"></div>
    </div>
    <div id="scan-status-text" style="color: var(--color-muted); font-size: 0.9em;">
      Initializing scan engine...
    </div>
  </div>

  <div id="step-result" class="step-box" style="display: none;"></div>

  <script>
    const vscode = acquireVsCodeApi();

    const btnScan = document.getElementById("btn-scan");
    const stepSelect = document.getElementById("step-select");
    const stepScanning = document.getElementById("step-scanning");
    const stepResult = document.getElementById("step-result");
    const progressFill = document.getElementById("progress-fill");
    const scanHeading = document.getElementById("scan-heading");
    const scanStatusText = document.getElementById("scan-status-text");

    btnScan.addEventListener("click", () => {
      const checkedBoxes = Array.from(document.querySelectorAll('input[name="manifest"]:checked'));
      const selectedPaths = checkedBoxes.map(cb => cb.value);

      stepSelect.style.display = "none";
      stepScanning.style.display = "block";
      stepResult.style.display = "none";

      vscode.postMessage({
        command: "startScan",
        selectedPaths: selectedPaths
      });
    });

    window.addEventListener("message", (event) => {
      const msg = event.data;
      if (msg.type === "progress") {
        const percent = msg.total > 0 ? Math.round((msg.completed / msg.total) * 100) : 0;
        progressFill.style.width = percent + "%";
        scanHeading.textContent = "Scanning " + msg.total + " packages (" + percent + "%)...";
        scanStatusText.textContent = "[" + msg.completed + "/" + msg.total + "] Scanning: " + msg.currentPackage;
      } else if (msg.type === "stateChange") {
        if (msg.state === "completed") {
          stepScanning.style.display = "none";
          stepResult.style.display = "block";

          let headerTitle = "✓ Scan Complete: All Clean";
          let headerColor = "var(--color-brand)";
          let boxBorder = "1px solid var(--color-brand)";
          let boxBg = "rgba(126, 231, 135, 0.1)";

          if (msg.vulnerableCount > 0) {
            headerTitle = "Scan Complete: Findings Detected";
            headerColor = "var(--color-danger)";
            boxBorder = "1px solid var(--color-danger)";
            boxBg = "rgba(255, 123, 114, 0.1)";
          } else if (msg.incompleteCount > 0) {
            headerTitle = "Scan Complete: Incomplete Coverage";
            headerColor = "var(--color-warn)";
            boxBorder = "1px solid var(--color-warn)";
            boxBg = "rgba(227, 179, 65, 0.1)";
          }

          let summaryLines = "<p>Successfully scanned " + msg.total + " package(s).</p>";
          if (msg.vulnerableCount > 0) {
            summaryLines += '<p style="color: var(--color-danger); font-weight: bold;">Found ' + msg.vulnerableCount + ' package(s) with security vulnerabilities.</p>';
          }
          if (msg.incompleteCount > 0) {
            summaryLines += '<p style="color: var(--color-warn); font-weight: bold;">' + msg.incompleteCount + ' package(s) have incomplete coverage (missing or failed checks). Not clean.</p>';
          }
          if (msg.vulnerableCount === 0 && msg.incompleteCount === 0 && msg.cleanCount > 0) {
            summaryLines += '<p style="color: var(--color-brand); font-weight: bold;">All ' + msg.cleanCount + ' package(s) are clean with verified coverage.</p>';
          } else if (msg.cleanCount > 0) {
            summaryLines += '<p style="color: var(--color-brand);">' + msg.cleanCount + ' package(s) are clean.</p>';
          }

          stepResult.innerHTML =
            '<div class="completed-box" style="border: ' + boxBorder + '; background: ' + boxBg + ';">' +
              '<h3 style="margin-top: 0; color: ' + headerColor + ';">' + headerTitle + '</h3>' +
              summaryLines +
              '<p style="color: var(--color-muted); margin-bottom: 0;">' +
                'The results are populated in the <strong>DepScope sidebar view</strong>.' +
              '</p>' +
            '</div>' +
            '<div style="margin-top: 16px;">' +
              '<button class="btn-primary" onclick="location.reload()">Scan Again</button>' +
            '</div>';
        } else if (msg.state === "error") {
          stepScanning.style.display = "none";
          stepResult.style.display = "block";
          stepResult.innerHTML =
            '<h3 style="color: var(--color-danger); margin-top: 0;">Scan Failed</h3>' +
            '<div class="error-box">' + escapeHtml(msg.error) + '</div>' +
            '<div style="margin-top: 16px;">' +
              '<button class="btn-primary" onclick="location.reload()">Retry</button>' +
            '</div>';
        }
      }
    });

    function escapeHtml(text) {
      if (!text) return "";
      return text
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
    }
  </script>
</body>
</html>`;
  }
}

function escapeHtml(text: string | null | undefined): string {
  if (!text) {
    return "";
  }
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}
