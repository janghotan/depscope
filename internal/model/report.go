package model

import (
	"fmt"
	"strings"
	"time"
)

// SchemaVersion 是扫描报告 JSON 契约的当前版本；字段变更必须同步更新 schema。
const SchemaVersion = "1.0.0"

// ScanReport 是终端界面与 HTTP API 共用的完整扫描报告。
type ScanReport struct {
	SchemaVersion string            `json:"schemaVersion"`
	DataMode      DataMode          `json:"dataMode"`
	DemoNotice    *string           `json:"demoNotice"`
	ScannedAt     time.Time         `json:"scannedAt"`
	Status        ScanStatus        `json:"status"`
	Target        ScanTarget        `json:"target"`
	Summary       Summary           `json:"summary"`
	Findings      []Finding         `json:"findings"`
	Checks        []CheckResult     `json:"checks"`
	Dependencies  DependencyGraph   `json:"dependencies"`
	Sources       []SourceReference `json:"sources"`
	Limitations   []Limitation      `json:"limitations"`
}

// IsClean 仅在最终状态明确为 clean 时返回 true。
func (r ScanReport) IsClean() bool {
	return r.Status == StatusClean
}

// ScanTarget 描述本次扫描的目标包及已解析版本。
type ScanTarget struct {
	Package         PackageIdentity `json:"package"`
	ResolvedVersion *string         `json:"resolvedVersion"`
}

// Validate 检查扫描目标是否有效。
func (t ScanTarget) Validate() error {
	if err := t.Package.Validate(); err != nil {
		return fmt.Errorf("target.package: %w", err)
	}
	if t.ResolvedVersion != nil && strings.TrimSpace(*t.ResolvedVersion) == "" {
		return fmt.Errorf("target.resolvedVersion must not be empty when present")
	}
	return nil
}

// Summary 汇总报告中的发现数量与最高严重性。
type Summary struct {
	ConfirmedFindings int       `json:"confirmedFindings"`
	HeuristicWarnings int       `json:"heuristicWarnings"`
	HighestSeverity   *Severity `json:"highestSeverity"`
}

// Finding 表示一条确认发现或启发式警告。
type Finding struct {
	ID          string          `json:"id"`
	Kind        FindingKind     `json:"kind"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Severity    Severity        `json:"severity"`
	Package     PackageIdentity `json:"package"`
	Version     *string         `json:"version"`
	Evidence    []Evidence      `json:"evidence"`
	Risk        *RiskAssessment `json:"risk"`
	Remediation *string         `json:"remediation,omitempty"`
}

// Evidence 表示支持发现或警告的结构化证据。
type Evidence struct {
	Kind      EvidenceKind `json:"kind"`
	Summary   string       `json:"summary"`
	SourceID  string       `json:"sourceId"`
	Reference string       `json:"reference,omitempty"`
}

// RiskAssessment 表示针对单条发现的风险评估结果。
type RiskAssessment struct {
	Score     float64 `json:"score"`
	Rationale string  `json:"rationale"`
}

// CheckResult 表示扫描过程中单项检查的结果。
type CheckResult struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Status    CheckStatus `json:"status"`
	Message   string      `json:"message"`
	SourceIDs []string    `json:"sourceIds"`
}

// DependencyGraph 表示从扫描目标出发的依赖图。
type DependencyGraph struct {
	RootNodeID string           `json:"rootNodeId"`
	Nodes      []DependencyNode `json:"nodes"`
}

// DependencyNode 表示依赖图中的一个已解析或未解析包。
type DependencyNode struct {
	ID             string                 `json:"id"`
	Package        PackageIdentity        `json:"package"`
	Version        *string                `json:"version"`
	Relationship   DependencyRelationship `json:"relationship"`
	DependencyPath []string               `json:"dependencyPath"`
	Dependencies   []string               `json:"dependencies"`
}

// SourceReference 表示报告引用的一项数据来源。
type SourceReference struct {
	ID          string         `json:"id"`
	Provider    SourceProvider `json:"provider"`
	Identifier  string         `json:"identifier"`
	URL         *string        `json:"url,omitempty"`
	RetrievedAt time.Time      `json:"retrievedAt"`
}

// Limitation 表示报告覆盖范围或数据可用性方面的限制。
type Limitation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
