// Package demo 提供完全离线、确定性的演示扫描场景。
// 所有数据均为合成数据，使用共享引擎完成检查与风险评估，绝不访问网络或执行依赖。
package demo

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/model"
)

const (
	// Notice 是每个演示报告都必须携带的固定说明。
	Notice = "DEMO DATA: Deterministic synthetic results. No network requests were made; this is not a real security assessment."
	// DataTime 是演示数据统一使用的固定时间戳。
	DataTime = "2026-01-01T00:00:00Z"
)

var demoTime = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

// Scenario 描述一个可运行的演示场景。
type Scenario struct {
	ID          string
	Title       string
	Description string
}

type scenario struct {
	info        Scenario
	pkg         model.PackageIdentity
	version     string
	publishedAt time.Time
	deprecated  *string
	scripts     []engine.LifecycleScript
	advisories  []engine.Advisory
}

var scenarios = []scenario{
	{
		info: Scenario{
			ID:          "vulnerable",
			Title:       "Known vulnerabilities",
			Description: "A synthetic npm release with confirmed critical and medium advisories, fixed versions, and risk scores.",
		},
		pkg:         model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "@depscope-demo/vulnerable-package"},
		version:     "2.0.0",
		publishedAt: time.Date(2025, time.November, 4, 9, 30, 0, 0, time.UTC),
		advisories: []engine.Advisory{
			{
				ID:             "DEMO-2026-0001",
				Aliases:        []string{"DEMO-ALIAS-0001"},
				Summary:        "Demo remote code execution in request parsing",
				Details:        "DEMO DATA: Synthetic advisory for a critical confirmed finding. The demo package parses untrusted configuration without validation.",
				Severity:       model.SeverityCritical,
				FixedVersions:  []string{"2.1.0"},
				AffectedRanges: []string{"< 2.1.0"},
			},
			{
				ID:             "DEMO-2026-0002",
				Aliases:        []string{"DEMO-ALIAS-0002"},
				Summary:        "Demo prototype pollution in option merging",
				Details:        "DEMO DATA: Synthetic advisory for a medium confirmed finding. The demo package merges nested options without filtering prototype keys.",
				Severity:       model.SeverityMedium,
				FixedVersions:  []string{"2.0.1"},
				AffectedRanges: []string{"< 2.0.1"},
			},
		},
	},
	{
		info: Scenario{
			ID:          "install-scripts",
			Title:       "Install-script warning",
			Description: "A synthetic npm release declaring preinstall and postinstall commands, plus a deprecated maintenance signal.",
		},
		pkg:         model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "@depscope-demo/scripted-package"},
		version:     "1.2.0",
		publishedAt: time.Date(2025, time.December, 2, 16, 45, 0, 0, time.UTC),
		deprecated:  stringPointer("DEMO DATA: synthetic release marked deprecated to demonstrate maintenance context."),
		scripts: []engine.LifecycleScript{
			{Name: "preinstall", Command: "node ./scripts/check-runtime.js"},
			{Name: "postinstall", Command: "node ./scripts/fetch-assets.js --quiet"},
		},
	},
	{
		info: Scenario{
			ID:          "clean",
			Title:       "No findings",
			Description: "A synthetic npm release with no advisories, no install scripts, and complete successful checks.",
		},
		pkg:         model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "@depscope-demo/clean-package"},
		version:     "1.0.0",
		publishedAt: time.Date(2025, time.September, 18, 11, 15, 0, 0, time.UTC),
	},
}

// Scenarios 按固定顺序返回可用的演示场景。
func Scenarios() []Scenario {
	result := make([]Scenario, 0, len(scenarios))
	for _, item := range scenarios {
		result = append(result, item.info)
	}
	return result
}

// Scan 使用共享引擎生成指定场景的确定性演示报告。
func Scan(ctx context.Context, id string) (model.ScanReport, error) {
	item, ok := findScenario(id)
	if !ok {
		return model.ScanReport{}, fmt.Errorf("unknown demo scenario %q; available scenarios: %s", id, scenarioIDs())
	}
	if err := ctx.Err(); err != nil {
		return model.ScanReport{}, err
	}
	scanner, err := buildScanner(item)
	if err != nil {
		return model.ScanReport{}, err
	}
	request := engine.ScanRequest{
		Target:   model.ScanTarget{Package: item.pkg},
		DataMode: model.DataModeDemo,
	}
	return scanner.Scan(ctx, request)
}

func findScenario(id string) (scenario, bool) {
	for _, item := range scenarios {
		if item.info.ID == id {
			return item, true
		}
	}
	return scenario{}, false
}

func scenarioIDs() string {
	ids := make([]string, 0, len(scenarios))
	for _, item := range scenarios {
		ids = append(ids, item.info.ID)
	}
	sort.Strings(ids)
	return fmt.Sprint(ids)
}

