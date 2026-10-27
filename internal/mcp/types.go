// Package mcp 提供用于 MCP 协议通信的结构化数据定义。
// 所有导出的字段名与类型均保持稳定，供 AI 编码智能体解析。
package mcp

// PackageIdentityOutput 描述包所属生态与名称。
type PackageIdentityOutput struct {
	Ecosystem string `json:"ecosystem" jsonschema:"Package ecosystem ('npm' or 'pypi')."`
	Name      string `json:"name" jsonschema:"Canonical package name."`
}

// PackageTargetOutput 描述单包扫描的目标与解析版本。
type PackageTargetOutput struct {
	Package         PackageIdentityOutput `json:"package" jsonschema:"Target package identity."`
	ResolvedVersion *string               `json:"resolved_version,omitempty" jsonschema:"Resolved exact package version."`
}

// ScanSummaryOutput 汇总发现计数与严重等级统计。
type ScanSummaryOutput struct {
	ConfirmedFindings int            `json:"confirmed_findings" jsonschema:"Number of confirmed vulnerability findings."`
	HeuristicWarnings int            `json:"heuristic_warnings" jsonschema:"Number of heuristic risk warnings."`
	HighestSeverity   *string        `json:"highest_severity,omitempty" jsonschema:"Highest severity detected (critical, high, medium, low, unknown)."`
	SeverityCounts    map[string]int `json:"severity_counts" jsonschema:"Count of findings per severity level."`
}

// FormatInfoOutput 描述检测到的项目清单及锁文件格式覆盖状态。
type FormatInfoOutput struct {
	Ecosystem          string `json:"ecosystem" jsonschema:"Package ecosystem for this manifest format."`
	Format             string `json:"format" jsonschema:"Detected project manifest and lockfile format identifier."`
	ManifestPath       string `json:"manifest_path" jsonschema:"Path to the dependency manifest file."`
	LockfilePath       string `json:"lockfile_path,omitempty" jsonschema:"Path to the associated lockfile, if present."`
	ResolvedPackages   int    `json:"resolved_packages" jsonschema:"Number of resolved exact package versions."`
	UnresolvedPackages int    `json:"unresolved_packages" jsonschema:"Number of declared dependencies missing exact pinned versions."`
	Resolution         string `json:"resolution" jsonschema:"Dependency resolution completeness ('complete', 'partial', 'unknown')."`
}

// FindingBriefOutput 包含发现或警告的简明摘要。
type FindingBriefOutput struct {
	ID            string                `json:"id" jsonschema:"Unique finding identifier."`
	Kind          string                `json:"kind" jsonschema:"Finding category ('confirmed_vulnerability' or 'heuristic_warning')."`
	Title         string                `json:"title" jsonschema:"Title or headline of the security finding."`
	Severity      string                `json:"severity" jsonschema:"Severity level ('critical', 'high', 'medium', 'low', 'unknown')."`
	Package       PackageIdentityOutput `json:"package" jsonschema:"Affected package identity."`
	Version       *string               `json:"version,omitempty" jsonschema:"Affected package version, if available."`
	Relationship  string                `json:"relationship,omitempty" jsonschema:"Dependency relationship ('direct' or 'transitive')."`
	Scopes        []string              `json:"scopes,omitempty" jsonschema:"Dependency scopes ('production', 'development', 'optional', 'unknown')."`
	Resolution    string                `json:"resolution,omitempty" jsonschema:"Resolution state ('complete', 'partial', 'unknown')."`
	RiskScore     *float64              `json:"risk_score,omitempty" jsonschema:"Calculated risk score between 0.0 and 10.0."`
	RiskRationale string                `json:"risk_rationale,omitempty" jsonschema:"Explanation of factors contributing to the risk score."`
}

