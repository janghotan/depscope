package model

// Ecosystem 表示受支持的包生态系统。
type Ecosystem string

const (
	// EcosystemNPM 表示 npm 生态系统。
	EcosystemNPM Ecosystem = "npm"
	// EcosystemPyPI 表示 PyPI 生态系统。
	EcosystemPyPI Ecosystem = "pypi"
)

// IsValid 报告生态系统标识是否受支持。
func (e Ecosystem) IsValid() bool {
	return e == EcosystemNPM || e == EcosystemPyPI
}

// DataMode 表示报告数据来自真实扫描还是确定性演示数据。
type DataMode string

const (
	// DataModeLive 表示数据来自真实来源。
	DataModeLive DataMode = "live"
	// DataModeDemo 表示数据是明确标记的合成演示数据。
	DataModeDemo DataMode = "demo"
)

// IsValid 报告数据模式是否有效。
func (m DataMode) IsValid() bool {
	return m == DataModeLive || m == DataModeDemo
}

// ScanStatus 表示扫描生命周期或最终结果状态。
type ScanStatus string

const (
	// StatusPending 表示扫描尚未开始。
	StatusPending ScanStatus = "pending"
	// StatusRunning 表示扫描正在运行。
	StatusRunning ScanStatus = "running"
	// StatusClean 表示所有检查成功且没有发现问题。
	StatusClean ScanStatus = "clean"
	// StatusFindings 表示所有检查成功且至少存在一条发现或警告。
	StatusFindings ScanStatus = "findings"
	// StatusIncomplete 表示至少一项检查未成功完成，结果不能视为干净。
	StatusIncomplete ScanStatus = "incomplete"
	// StatusFailed 表示扫描整体失败。
	StatusFailed ScanStatus = "failed"
	// StatusCancelled 表示扫描被取消。
	StatusCancelled ScanStatus = "cancelled"
)

// IsValid 报告扫描状态是否有效。
func (s ScanStatus) IsValid() bool {
	switch s {
	case StatusPending, StatusRunning, StatusClean, StatusFindings, StatusIncomplete, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

// IsFinal 报告扫描状态是否为最终状态。
func (s ScanStatus) IsFinal() bool {
	switch s {
	case StatusClean, StatusFindings, StatusIncomplete, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

// Severity 表示标准化后的严重性等级。
type Severity string

const (
	// SeverityCritical 表示最高严重性。
	SeverityCritical Severity = "critical"
	// SeverityHigh 表示高严重性。
	SeverityHigh Severity = "high"
	// SeverityMedium 表示中严重性。
	SeverityMedium Severity = "medium"
	// SeverityLow 表示低严重性。
	SeverityLow Severity = "low"
	// SeverityUnknown 表示严重性未知；未知不能等同于无风险。
	SeverityUnknown Severity = "unknown"
)

// IsValid 报告严重性等级是否有效。
func (s Severity) IsValid() bool {
	switch s {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityUnknown:
		return true
	default:
		return false
	}
}

// Rank 返回用于比较的严重性权重；未知等级权重最低但不代表安全。
func (s Severity) Rank() int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}

// FindingKind 表示发现的可信程度。
type FindingKind string

const (
	// FindingConfirmed 表示有明确证据支持的确认发现。
	FindingConfirmed FindingKind = "confirmed"
	// FindingHeuristic 表示基于启发式规则的警告。
	FindingHeuristic FindingKind = "heuristic"
)

// IsValid 报告发现类型是否有效。
func (k FindingKind) IsValid() bool {
	return k == FindingConfirmed || k == FindingHeuristic
}

// CheckStatus 表示单项检查的结果状态。
type CheckStatus string

const (
	// CheckSucceeded 表示检查完整成功。
	CheckSucceeded CheckStatus = "succeeded"
	// CheckUnavailable 表示检查依赖的来源不可用。
	CheckUnavailable CheckStatus = "unavailable"
	// CheckSkipped 表示检查被有意跳过。
	CheckSkipped CheckStatus = "skipped"
	// CheckFailed 表示检查已执行但失败。
	CheckFailed CheckStatus = "failed"
)

// IsValid 报告检查状态是否有效。
func (s CheckStatus) IsValid() bool {
	switch s {
	case CheckSucceeded, CheckUnavailable, CheckSkipped, CheckFailed:
		return true
	default:
		return false
	}
}

// EvidenceKind 表示证据的类型。
type EvidenceKind string

const (
	// EvidenceAdvisory 表示来自漏洞公告的证据。
	EvidenceAdvisory EvidenceKind = "advisory"
	// EvidenceHeuristic 表示来自启发式规则的证据。
	EvidenceHeuristic EvidenceKind = "heuristic"
	// EvidenceRegistry 表示来自注册表元数据的证据。
	EvidenceRegistry EvidenceKind = "registry"
	// EvidenceDependencyPath 表示来自依赖路径的证据。
	EvidenceDependencyPath EvidenceKind = "dependency-path"
)

// IsValid 报告证据类型是否有效。
func (k EvidenceKind) IsValid() bool {
	switch k {
	case EvidenceAdvisory, EvidenceHeuristic, EvidenceRegistry, EvidenceDependencyPath:
		return true
	default:
		return false
	}
}

// DependencyRelationship 表示依赖节点与扫描目标的关系。
type DependencyRelationship string

const (
	// DependencyRoot 表示扫描目标本身。
	DependencyRoot DependencyRelationship = "root"
	// DependencyDirect 表示直接依赖。
	DependencyDirect DependencyRelationship = "direct"
	// DependencyTransitive 表示传递依赖。
	DependencyTransitive DependencyRelationship = "transitive"
)

// IsValid 报告依赖关系是否有效。
func (r DependencyRelationship) IsValid() bool {
	switch r {
	case DependencyRoot, DependencyDirect, DependencyTransitive:
		return true
	default:
		return false
	}
}

// SourceProvider 表示报告引用的数据来源类型。
type SourceProvider string

const (
	// SourceProviderOSV 表示 OSV.dev。
	SourceProviderOSV SourceProvider = "osv"
	// SourceProviderNPM 表示 npm Registry。
	SourceProviderNPM SourceProvider = "npm"
	// SourceProviderPyPI 表示 PyPI。
	SourceProviderPyPI SourceProvider = "pypi"
	// SourceProviderDemo 表示确定性的合成演示数据。
	SourceProviderDemo SourceProvider = "demo"
)

// IsValid 报告来源类型是否有效。
func (p SourceProvider) IsValid() bool {
	switch p {
	case SourceProviderOSV, SourceProviderNPM, SourceProviderPyPI, SourceProviderDemo:
		return true
	default:
		return false
	}
}

// IsDemo 报告来源是否为演示数据。
func (p SourceProvider) IsDemo() bool {
	return p == SourceProviderDemo
}
