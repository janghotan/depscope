import type {
  CheckResult,
  CheckStatus,
  DependencyNode,
  Finding,
  ScanReport,
  ScanStatus,
  Severity,
  SourceReference,
} from "@/generated/report-types";

const SEVERITY_RANK: Record<Severity, number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
  unknown: 4,
};

const KIND_RANK = {
  confirmed: 0,
  heuristic: 1,
} as const;

export const CONDITION_COPY: Record<
  ScanStatus,
  { label: string; description: string }
> = {
  clean: {
    label: "Clean",
    description:
      "Every recorded check succeeded and no findings were returned. Absence of findings is not a guarantee of safety.",
  },
  findings: {
    label: "Findings",
    description:
      "At least one advisory match or heuristic signal was recorded. Read the evidence and the checks before acting.",
  },
  incomplete: {
    label: "Incomplete",
    description:
      "One or more checks did not complete. Missing coverage must not be read as safety.",
  },
  failed: {
    label: "Failed",
    description:
      "The scan did not complete. The record below explains what failed; it is not a clean result.",
  },
  cancelled: {
    label: "Cancelled",
    description:
      "The scan was cancelled before a condition could be established.",
  },
};

export const CHECK_STATUS_LABEL: Record<CheckStatus, string> = {
  succeeded: "Succeeded",
  unavailable: "Unavailable",
  skipped: "Skipped",
  failed: "Failed",
};

export const SEVERITY_LABEL: Record<Severity, string> = {
  critical: "Critical",
  high: "High",
  medium: "Medium",
  low: "Low",
  unknown: "Unknown",
};

/** 报告已按契约校验，这里只做稳定的 UTC 展示格式。 */
export function formatTimestamp(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(
    date.getUTCDate(),
  )} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())} UTC`;
}

export function targetLabel(report: ScanReport): string {
  const version = report.target.resolvedVersion;
  const identity = `${report.target.package.ecosystem} · ${report.target.package.name}`;
  return version ? `${identity} @ ${version}` : `${identity} @ unresolved`;
}

export function findingLocation(finding: Finding): string {
  const base = `${finding.package.ecosystem} · ${finding.package.name}`;
  return finding.version ? `${base} @ ${finding.version}` : base;
}

/** 仅调整展示顺序：已确认发现优先，其次按严重度；不重新计算风险。 */
export function sortFindings(findings: readonly Finding[]): Finding[] {
  return [...findings].sort((left, right) => {
    const kindDelta = KIND_RANK[left.kind] - KIND_RANK[right.kind];
    if (kindDelta !== 0) {
      return kindDelta;
    }
    const severityDelta =
      SEVERITY_RANK[left.severity] - SEVERITY_RANK[right.severity];
    if (severityDelta !== 0) {
      return severityDelta;
    }
    return left.title.localeCompare(right.title);
  });
}

export function checkTally(checks: readonly CheckResult[]) {
  const tally = {
    total: checks.length,
    succeeded: 0,
    unavailable: 0,
    skipped: 0,
    failed: 0,
  };
  for (const check of checks) {
    tally[check.status] += 1;
  }
  return tally;
}

export function findingsForNode(
  node: DependencyNode,
  findings: readonly Finding[],
): Finding[] {
  return findings.filter((finding) => {
    if (finding.package.ecosystem !== node.package.ecosystem) {
      return false;
    }
    if (finding.package.name !== node.package.name) {
      return false;
    }
    return finding.version === null || finding.version === node.version;
  });
}

export function providerLabel(source: SourceReference): string {
  switch (source.provider) {
    case "osv":
      return "OSV.dev";
    case "npm":
      return "npm registry";
    case "pypi":
      return "PyPI";
    case "demo":
      return "Demo fixture";
  }
}

export function plural(count: number, singular: string, pluralForm: string) {
  return `${count} ${count === 1 ? singular : pluralForm}`;
}

/** 生成可复制、无 HTML 的纯文本记录，顺序与页面阅读顺序一致。 */
export function plainTextReport(report: ScanReport): string {
  const lines: string[] = [];
  lines.push("DEPSCOPE CONDITION RECORD");
  lines.push(`Target: ${targetLabel(report)}`);
  lines.push(`Schema: ${report.schemaVersion}`);
  lines.push(`Data mode: ${report.dataMode.toUpperCase()}`);
  if (report.demoNotice) {
    lines.push(`Demo notice: ${report.demoNotice}`);
  }
  lines.push(`Condition: ${CONDITION_COPY[report.status].label.toUpperCase()}`);
  lines.push(`Scanned at: ${formatTimestamp(report.scannedAt)}`);
  lines.push(
    `Summary: ${report.summary.confirmedFindings} confirmed, ${report.summary.heuristicWarnings} heuristic, highest severity ${
      report.summary.highestSeverity ?? "none"
    }`,
  );

  lines.push("");
  lines.push("FINDINGS");
  if (report.findings.length === 0) {
    lines.push("None recorded.");
  } else {
    for (const finding of sortFindings(report.findings)) {
      lines.push(
        `- [${finding.severity.toUpperCase()} / ${finding.kind.toUpperCase()}] ${finding.title}`,
      );
      lines.push(`  Package: ${findingLocation(finding)}`);
      lines.push(`  ${finding.description}`);
      for (const evidence of finding.evidence) {
        lines.push(
          `  Evidence (${evidence.kind}): ${evidence.summary}${
            evidence.reference ? ` [${evidence.reference}]` : ""
          }`,
        );
      }
      if (finding.risk) {
        lines.push(
          `  Risk: ${finding.risk.score}/100 — ${finding.risk.rationale}`,
        );
      }
      if (finding.remediation) {
        lines.push(`  Remediation: ${finding.remediation}`);
      }
    }
  }

  lines.push("");
  lines.push("CHECKS");
  if (report.checks.length === 0) {
    lines.push("No checks were recorded.");
  } else {
    for (const check of report.checks) {
      lines.push(
        `- ${check.name}: ${CHECK_STATUS_LABEL[check.status]} — ${check.message}`,
      );
    }
  }

  lines.push("");
  lines.push("LIMITATIONS");
  if (report.limitations.length === 0) {
    lines.push("None recorded.");
  } else {
    for (const limitation of report.limitations) {
      lines.push(`- ${limitation.code}: ${limitation.message}`);
    }
  }

  lines.push("");
  lines.push("SOURCES");
  if (report.sources.length === 0) {
    lines.push("No sources were recorded.");
  } else {
    for (const source of report.sources) {
      lines.push(
        `- ${providerLabel(source)} / ${source.identifier} (retrieved ${formatTimestamp(
          source.retrievedAt,
        )})`,
      );
    }
  }

  return lines.join("\n");
}