// CheckResultOutput 记录扫描执行单项检查的结果。
type CheckResultOutput struct {
	ID        string   `json:"id" jsonschema:"Unique identifier of the check."`
	Name      string   `json:"name" jsonschema:"Human-readable check name."`
	Status    string   `json:"status" jsonschema:"Check execution outcome ('succeeded', 'failed', 'unavailable', 'skipped')."`
	Message   string   `json:"message" jsonschema:"Detailed explanation of check outcome."`
	SourceIDs []string `json:"source_ids" jsonschema:"IDs of sources that provided data for this check."`
}

// LimitationOutput 记录扫描过程中的限制或未覆盖范围。
type LimitationOutput struct {
	Code    string `json:"code" jsonschema:"Stable machine-readable limitation code."`
	Message string `json:"message" jsonschema:"Explanation of the limitation or skipped scope."`
}

// SourceOutput 记录扫描使用的数据源信息。
type SourceOutput struct {
	ID          string  `json:"id" jsonschema:"Unique source reference identifier."`
	Provider    string  `json:"provider" jsonschema:"Source provider identifier ('osv', 'npm', 'pypi', 'demo', etc.)."`
	Identifier  string  `json:"identifier" jsonschema:"Provider-specific reference identifier."`
	URL         *string `json:"url,omitempty" jsonschema:"Upstream URL for advisory or package documentation."`
	RetrievedAt string  `json:"retrieved_at" jsonschema:"RFC3339 timestamp when source data was retrieved."`
}

// FindingEvidenceOutput 描述支持某项发现的具体证据。
type FindingEvidenceOutput struct {
	Kind      string `json:"kind" jsonschema:"Evidence kind ('advisory', 'dependency_path', 'risk_factor', etc.)."`
	Summary   string `json:"summary" jsonschema:"Textual description of the evidence item."`
	SourceID  string `json:"source_id" jsonschema:"Source reference providing this evidence."`
	Reference string `json:"reference,omitempty" jsonschema:"Upstream identifier or external advisory reference key."`
}

// RiskAssessmentOutput 包含发现的风险评分与推导原因。
type RiskAssessmentOutput struct {
	Score     float64 `json:"score" jsonschema:"Risk score between 0.0 and 10.0."`
	Rationale string  `json:"rationale" jsonschema:"Detailed reasoning for the assigned score."`
}

// DependencyEvidenceItemOutput 记录依赖声明或锁文件中的字段证据。
type DependencyEvidenceItemOutput struct {
	Path  string `json:"path" jsonschema:"File path containing the dependency declaration."`
	Field string `json:"field,omitempty" jsonschema:"Manifest or lockfile field where dependency was located."`
}

// DependencyContextOutput 记录发现在项目依赖图中的进入途径与分类。
type DependencyContextOutput struct {
	Relationship        string                         `json:"relationship" jsonschema:"Dependency relationship ('direct' or 'transitive')."`
	Scopes              []string                       `json:"scopes" jsonschema:"Dependency scopes ('production', 'development', 'optional', 'unknown')."`
	Resolution          string                         `json:"resolution" jsonschema:"Resolution state ('complete', 'partial', 'unknown')."`
	Evidence            []DependencyEvidenceItemOutput `json:"evidence" jsonschema:"Manifest/lockfile evidence proving dependency classification."`
	RepresentativePaths [][]string                     `json:"representative_paths,omitempty" jsonschema:"Representative dependency chains leading from project root to affected package."`
}

// ScanProjectInput 为 scan_project 工具的输入参数。
type ScanProjectInput struct {
	Path string `json:"path" jsonschema:"Project directory path to scan. Must reside within permitted project directories."`
}

