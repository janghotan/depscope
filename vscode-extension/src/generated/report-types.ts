/**
 * 此文件由 schemas/scan-report.v1.schema.json 自动生成。
 * 请勿手动编辑；运行 npm run generate-types 重新生成。
 */

/**
 * Canonical API response for a DepScope scan report. Synthetic fixtures must use dataMode=demo and provider=demo sources.
 */
export type ScanReport = {
  schemaVersion: "1.0.0";
  dataMode: DataMode;
  demoNotice: string | null;
  scannedAt: string;
  status: ScanStatus;
  target: ScanTarget;
  summary: Summary;
  findings: Finding[];
  checks: CheckResult[];
  dependencies: DependencyGraph;
  sources: SourceReference[];
  limitations: Limitation[];
};
export type DataMode = "live" | "demo";
export type ScanStatus = "clean" | "findings" | "incomplete" | "failed" | "cancelled";
export type Ecosystem = "npm" | "pypi";
export type NullableSeverity = null | "critical" | "high" | "medium" | "low" | "unknown";
export type Finding = {
  id: string;
  kind: FindingKind;
  title: string;
  description: string;
  severity: Severity;
  package: PackageIdentity;
  version: string | null;
  /**
   * @minItems 1
   */
  evidence: [Evidence, ...Evidence[]];
  risk: null | Risk;
  remediation?: string;
};
export type FindingKind = "confirmed" | "heuristic";
export type Severity = "critical" | "high" | "medium" | "low" | "unknown";
export type EvidenceKind = "advisory" | "heuristic" | "registry" | "dependency-path";
export type CheckStatus = "succeeded" | "unavailable" | "skipped" | "failed";
export type DependencyRelationship = "root" | "direct" | "transitive";
export type SourceProvider = "osv" | "npm" | "pypi" | "demo";

export interface ScanTarget {
  package: PackageIdentity;
  resolvedVersion: string | null;
}
export interface PackageIdentity {
  ecosystem: Ecosystem;
  name: string;
}
export interface Summary {
  confirmedFindings: number;
  heuristicWarnings: number;
  highestSeverity: NullableSeverity;
}
export interface Evidence {
  kind: EvidenceKind;
  summary: string;
  sourceId: string;
  reference?: string;
}
export interface Risk {
  score: number;
  rationale: string;
}
export interface CheckResult {
  id: string;
  name: string;
  status: CheckStatus;
  message: string;
  sourceIds: string[];
}
export interface DependencyGraph {
  rootNodeId: string;
  /**
   * @minItems 1
   */
  nodes: [DependencyNode, ...DependencyNode[]];
}
export interface DependencyNode {
  id: string;
  package: PackageIdentity;
  version: string | null;
  relationship: DependencyRelationship;
  /**
   * @minItems 1
   */
  dependencyPath: [string, ...string[]];
  dependencies: string[];
}
export interface SourceReference {
  id: string;
  provider: SourceProvider;
  identifier: string;
  url?: string;
  retrievedAt: string;
}
export interface Limitation {
  code: string;
  message: string;
}
