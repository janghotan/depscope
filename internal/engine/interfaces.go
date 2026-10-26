// Package engine 定义共享安全引擎的编排接口、外部数据提供者契约与运行配置。
package engine

import (
	"context"
	"fmt"
	"time"

	"depscope/internal/model"
)

// ScanRequest 描述一次扫描请求；本阶段只固定最小字段，完整扫描语义仍待后续阶段确认。
type ScanRequest struct {
	Target   model.ScanTarget `json:"target"`
	DataMode model.DataMode   `json:"dataMode"`
}

// Validate 检查扫描请求是否有效。
func (r ScanRequest) Validate() error {
	if !r.DataMode.IsValid() {
		return fmt.Errorf("dataMode %q is not supported", r.DataMode)
	}
	if err := r.Target.Validate(); err != nil {
		return fmt.Errorf("target: %w", err)
	}
	return nil
}

// ScanOrchestrator 负责协调依赖解析、漏洞查询、风险评估与报告组装。
// 实现必须使用传入的 context 实现取消与超时，并保证缺失结果不会产生 clean 状态。
type ScanOrchestrator interface {
	Scan(ctx context.Context, request ScanRequest) (model.ScanReport, error)
}

// RegistryMetadataProvider 提供单个生态系统的包元数据与依赖需求。
type RegistryMetadataProvider interface {
	Ecosystem() model.Ecosystem
	PackageMetadata(ctx context.Context, pkg model.PackageIdentity) (RegistryMetadata, error)
	Dependencies(ctx context.Context, pkg model.PackageIdentity, version string) ([]DependencyRequirement, error)
}

// RegistryMetadata 表示注册表返回的包级元数据。
type RegistryMetadata struct {
	Package           model.PackageIdentity
	Versions          []string
	LatestVersion     *string
	LatestPublishedAt *time.Time
	RepositoryURL     *string
	Source            model.SourceReference
	Limitations       []model.Limitation
}

// DependencyRequirement 表示某个已解析版本声明的依赖需求。
type DependencyRequirement struct {
	Package    model.PackageIdentity
	Constraint string
	Optional   bool
}

// LifecycleScript 表示发布包声明的生命周期命令；命令只作为证据展示，绝不执行。
type LifecycleScript struct {
	Name    string
	Command string
}

// VersionMetadata 表示某个具体版本的元数据与声明信息。
type VersionMetadata struct {
	Package          model.PackageIdentity
	Version          string
	PublishedAt      *time.Time
	RepositoryURL    *string
	Deprecated       *string
	LifecycleScripts []LifecycleScript
	Dependencies     []DependencyRequirement
	Source           model.SourceReference
	Limitations      []model.Limitation
}

// VersionMetadataProvider 是 RegistryMetadataProvider 的可选扩展。
// 支持该接口的注册表可以提供版本发布日期、仓库地址、生命周期脚本等详情。
type VersionMetadataProvider interface {
	VersionMetadata(ctx context.Context, pkg model.PackageIdentity, version string) (VersionMetadata, error)
}

// VulnerabilityProvider 按包与已解析版本查询适用的漏洞公告。
// 返回的公告必须已经匹配该版本，禁止由提供者伪造或补全缺失数据。
type VulnerabilityProvider interface {
	Query(ctx context.Context, pkg model.PackageIdentity, version string) (VulnerabilityResult, error)
}

// VulnerabilityResult 表示一次漏洞查询的结果与来源。
type VulnerabilityResult struct {
	Advisories  []Advisory
	Source      model.SourceReference
	Limitations []model.Limitation
}

// Advisory 表示数据来源返回的漏洞公告。
type Advisory struct {
	ID             string
	Aliases        []string
	Summary        string
	Details        string
	Severity       model.Severity
	References     []string
	FixedVersions  []string
	AffectedRanges []string
}

// RiskInput 表示一次单条发现的风险评估输入。
type RiskInput struct {
	Finding model.Finding
}

// RiskEvaluator 评估单条发现的风险；评分方法尚未确定，接口不预设公式。
type RiskEvaluator interface {
	Evaluate(ctx context.Context, input RiskInput) (model.RiskAssessment, error)
}

// Cache 是引擎可选的字节缓存契约。
// 实现必须遵守 context 取消；ttl 为零或负数时的语义由实现明确记录。
type Cache interface {
	Get(ctx context.Context, key string) (value []byte, found bool, err error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// NoopCache 是禁用缓存时使用的空实现。
type NoopCache struct{}

// Get 始终返回缓存未命中。
func (NoopCache) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, nil
}

// Set 忽略写入并返回成功。
func (NoopCache) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

// ProviderErrorKind 表示外部提供者失败的分类。
type ProviderErrorKind string

const (
	// ProviderErrorNotFound 表示请求的包或版本不存在。
	ProviderErrorNotFound ProviderErrorKind = "not_found"
	// ProviderErrorUnavailable 表示来源暂时不可用。
	ProviderErrorUnavailable ProviderErrorKind = "unavailable"
	// ProviderErrorRateLimited 表示来源触发了速率限制。
	ProviderErrorRateLimited ProviderErrorKind = "rate_limited"
	// ProviderErrorInvalidResponse 表示来源返回了无法解析的数据。
	ProviderErrorInvalidResponse ProviderErrorKind = "invalid_response"
	// ProviderErrorTimeout 表示来源在截止时间前未响应。
	ProviderErrorTimeout ProviderErrorKind = "timeout"
)

// IsValid 报告错误分类是否有效。
func (k ProviderErrorKind) IsValid() bool {
	switch k {
	case ProviderErrorNotFound, ProviderErrorUnavailable, ProviderErrorRateLimited, ProviderErrorInvalidResponse, ProviderErrorTimeout:
		return true
	default:
		return false
	}
}

// ProviderError 将外部提供者失败分类，便于编排层映射为 unavailable 或 failed 检查结果。
type ProviderError struct {
	Provider string
	Kind     ProviderErrorKind
	Err      error
}

// Error 返回包含提供者与失败分类的错误文本。
func (e *ProviderError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err == nil {
		return fmt.Sprintf("provider %s: %s", e.Provider, e.Kind)
	}
	return fmt.Sprintf("provider %s: %s: %v", e.Provider, e.Kind, e.Err)
}

// Unwrap 返回底层错误。
func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