// ScanProjectOutput 为 scan_project 工具的输出结果。
type ScanProjectOutput struct {
	ScanID      string               `json:"scan_id" jsonschema:"Unique identifier assigned to this project scan session."`
	Root        string               `json:"root" jsonschema:"Canonical project root path that was scanned."`
	ScannedAt   string               `json:"scanned_at" jsonschema:"RFC3339 timestamp when the scan was executed."`
	Status      string               `json:"status" jsonschema:"Overall scan status (clean, vulnerable, warning, incomplete, failed)."`
	Message     string               `json:"message" jsonschema:"Human-readable outcome message explaining the scan result."`
	Summary     ScanSummaryOutput    `json:"summary" jsonschema:"Structured scan totals and severity breakdown."`
	Formats     []FormatInfoOutput   `json:"formats" jsonschema:"Detected project manifest and lockfile formats."`
	Findings    []FindingBriefOutput `json:"findings" jsonschema:"List of security findings and warnings discovered in project dependencies."`
	Checks      []CheckResultOutput  `json:"checks" jsonschema:"Outcome of individual checks executed during scanning."`
	Limitations []LimitationOutput   `json:"limitations" jsonschema:"Limitations and coverage caveats for this scan."`
	Sources     []SourceOutput       `json:"sources" jsonschema:"Data sources and registries consulted for this scan."`
}

// ScanPackageInput 为 scan_package 工具的输入参数。
type ScanPackageInput struct {
	Package   string `json:"package" jsonschema:"Package name or specifier to scan (e.g. 'lodash', 'npm:lodash@4.17.21', or 'pypi:requests==2.31.0')."`
	Version   string `json:"version,omitempty" jsonschema:"Optional specific package version. If empty, the latest version is resolved."`
	Ecosystem string `json:"ecosystem,omitempty" jsonschema:"Optional package ecosystem ('npm' or 'pypi'). Defaults to 'npm' if unspecified."`
}

// ScanPackageOutput 为 scan_package 工具的输出结果。
type ScanPackageOutput struct {
	ScanID      string               `json:"scan_id" jsonschema:"Unique identifier assigned to this package scan session."`
	Target      PackageTargetOutput  `json:"target" jsonschema:"The package and resolved version that was scanned."`
	ScannedAt   string               `json:"scanned_at" jsonschema:"RFC3339 timestamp when the scan was executed."`
	Status      string               `json:"status" jsonschema:"Overall scan status (clean, vulnerable, warning, incomplete, failed)."`
	DataMode    string               `json:"data_mode" jsonschema:"Data mode used for this scan ('live' or 'demo')."`
	Message     string               `json:"message" jsonschema:"Human-readable outcome message explaining the scan result."`
	Summary     ScanSummaryOutput    `json:"summary" jsonschema:"Structured scan totals and severity breakdown."`
	Findings    []FindingBriefOutput `json:"findings" jsonschema:"List of security findings and warnings discovered for the package."`
	Checks      []CheckResultOutput  `json:"checks" jsonschema:"Outcome of individual checks executed during scanning."`
	Limitations []LimitationOutput   `json:"limitations" jsonschema:"Limitations and coverage caveats for this scan."`
	Sources     []SourceOutput       `json:"sources" jsonschema:"Data sources and registries consulted for this scan."`
}

// GetFindingDetailsInput 为 get_finding_details 工具的输入参数。
type GetFindingDetailsInput struct {
	FindingID string `json:"finding_id" jsonschema:"Unique identifier of the finding (from scan_project or scan_package)."`
	ScanID    string `json:"scan_id,omitempty" jsonschema:"Optional scan session ID where the finding was observed. If omitted, recent scans will be searched."`
}

