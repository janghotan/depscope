import * as vscode from "vscode";
import { CheckResult, Evidence, Finding, ScanReport, SourceReference } from "./generated/report-types";
import { isReportIncomplete } from "./statusUtils";

/**
 * 转义 HTML 文本，防止在 Webview 中发生脚本注入。
 */
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

/**
 * 根据严重级别获取对应 CSS 颜色类名与语义色值。
 */
function getSeverityBadgeStyle(severity: string | null | undefined): { color: string; bg: string } {
  switch (severity?.toLowerCase()) {
    case "critical":
    case "high":
      return { color: "#ff7b72", bg: "rgba(255, 123, 114, 0.15)" };
    case "medium":
      return { color: "#e3b341", bg: "rgba(227, 179, 65, 0.15)" };
    case "low":
      return { color: "#79c0ff", bg: "rgba(121, 192, 255, 0.15)" };
    case "unknown":
      return { color: "#8b949e", bg: "rgba(139, 148, 158, 0.15)" };
    default:
      return { color: "#7ee787", bg: "rgba(126, 231, 135, 0.15)" };
  }
}

/**
 * 为已知漏洞库前缀生成 OSV 或 GitHub Advisory 规范超链接。
 */
function getAdvisoryUrl(id: string): string {
  if (id.startsWith("GHSA-")) {
    return `https://github.com/advisories/${encodeURIComponent(id)}`;
  }
  return `https://osv.dev/vulnerability/${encodeURIComponent(id)}`;
}

/**
 * 将 OSV 返回的 markdown/纯文本描述解析渲染为结构化的安全说明 HTML：
 * 1. 消除原生的 "## Header" markdown 语法并渲染为带样式的子标题；
 * 2. 提取并分离 References 列表，转为带可折叠控制 ("Show N more references") 的链接列表；
 * 3. 渲染内联代码块、加粗文本及元数据字段；
 * 4. 彻底防止长 URL 及纯文本溢出。
 */
