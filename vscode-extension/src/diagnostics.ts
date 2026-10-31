import * as vscode from "vscode";
import { Finding, ScanReport, Severity } from "./generated/report-types";

let diagnosticCollection: vscode.DiagnosticCollection | null = null;

/**
 * 获取或创建 DepScope 专用的诊断集合。
 */
export function getDiagnosticCollection(): vscode.DiagnosticCollection {
  if (!diagnosticCollection) {
    diagnosticCollection = vscode.languages.createDiagnosticCollection("depscope");
  }
  return diagnosticCollection;
}

/**
 * 正则特殊字符转义。
 */
function escapeRegExp(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * 将严重级别映射为 VS Code 诊断严重性：
 * - Critical / High -> Error (严重阻断/红波浪线)
 * - Medium / 启发式警告 -> Warning (黄色警告)
 * - Low / Unknown -> Information (提示信息)
 */
function mapSeverityToDiagnosticSeverity(
  severity: Severity | null | undefined,
  findings: Finding[]
): vscode.DiagnosticSeverity {
  switch (severity) {
    case "critical":
    case "high":
      return vscode.DiagnosticSeverity.Error;
    case "medium":
      return vscode.DiagnosticSeverity.Warning;
    case "low":
    case "unknown":
      return vscode.DiagnosticSeverity.Information;
    default:
      if (findings.length > 0) {
        return vscode.DiagnosticSeverity.Warning;
      }
      return vscode.DiagnosticSeverity.Information;
  }
}

/**
 * 更新工作区清单文件（package.json 与 requirements.txt）中存在风险的依赖行诊断标记。
 *
 * @param reports 所有包的扫描结果集合
 */
export async function updateDiagnostics(reports: ScanReport[]): Promise<void> {
  const collection = getDiagnosticCollection();
  collection.clear();

  // 仅提取存在安全发现的报告
  const vulnerableReports = reports.filter((r) => r.findings && r.findings.length > 0);
  if (vulnerableReports.length === 0) {
    return;
  }

  const npmReports = vulnerableReports.filter((r) => r.target.package.ecosystem === "npm");
  const pypiReports = vulnerableReports.filter((r) => r.target.package.ecosystem === "pypi");

  // 查找工作区内的所有 package.json 与 requirements.txt 文件
  const packageJsonUris = await vscode.workspace.findFiles("**/package.json", "**/node_modules/**");
  const requirementsUris = await vscode.workspace.findFiles("**/requirements*.txt", "**/node_modules/**");

  // 1. 处理 package.json
  for (const uri of packageJsonUris) {
    try {
      const document = await vscode.workspace.openTextDocument(uri);
      const diagnostics: vscode.Diagnostic[] = [];
      const text = document.getText();
      const lines = text.split(/\r?\n/);

      for (const report of npmReports) {
        const pkgName = report.target.package.name;
        // 匹配 "pkg-name": "version"
        const regex = new RegExp(`"${escapeRegExp(pkgName)}"\\s*:\\s*"([^"]+)"`);

        for (let lineIdx = 0; lineIdx < lines.length; lineIdx++) {
          const line = lines[lineIdx];
          const match = regex.exec(line);
          if (match) {
            const startChar = match.index;
            const endChar = match.index + match[0].length;
            const range = new vscode.Range(lineIdx, startChar, lineIdx, endChar);

            const highestSev = report.summary.highestSeverity;
            const diagSev = mapSeverityToDiagnosticSeverity(highestSev, report.findings);
            const findingIds = report.findings.map((f) => f.id).join(", ");
            const primary = report.findings[0];

            const message = `DepScope: ${pkgName}@${report.target.resolvedVersion || match[1]} [${(
              highestSev || "WARNING"
            ).toUpperCase()}] - ${primary.title} (${findingIds})`;

            const diagnostic = new vscode.Diagnostic(range, message, diagSev);
            diagnostic.source = "DepScope";
            diagnostic.code = primary.id;
            diagnostics.push(diagnostic);
          }
        }
      }

      if (diagnostics.length > 0) {
        collection.set(uri, diagnostics);
      }
    } catch {
      // 忽略无法读取或解析的文件
    }
  }

  // 2. 处理 requirements.txt
  for (const uri of requirementsUris) {
    try {
      const document = await vscode.workspace.openTextDocument(uri);
      const diagnostics: vscode.Diagnostic[] = [];
      const text = document.getText();
      const lines = text.split(/\r?\n/);

      for (const report of pypiReports) {
        const pkgName = report.target.package.name;
        // PyPI 规范名大小写不敏感，且下划线与连字符等价
        const normalizedName = escapeRegExp(pkgName).replace(/[-_]/g, "[-_]");
        const regex = new RegExp(`^\\s*(${normalizedName})(?:==|===|>=|<=|~=|>|<|!=)\\s*([^#\\s]+)`, "i");

        for (let lineIdx = 0; lineIdx < lines.length; lineIdx++) {
          const line = lines[lineIdx];
          const match = regex.exec(line);
          if (match) {
            const startChar = line.indexOf(match[1]);
            const endChar = startChar + match[0].trim().length;
            const range = new vscode.Range(lineIdx, startChar, lineIdx, endChar);

            const highestSev = report.summary.highestSeverity;
            const diagSev = mapSeverityToDiagnosticSeverity(highestSev, report.findings);
            const findingIds = report.findings.map((f) => f.id).join(", ");
            const primary = report.findings[0];

            const message = `DepScope: ${pkgName}@${report.target.resolvedVersion || match[2]} [${(
              highestSev || "WARNING"
            ).toUpperCase()}] - ${primary.title} (${findingIds})`;

            const diagnostic = new vscode.Diagnostic(range, message, diagSev);
            diagnostic.source = "DepScope";
            diagnostic.code = primary.id;
            diagnostics.push(diagnostic);
          }
        }
      }

      if (diagnostics.length > 0) {
        collection.set(uri, diagnostics);
      }
    } catch {
      // 忽略无法读取或解析的文件
    }
  }
}

/**
 * 清除所有诊断信息。
 */
export function clearDiagnostics(): void {
  if (diagnosticCollection) {
    diagnosticCollection.clear();
  }
}

/**
 * 释放诊断集合资源。
 */
export function disposeDiagnostics(): void {
  if (diagnosticCollection) {
    diagnosticCollection.dispose();
    diagnosticCollection = null;
  }
}
