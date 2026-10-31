import * as vscode from "vscode";
import { Finding, ScanReport, Severity } from "./generated/report-types";
import { isReportClean, isReportIncomplete } from "./statusUtils";

export { isReportClean, isReportIncomplete };

/**
 * 树视图节点分类类型：顶级分组或具体包叶子节点。
 */
export type TreeItemNode = GroupNode | PackageNode;

/**
 * 顶级分组节点。
 */
export class GroupNode {
  readonly kind = "group";
  constructor(
    public readonly category: "confirmed" | "heuristic" | "incomplete" | "clean",
    public readonly label: string,
    public readonly reports: ScanReport[]
  ) {}
}

/**
 * 单个软件包叶子节点。
 */
export class PackageNode {
  readonly kind = "package";
  constructor(public readonly report: ScanReport) {}
}

/**
 * 根据严重级别及覆盖完整性获取匹配 DepScope 终端配色语义的 VS Code 图标与颜色：
 * - Critical / High: 红色危险色 (ansiDanger / #ff7b72) -> errorForeground
 * - Medium / Incomplete: 琥珀黄色警告色 (ansiMedium / #e3b341) -> editorWarning.foreground
 * - Low / Unknown: 浅蓝色信息色 (ansiLow / #79c0ff) -> editorInfo.foreground
 * - Clean: 翡翠绿色通过色 (ansiSuccess / #7ee787) -> testing.iconPassed
 */
export function getSeverityIcon(severity: Severity | null | undefined, isIncomplete = false): vscode.ThemeIcon {
  switch (severity) {
    case "critical":
    case "high":
      return new vscode.ThemeIcon("error", new vscode.ThemeColor("errorForeground"));
    case "medium":
      return new vscode.ThemeIcon("warning", new vscode.ThemeColor("editorWarning.foreground"));
    case "low":
    case "unknown":
      return new vscode.ThemeIcon("info", new vscode.ThemeColor("editorInfo.foreground"));
    default:
      if (isIncomplete) {
        return new vscode.ThemeIcon("warning", new vscode.ThemeColor("editorWarning.foreground"));
      }
      return new vscode.ThemeIcon("pass", new vscode.ThemeColor("testing.iconPassed"));
  }
}

/**
 * ScanTreeProvider 实现 vscode.TreeDataProvider 接口，
 * 管理工作区依赖扫描结果的四级分组侧边栏：
 * 1. "Confirmed vulnerabilities"（OSV 匹配确认的高危/漏洞）
 * 2. "Heuristic signals"（生命周期脚本/仿冒抢注等启发式警告）
 * 3. "Incomplete coverage"（检查失败或不完整覆盖，绝不混入 Clean）
 * 4. "Clean"（全部检查成功且无任何发现的安全依赖）
 */
export class ScanTreeProvider implements vscode.TreeDataProvider<TreeItemNode> {
  private _onDidChangeTreeData: vscode.EventEmitter<TreeItemNode | undefined | null | void> =
    new vscode.EventEmitter<TreeItemNode | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<TreeItemNode | undefined | null | void> =
    this._onDidChangeTreeData.event;

  private confirmedReports: ScanReport[] = [];
  private heuristicReports: ScanReport[] = [];
  private incompleteReports: ScanReport[] = [];
  private cleanReports: ScanReport[] = [];
  private hasScanned = false;

  /**
   * 刷新树结构并触发视图更新。
   */
  public refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  /**
   * 载入全部包的扫描报告，并严格按风险与覆盖性质进行四分类。
   */
  public setReports(reports: ScanReport[]): void {
    this.hasScanned = true;
    this.confirmedReports = [];
    this.heuristicReports = [];
    this.incompleteReports = [];
    this.cleanReports = [];

    for (const report of reports) {
      const findings = report.findings || [];
      const hasConfirmed = findings.some((f) => f.kind === "confirmed");
      const hasHeuristic = findings.some((f) => f.kind === "heuristic");
      const incomplete = isReportIncomplete(report);

      if (hasConfirmed) {
        this.confirmedReports.push(report);
      } else if (hasHeuristic) {
        this.heuristicReports.push(report);
      } else if (incomplete) {
        this.incompleteReports.push(report);
      } else if (isReportClean(report)) {
        this.cleanReports.push(report);
      } else {
        this.incompleteReports.push(report);
      }
    }

    // 组内排序：按严重程度降序排列，同严重程度按包名升序
    const severityWeight = (sev: Severity | null | undefined): number => {
      switch (sev) {
        case "critical":
          return 5;
        case "high":
          return 4;
        case "medium":
          return 3;
        case "low":
          return 2;
        case "unknown":
          return 1;
        default:
          return 0;
      }
    };

    const sortFn = (a: ScanReport, b: ScanReport) => {
      const weightA = severityWeight(a.summary.highestSeverity);
      const weightB = severityWeight(b.summary.highestSeverity);
      if (weightA !== weightB) {
        return weightB - weightA;
      }
      return a.target.package.name.localeCompare(b.target.package.name);
    };

    this.confirmedReports.sort(sortFn);
    this.heuristicReports.sort(sortFn);
    this.incompleteReports.sort((a, b) => a.target.package.name.localeCompare(b.target.package.name));
    this.cleanReports.sort((a, b) => a.target.package.name.localeCompare(b.target.package.name));

    this.refresh();
  }