function renderFindingDescription(rawDescription: string | null | undefined): string {
  if (!rawDescription) {
    return "";
  }

  const lines = rawDescription.split(/\r?\n/);
  const bodyHtmlParts: string[] = [];
  const referenceUrls: string[] = [];
  let inReferencesBlock = false;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line) {
      inReferencesBlock = false;
      continue;
    }

    // 检查是否进入 References 行
    if (line.startsWith("References:")) {
      inReferencesBlock = true;
      const urls = line.substring("References:".length).match(/https?:\/\/[^\s]+/g);
      if (urls) {
        referenceUrls.push(...urls);
      }
      continue;
    }

    if (inReferencesBlock) {
      const urls = line.match(/https?:\/\/[^\s]+/g);
      if (urls) {
        referenceUrls.push(...urls);
        continue;
      } else {
        inReferencesBlock = false;
      }
    }

    // 处理 markdown 标题 (## Recommendation 等)
    const headerMatch = line.match(/^(#{1,6})\s+(.+)$/);
    if (headerMatch) {
      bodyHtmlParts.push(`<h4 class="md-heading">${escapeHtml(headerMatch[2])}</h4>`);
      continue;
    }

    // 处理结构化元数据行 (Aliases, Affected ranges, OSV severity)
    const metaMatch = line.match(/^(Aliases|Affected ranges|OSV severity):\s*(.+)$/i);
    if (metaMatch) {
      bodyHtmlParts.push(
        `<div class="md-meta-line"><strong>${escapeHtml(metaMatch[1])}:</strong> ${escapeHtml(
          metaMatch[2]
        )}</div>`
      );
      continue;
    }

    // 常规段落：转义后渲染行内代码及加粗样式
    let escaped = escapeHtml(line);
    escaped = escaped.replace(/`([^`]+)`/g, `<code class="md-code">$1</code>`);
    escaped = escaped.replace(/\*\*([^*]+)\*\*/g, `<strong>$1</strong>`);
    bodyHtmlParts.push(`<p class="md-paragraph">${escaped}</p>`);
  }

  let finalHtml = bodyHtmlParts.join("\n");

  // 若提取到了 References URL 列表，按折叠形式渲染
  if (referenceUrls.length > 0) {
    const firstThree = referenceUrls.slice(0, 3);
    const remaining = referenceUrls.slice(3);

    let refsHtml = `
      <div class="references-box">
        <div class="references-header">References (${referenceUrls.length})</div>
        <ul class="references-list">
          ${firstThree
            .map(
              (u) =>
                `<li><a class="link" href="${escapeHtml(u)}">${escapeHtml(u)}</a></li>`
            )
            .join("")}
        </ul>
    `;

    if (remaining.length > 0) {
      refsHtml += `
        <details class="references-details">
          <summary class="references-summary">Show ${remaining.length} more references</summary>
          <ul class="references-list">
            ${remaining
              .map(
                (u) =>
                  `<li><a class="link" href="${escapeHtml(u)}">${escapeHtml(u)}</a></li>`
              )
              .join("")}
          </ul>
        </details>
      `;
    }

    refsHtml += `</div>`;
    finalHtml += "\n" + refsHtml;
  }

  return finalHtml;
}

/**
 * ReportWebviewManager 管理扫描报告 Webview 面板的创建与生命周期。
 */
export class ReportWebviewManager {
  private static currentPanel: vscode.WebviewPanel | null = null;

  /**
   * 打开或切换到指定包的扫描报告详情面板。
   */
  public static show(report: ScanReport): void {
    const pkg = report.target.package;
    const version = report.target.resolvedVersion || "latest";
    const title = `DepScope: ${pkg.name}@${version}`;

    if (ReportWebviewManager.currentPanel) {
      ReportWebviewManager.currentPanel.title = title;
      ReportWebviewManager.currentPanel.webview.html = ReportWebviewManager.renderHtml(report);
      ReportWebviewManager.currentPanel.reveal(vscode.ViewColumn.Beside);
      return;
    }

    const panel = vscode.window.createWebviewPanel(
      "depscopeReport",
      title,
      vscode.ViewColumn.Beside,
      {
        enableScripts: false,
        retainContextWhenHidden: true,
      }
    );

    panel.onDidDispose(() => {
      ReportWebviewManager.currentPanel = null;
    });

    panel.webview.html = ReportWebviewManager.renderHtml(report);
    ReportWebviewManager.currentPanel = panel;
  }

  /**
   * 将 ScanReport v1 渲染为完整的 HTML 页面内容。
   */
  public static renderHtml(report: ScanReport): string {
    const pkg = report.target.package;
    const version = report.target.resolvedVersion || "latest";
    const incomplete = isReportIncomplete(report);

    let statusBadgeText: string;
    let statusBadgeStyle: { color: string; bg: string };

    if (report.summary.highestSeverity) {
      statusBadgeText = report.summary.highestSeverity.toUpperCase();
      statusBadgeStyle = getSeverityBadgeStyle(report.summary.highestSeverity);
    } else if (incomplete) {
      statusBadgeText = "INCOMPLETE COVERAGE";
      statusBadgeStyle = { color: "#e3b341", bg: "rgba(227, 179, 65, 0.15)" };
    } else {
      statusBadgeText = "CLEAN";
      statusBadgeStyle = { color: "#7ee787", bg: "rgba(126, 231, 135, 0.15)" };
    }

    const findings = report.findings || [];
    const checks = report.checks || [];
    const sources = report.sources || [];

    const findingsHtml =
      findings.length === 0
        ? incomplete
          ? `
        <div class="card" style="border-left: 4px solid var(--color-warn);">
          <div style="display: flex; align-items: center; gap: 14px;">
            <div style="font-size: 2em; color: var(--color-warn); font-weight: bold;">⚠</div>
            <div>
              <h3 style="margin: 0 0 4px 0; color: var(--color-warn);">Incomplete Coverage</h3>
              <p style="margin: 0; color: var(--vscode-editor-foreground);">
                Coverage is incomplete; this result is not clean. Missing or unsuccessful checks prevent a clean result.
              </p>
            </div>
          </div>
        </div>
      `
          : `
        <div class="card clean-card">
          <div class="clean-icon">✓</div>
          <div>
            <h3 style="margin: 0 0 4px 0;">Clean Package</h3>
            <p style="margin: 0; color: var(--color-muted);">
              All coverage checks succeeded and no confirmed vulnerabilities or heuristic signals were detected.
            </p>
          </div>
        </div>
      `
        : findings
            .map((finding: Finding) => {
              const findingSev = getSeverityBadgeStyle(finding.severity);
              const advisoryUrl = getAdvisoryUrl(finding.id);
              const isConfirmed = finding.kind === "confirmed";
              const confidenceLabel = isConfirmed
                ? "Confirmed (OSV-matched)"
                : "Heuristic signal (Inspection Signal)";
              const confidenceClass = isConfirmed ? "conf-confirmed" : "conf-heuristic";

              const evidenceList = (finding.evidence || [])
                .map(
                  (ev: Evidence) => `
                    <li class="evidence-item">
                      <span class="evidence-kind">[${escapeHtml(ev.kind)}]</span>
                      <span class="evidence-summary">${escapeHtml(ev.summary)}</span>
                      ${
                        ev.reference
                          ? `<a class="link" href="${escapeHtml(ev.reference)}">${escapeHtml(
                              ev.reference
                            )}</a>`
                          : ""
                      }
                    </li>
                  `
                )
                .join("");

              return `
                <div class="card finding-card">
                  <div class="finding-header">
                    <div class="finding-title-row">
                      <a class="advisory-link" href="${advisoryUrl}">${escapeHtml(finding.id)}</a>
                      <span class="badge" style="color: ${findingSev.color}; background: ${findingSev.bg};">
                        ${escapeHtml(finding.severity.toUpperCase())}
                      </span>
                      <span class="badge ${confidenceClass}">${confidenceLabel}</span>
                    </div>
                    <h3 class="finding-title">${escapeHtml(finding.title)}</h3>
                  </div>

                  <div class="finding-desc">${renderFindingDescription(finding.description)}</div>

                  ${
                    finding.remediation
                      ? `
                    <div class="remediation-box">
                      <strong>Remediation:</strong> ${escapeHtml(finding.remediation)}
                    </div>
                  `
                      : ""
                  }

                  ${
                    finding.risk
                      ? `
                    <div class="risk-box">
                      <strong>Risk Assessment (Score: ${finding.risk.score}/10):</strong>
                      <span>${escapeHtml(finding.risk.rationale)}</span>
                    </div>
                  `
                      : ""
                  }

                  <div class="section-sub">
                    <span class="sub-heading">Evidence (${finding.evidence.length})</span>
                    <ul class="evidence-list">
                      ${evidenceList}
                    </ul>
                  </div>
                </div>
              `;
            })
            .join("");

    const checksHtml = checks
      .map((c: CheckResult) => {
        let statusBadge = `<span class="badge badge-success">OK</span>`;
        if (c.status === "failed") {
          statusBadge = `<span class="badge badge-danger">FAILED</span>`;
        } else if (c.status === "unavailable") {
          statusBadge = `<span class="badge badge-warn">UNAVAILABLE</span>`;
        } else if (c.status === "skipped") {
          statusBadge = `<span class="badge badge-dim">SKIPPED</span>`;
        }
        return `
          <div class="check-row">
            <span class="check-name">${escapeHtml(c.name)}</span>
            ${statusBadge}
            <span class="check-msg">${escapeHtml(c.message)}</span>
          </div>
        `;
      })
      .join("");

    const sourcesHtml = sources
      .map(
        (s: SourceReference) => `
        <div class="source-tag">
          <strong>${escapeHtml(s.provider)}:</strong>
          ${s.url ? `<a class="link" href="${escapeHtml(s.url)}">${escapeHtml(s.identifier)}</a>` : escapeHtml(s.identifier)}
        </div>
      `
      )
      .join("");

    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>${escapeHtml(pkg.name)}@${escapeHtml(version)}</title>
  <style>
    :root {
      --color-brand: #7ee787;
      --color-danger: #ff7b72;
      --color-warn: #e3b341;
      --color-info: #79c0ff;
      --color-muted: #8b949e;
      --border-color: var(--vscode-widget-border, #30363d);
    }
    * {
      box-sizing: border-box;
    }
    body {
      font-family: var(--vscode-font-family, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif);
      font-size: var(--vscode-font-size, 13px);
      line-height: 1.6;
      color: var(--vscode-editor-foreground, #c9d1d9);
      background-color: var(--vscode-editor-background, #0d1117);
      padding: 24px 32px;
      margin: 0 auto;
      max-width: 860px;
      word-break: break-word;
      overflow-wrap: break-word;
    }
    a.link {
      color: var(--vscode-textLink-foreground, #58a6ff);
      text-decoration: none;
      word-break: break-all;
      overflow-wrap: break-word;
    }
    a.link:hover {
      text-decoration: underline;
    }
    .header-bar {
      border-bottom: 1px solid var(--border-color);
      padding-bottom: 18px;
      margin-bottom: 28px;
    }
    .header-top {
      display: flex;
      align-items: center;
      gap: 12px;
      flex-wrap: wrap;
    }
    .pkg-title {
      font-size: 1.6em;
      font-weight: 600;
      margin: 0;
      color: var(--vscode-editor-foreground, #f0f6fc);
      letter-spacing: -0.3px;
    }
    .meta-row {
      display: flex;
      align-items: center;
      gap: 14px;
      margin-top: 10px;
      color: var(--color-muted);
      font-size: 0.9em;
      flex-wrap: wrap;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      padding: 3px 8px;
      border-radius: 4px;
      font-size: 0.78em;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.6px;
      line-height: 1.2;
    }
    .badge-success { color: var(--color-brand); background: rgba(126, 231, 135, 0.15); }
    .badge-danger { color: var(--color-danger); background: rgba(255, 123, 114, 0.15); }
    .badge-warn { color: var(--color-warn); background: rgba(227, 179, 65, 0.15); }
    .badge-dim { color: var(--color-muted); background: rgba(139, 148, 158, 0.15); }
    .conf-confirmed { color: #f0883e; background: rgba(240, 136, 62, 0.15); border: 1px solid rgba(240, 136, 62, 0.3); }
    .conf-heuristic { color: #79c0ff; background: rgba(121, 192, 255, 0.15); border: 1px solid rgba(121, 192, 255, 0.3); }

    .card {
      background: var(--vscode-editorWidget-background, #161b22);
      border: 1px solid var(--border-color);
      border-radius: 6px;
      padding: 18px 20px;
      margin-bottom: 20px;
      word-break: break-word;
      overflow-wrap: break-word;
    }
    .clean-card {
      display: flex;
      align-items: center;
      gap: 16px;
      border-color: rgba(126, 231, 135, 0.3);
    }
    .clean-icon {
      font-size: 2.2em;
      color: var(--color-brand);
      font-weight: bold;
    }
    .finding-card {
      border-left: 4px solid var(--color-danger);
    }
    .finding-header {
      margin-bottom: 14px;
    }
    .finding-title-row {
      display: flex;
      align-items: center;
      gap: 10px;
      flex-wrap: wrap;
      margin-bottom: 8px;
    }
    .advisory-link {
      font-family: var(--vscode-editor-font-family, ui-monospace, SFMono-Regular, Consolas, Menlo, monospace);
      font-weight: 700;
      font-size: 1.05em;
      color: var(--vscode-textLink-foreground, #58a6ff);
      background: var(--vscode-textCodeBlock-background, rgba(110, 118, 129, 0.15));
      padding: 2px 8px;
      border-radius: 4px;
      border: 1px solid var(--border-color);
      text-decoration: none;
      display: inline-block;
      letter-spacing: 0.2px;
    }
    .advisory-link:hover {
      text-decoration: underline;
      background: rgba(110, 118, 129, 0.25);
    }
    .finding-title {
      margin: 6px 0 0 0;
      font-size: 1.2em;
      font-weight: 600;
      color: var(--vscode-editor-foreground, #f0f6fc);
    }
    .finding-desc {
      color: var(--vscode-editor-foreground, #c9d1d9);
      margin-bottom: 16px;
      font-size: 0.96em;
      line-height: 1.6;
    }
    .md-heading {
      margin: 18px 0 8px 0;
      color: var(--vscode-editor-foreground, #f0f6fc);
      font-weight: 600;
      font-size: 1.05em;
      border-bottom: 1px solid var(--border-color);
      padding-bottom: 4px;
    }
    .md-paragraph {
      margin: 8px 0;
    }
    .md-meta-line {
      margin: 4px 0;
      color: var(--color-muted);
      font-size: 0.9em;
    }
    .md-code {
      font-family: var(--vscode-editor-font-family, ui-monospace, monospace);
      font-size: 0.9em;
      padding: 2px 5px;
      background: var(--vscode-textCodeBlock-background, rgba(110, 118, 129, 0.2));
      border-radius: 4px;
      color: var(--vscode-editor-foreground, #f0f6fc);
    }
    .references-box {
      margin-top: 14px;
      background: rgba(255, 255, 255, 0.02);
      border: 1px solid var(--border-color);
      border-radius: 6px;
      padding: 10px 14px;
    }
    .references-header {
      font-weight: 600;
      font-size: 0.85em;
      color: var(--color-muted);
      text-transform: uppercase;
      margin-bottom: 6px;
      letter-spacing: 0.5px;
    }
    .references-list {
      list-style: none;
      padding: 0;
      margin: 0;
    }
    .references-list li {
      padding: 3px 0;
      font-size: 0.9em;
    }
    .references-details {
      margin-top: 6px;
    }
    .references-summary {
      cursor: pointer;
      font-size: 0.9em;
      color: var(--vscode-textLink-foreground, #58a6ff);
      font-weight: 500;
      user-select: none;
      padding: 2px 0;
    }
    .references-summary:hover {
      text-decoration: underline;
    }
    .remediation-box {
      background: rgba(126, 231, 135, 0.08);
      border: 1px solid rgba(126, 231, 135, 0.25);
      border-radius: 4px;
      padding: 10px 14px;
      margin-bottom: 14px;
      font-size: 0.95em;
    }
    .risk-box {
      background: rgba(227, 179, 65, 0.08);
      border: 1px solid rgba(227, 179, 65, 0.25);
      border-radius: 4px;
      padding: 10px 14px;
      margin-bottom: 14px;
      font-size: 0.95em;
    }
    .section-sub {
      margin-top: 16px;
      border-top: 1px dashed var(--border-color);
      padding-top: 12px;
    }
    .sub-heading {
      font-weight: 600;
      font-size: 0.85em;
      color: var(--color-muted);
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }
    .evidence-list {
      list-style: none;
      padding: 0;
      margin: 8px 0 0 0;
    }
    .evidence-item {
      padding: 5px 0;
      font-size: 0.9em;
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
      border-bottom: 1px solid rgba(255, 255, 255, 0.03);
    }
    .evidence-kind {
      color: var(--color-muted);
      font-family: monospace;
    }
    .evidence-summary {
      flex: 1;
    }
    .section-title {
      font-size: 1.15em;
      font-weight: 600;
      margin: 32px 0 14px 0;
      color: var(--vscode-editor-foreground, #f0f6fc);
      display: flex;
      align-items: center;
      gap: 8px;
      border-bottom: 1px solid var(--border-color);
      padding-bottom: 6px;
    }
    .check-row {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 8px 0;
      border-bottom: 1px solid rgba(255, 255, 255, 0.04);
      font-size: 0.9em;
    }
    .check-row:last-child {
      border-bottom: none;
    }
    .check-name {
      font-weight: 600;
      min-width: 140px;
    }
    .check-msg {
      color: var(--color-muted);
      flex: 1;
    }
    .sources-row {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      margin-top: 8px;
    }
    .source-tag {
      background: var(--vscode-editorWidget-background, #161b22);
      border: 1px solid var(--border-color);
      border-radius: 4px;
      padding: 6px 12px;
      font-size: 0.85em;
    }
  </style>
</head>
<body>
  <div class="header-bar">
    <div class="header-top">
      <span class="badge badge-dim">${escapeHtml(pkg.ecosystem)}</span>
      <h1 class="pkg-title">${escapeHtml(pkg.name)}@${escapeHtml(version)}</h1>
      <span class="badge" style="color: ${statusBadgeStyle.color}; background: ${statusBadgeStyle.bg};">
        ${escapeHtml(statusBadgeText)}
      </span>
      <span class="badge badge-dim">Status: ${escapeHtml(report.status)}</span>
    </div>
    <div class="meta-row">
      <span>Confirmed findings: ${report.summary.confirmedFindings}</span>
      <span>•</span>
      <span>Heuristic signals: ${report.summary.heuristicWarnings}</span>
      <span>•</span>
      <span>Scanned at: ${escapeHtml(report.scannedAt)}</span>
    </div>
  </div>

  <div class="section-title">Security Findings</div>
  ${findingsHtml}

  <div class="section-title">Coverage &amp; Checks (${checks.length})</div>
  <div class="card">
    ${checksHtml}
  </div>

  <div class="section-title">Sources Consulted</div>
  <div class="sources-row">
    ${sourcesHtml}
  </div>
</body>
</html>`;
  }
}