// GetFindingDetailsOutput 为 get_finding_details 工具的输出结果。
type GetFindingDetailsOutput struct {
	FindingID         string                   `json:"finding_id" jsonschema:"Unique finding identifier."`
	ScanID            string                   `json:"scan_id" jsonschema:"Scan session ID containing this finding."`
	Kind              string                   `json:"kind" jsonschema:"Finding kind (confirmed_vulnerability or heuristic_warning)."`
	Title             string                   `json:"title" jsonschema:"Title or headline of the finding."`
	Description       string                   `json:"description" jsonschema:"Detailed finding description."`
	Severity          string                   `json:"severity" jsonschema:"Assigned severity level (critical, high, medium, low, unknown)."`
	Package           PackageIdentityOutput    `json:"package" jsonschema:"Package identity affected by this finding."`
	Version           *string                  `json:"version,omitempty" jsonschema:"Specific package version affected."`
	Risk              *RiskAssessmentOutput    `json:"risk,omitempty" jsonschema:"Risk assessment score and rationale."`
	Remediation       *string                  `json:"remediation,omitempty" jsonschema:"Suggested remediation or upgrade guidance if available."`
	Evidence          []FindingEvidenceOutput  `json:"evidence" jsonschema:"Structured evidence supporting the finding."`
	Sources           []SourceOutput           `json:"sources" jsonschema:"Source references corresponding to the finding's evidence."`
	DependencyContext *DependencyContextOutput `json:"dependency_context,omitempty" jsonschema:"Project dependency relationship, scope, and resolution context."`
	Message           string                   `json:"message" jsonschema:"Descriptive notes regarding evidence completeness and security implications."`
}

// SummaryTotals 记录汇总计数。
type SummaryTotals struct {
	TotalFindings           int            `json:"total_findings" jsonschema:"Total count of confirmed findings and heuristic warnings."`
	ConfirmedFindings       int            `json:"confirmed_findings" jsonschema:"Count of confirmed vulnerability findings."`
	HeuristicWarnings       int            `json:"heuristic_warnings" jsonschema:"Count of heuristic risk warnings."`
	HighestSeverity         *string        `json:"highest_severity,omitempty" jsonschema:"Highest severity observed across all findings."`
	SeverityCounts          map[string]int `json:"severity_counts" jsonschema:"Count of findings categorized by severity."`
	DependenciesCount       int            `json:"dependencies_count" jsonschema:"Total dependency nodes discovered."`
	ResolvedPackagesCount   int            `json:"resolved_packages_count" jsonschema:"Number of exact package versions resolved."`
	UnresolvedPackagesCount int            `json:"unresolved_packages_count" jsonschema:"Number of dependencies missing exact resolved versions."`
}

// GetScanSummaryInput 为 get_scan_summary 工具的输入参数。
type GetScanSummaryInput struct {
	ScanID    string `json:"scan_id,omitempty" jsonschema:"Optional scan ID returned by a previous scan_project or scan_package call."`
	Path      string `json:"path,omitempty" jsonschema:"Optional project directory path to scan and summarize if scan_id is omitted."`
	Package   string `json:"package,omitempty" jsonschema:"Optional package specifier to scan and summarize if scan_id is omitted."`
	Version   string `json:"version,omitempty" jsonschema:"Optional package version when package is specified."`
	Ecosystem string `json:"ecosystem,omitempty" jsonschema:"Optional ecosystem ('npm' or 'pypi') when package is specified."`
}

// GetScanSummaryOutput 为 get_scan_summary 工具的输出结果。
type GetScanSummaryOutput struct {
	ScanID                    string        `json:"scan_id" jsonschema:"Scan session ID for this summary."`
	TargetType                string        `json:"target_type" jsonschema:"Type of scan summarized ('project' or 'package')."`
	TargetName                string        `json:"target_name" jsonschema:"Target name or path that was summarized."`
	Status                    string        `json:"status" jsonschema:"Overall scan status (clean, vulnerable, warning, incomplete, failed)."`
	ScannedAt                 string        `json:"scanned_at" jsonschema:"RFC3339 timestamp when the scan was executed."`
	Totals                    SummaryTotals `json:"totals" jsonschema:"Detailed counts for findings and severities."`
	Resolution                string        `json:"resolution" jsonschema:"Dependency resolution completeness state (complete, partial, unknown, or none)."`
	LimitationsCount          int           `json:"limitations_count" jsonschema:"Number of limitations recorded during scanning."`
	HasUnresolvedDependencies bool          `json:"has_unresolved_dependencies" jsonschema:"True if dependencies could not be completely resolved."`
	Message                   string        `json:"message" jsonschema:"Summary description explaining the security and resolution posture."`
}