func buildScanner(item scenario) (*engine.Scanner, error) {
	source := model.SourceReference{
		ID:          "demo:registry",
		Provider:    model.SourceProviderDemo,
		Identifier:  "demo:registry:" + string(item.pkg.Ecosystem),
		RetrievedAt: demoTime,
	}
	vulnerabilitySource := model.SourceReference{
		ID:          "demo:advisories",
		Provider:    model.SourceProviderDemo,
		Identifier:  "demo:advisories:v1",
		RetrievedAt: demoTime,
	}
	config := engine.DefaultConfig()
	config.CacheEnabled = false
	config.CacheTTL = 0
	return engine.NewScanner(engine.ScannerOptions{
		Config: config,
		Registries: map[model.Ecosystem]engine.RegistryMetadataProvider{
			item.pkg.Ecosystem: registry{data: item, source: source},
		},
		Vulnerabilities: vulnerabilities{data: item, source: vulnerabilitySource},
		Risk:            engine.RulesRiskEvaluator{},
		Clock:           func() time.Time { return demoTime },
		DataMode:        model.DataModeDemo,
		DemoNotice:      Notice,
	})
}

type registry struct {
	data   scenario
	source model.SourceReference
}

// Ecosystem 返回演示注册表支持的生态系统。
func (r registry) Ecosystem() model.Ecosystem { return r.data.pkg.Ecosystem }

// PackageMetadata 返回固定的合成包元数据。
func (r registry) PackageMetadata(ctx context.Context, pkg model.PackageIdentity) (engine.RegistryMetadata, error) {
	if err := ctx.Err(); err != nil {
		return engine.RegistryMetadata{}, err
	}
	if pkg != r.data.pkg {
		return engine.RegistryMetadata{}, &engine.ProviderError{
			Provider: "demo",
			Kind:     engine.ProviderErrorNotFound,
			Err:      fmt.Errorf("package %s is not part of this demo scenario", pkg.String()),
		}
	}
	latest := r.data.version
	published := r.data.publishedAt
	return engine.RegistryMetadata{
		Package:           r.data.pkg,
		Versions:          []string{r.data.version},
		LatestVersion:     &latest,
		LatestPublishedAt: &published,
		Source:            r.source,
		Limitations:       []model.Limitation{},
	}, nil
}

// Dependencies 返回空的直接依赖列表；演示场景专注于单包风险展示。
func (r registry) Dependencies(context.Context, model.PackageIdentity, string) ([]engine.DependencyRequirement, error) {
	return []engine.DependencyRequirement{}, nil
}

// VersionMetadata 返回固定的合成版本详情。
func (r registry) VersionMetadata(ctx context.Context, pkg model.PackageIdentity, version string) (engine.VersionMetadata, error) {
	if err := ctx.Err(); err != nil {
		return engine.VersionMetadata{}, err
	}
	if pkg != r.data.pkg || version != r.data.version {
		return engine.VersionMetadata{}, &engine.ProviderError{
			Provider: "demo",
			Kind:     engine.ProviderErrorNotFound,
			Err:      fmt.Errorf("%s@%s is not part of this demo scenario", pkg.String(), version),
		}
	}
	published := r.data.publishedAt
	return engine.VersionMetadata{
		Package:          r.data.pkg,
		Version:          r.data.version,
		PublishedAt:      &published,
		Deprecated:       r.data.deprecated,
		LifecycleScripts: append([]engine.LifecycleScript(nil), r.data.scripts...),
		Dependencies:     []engine.DependencyRequirement{},
		Source:           r.source,
		Limitations:      []model.Limitation{},
	}, nil
}

type vulnerabilities struct {
	data   scenario
	source model.SourceReference
}

// Query 返回固定的合成公告；公告不包含任何外部链接。
func (v vulnerabilities) Query(ctx context.Context, pkg model.PackageIdentity, version string) (engine.VulnerabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return engine.VulnerabilityResult{}, err
	}
	if pkg != v.data.pkg || version != v.data.version {
		return engine.VulnerabilityResult{}, &engine.ProviderError{
			Provider: "demo",
			Kind:     engine.ProviderErrorNotFound,
			Err:      fmt.Errorf("%s@%s is not part of this demo scenario", pkg.String(), version),
		}
	}
	return engine.VulnerabilityResult{
		Advisories:  cloneAdvisories(v.data.advisories),
		Source:      v.source,
		Limitations: []model.Limitation{},
	}, nil
}

func cloneAdvisories(items []engine.Advisory) []engine.Advisory {
	result := make([]engine.Advisory, 0, len(items))
	for _, item := range items {
		item.Aliases = append([]string(nil), item.Aliases...)
		item.References = append([]string(nil), item.References...)
		item.FixedVersions = append([]string(nil), item.FixedVersions...)
		item.AffectedRanges = append([]string(nil), item.AffectedRanges...)
		result = append(result, item)
	}
	return result
}

func stringPointer(value string) *string {
	return &value
}