  /**
   * 清空所有扫描数据。
   */
  public clear(): void {
    this.hasScanned = false;
    this.confirmedReports = [];
    this.heuristicReports = [];
    this.incompleteReports = [];
    this.cleanReports = [];
    this.refresh();
  }

  getTreeItem(element: TreeItemNode): vscode.TreeItem {
    if (element.kind === "group") {
      const item = new vscode.TreeItem(
        element.label,
        element.reports.length > 0
          ? vscode.TreeItemCollapsibleState.Expanded
          : vscode.TreeItemCollapsibleState.Collapsed
      );
      item.description = `${element.reports.length}`;
      item.contextValue = "depscope.group";

      switch (element.category) {
        case "confirmed":
          item.iconPath = new vscode.ThemeIcon("bug", new vscode.ThemeColor("errorForeground"));
          item.tooltip = "OSV-matched confirmed vulnerabilities";
          break;
        case "heuristic":
          item.iconPath = new vscode.ThemeIcon(
            "warning",
            new vscode.ThemeColor("editorWarning.foreground")
          );
          item.tooltip = "Heuristic signals (lifecycle scripts, typosquatting)";
          break;
        case "incomplete":
          item.iconPath = new vscode.ThemeIcon(
            "alert",
            new vscode.ThemeColor("editorWarning.foreground")
          );
          item.tooltip = "Incomplete coverage: checks failed or missing; not a clean result";
          break;
        case "clean":
          item.iconPath = new vscode.ThemeIcon("pass", new vscode.ThemeColor("testing.iconPassed"));
          item.tooltip = "Packages with verified successful checks and no findings";
          break;
      }
      return item;
    }

    // 叶子节点展示包名与版本
    const report = element.report;
    const pkgName = report.target.package.name;
    const version = report.target.resolvedVersion || "latest";
    const incomplete = isReportIncomplete(report);

    const item = new vscode.TreeItem(pkgName, vscode.TreeItemCollapsibleState.None);

    let desc = version;
    if (report.summary.highestSeverity) {
      desc += ` [${report.summary.highestSeverity.toUpperCase()}]`;
    } else if (incomplete) {
      desc += ` [${report.status.toUpperCase()}]`;
    }
    item.description = desc;

    item.iconPath = getSeverityIcon(report.summary.highestSeverity, incomplete);
    item.contextValue = "depscope.package";

    // 格式化提示气泡
    const findingTitles = (report.findings || [])
      .map((f: Finding) => `• [${f.severity.toUpperCase()}] ${f.id}: ${f.title}`)
      .join("\n");

    const failedChecks = (report.checks || [])
      .filter((c) => c.status !== "succeeded")
      .map((c) => `• [${c.status.toUpperCase()}] ${c.name}: ${c.message}`)
      .join("\n");

    let tooltipContent =
      `**${report.target.package.ecosystem}:${pkgName}@${version}**\n\n` +
      `Status: ${report.status}\n`;

    if (findingTitles) {
      tooltipContent += `\nFindings:\n${findingTitles}\n`;
    }
    if (failedChecks) {
      tooltipContent += `\nCoverage gaps / Failed checks:\n${failedChecks}\n`;
    }
    if (!findingTitles && !failedChecks) {
      tooltipContent += `\nAll coverage checks passed. No known vulnerabilities detected.`;
    }

    item.tooltip = new vscode.MarkdownString(tooltipContent);

    // 点击打开 Webview 详情面板
    item.command = {
      command: "depscope.showReport",
      title: "Show Finding Report",
      arguments: [report],
    };

    return item;
  }

  getChildren(element?: TreeItemNode): vscode.ProviderResult<TreeItemNode[]> {
    if (!element) {
      if (!this.hasScanned) {
        return [];
      }
      return [
        new GroupNode("confirmed", "Confirmed vulnerabilities", this.confirmedReports),
        new GroupNode("heuristic", "Heuristic signals", this.heuristicReports),
        new GroupNode("incomplete", "Incomplete coverage", this.incompleteReports),
        new GroupNode("clean", "Clean", this.cleanReports),
      ];
    }

    if (element.kind === "group") {
      return element.reports.map((report) => new PackageNode(report));
    }

    return [];
  }
}
