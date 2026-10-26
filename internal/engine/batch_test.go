package engine_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/model"
)

type mappingVulnerabilityProvider struct {
	mu      sync.Mutex
	results map[string]engine.VulnerabilityResult
	errs    map[string]error
	calls   []string
	block   bool
}

func (p *mappingVulnerabilityProvider) Query(ctx context.Context, pkg model.PackageIdentity, version string) (engine.VulnerabilityResult, error) {
	key := pkg.String() + "@" + version
	p.mu.Lock()
	p.calls = append(p.calls, key)
	p.mu.Unlock()
	if p.block {
		<-ctx.Done()
		return engine.VulnerabilityResult{}, ctx.Err()
	}
	if err := p.errs[key]; err != nil {
		return engine.VulnerabilityResult{}, err
	}
	return p.results[key], nil
}

func (p *mappingVulnerabilityProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

func batchScanner(t *testing.T, provider engine.VulnerabilityProvider, risk engine.RiskEvaluator) *engine.Scanner {
	t.Helper()
	opts := scannerOptions(&basicRegistry{ecosystem: model.EcosystemNPM}, provider)
	if risk != nil {
		opts.Risk = risk
	}
	return buildScanner(t, opts)
}

func batchResultByPackage(results []engine.PackageScanResult, name string) *engine.PackageScanResult {
	for index := range results {
		if results[index].Package.Name == name {
			return &results[index]
		}
	}
	return nil
}

func checkByIDForBatch(checks []model.CheckResult, id string) *model.CheckResult {
	for index := range checks {
		if checks[index].ID == id {
			return &checks[index]
		}
	}
	return nil
}

func TestScanResolvedBatchSortsAndDeduplicates(t *testing.T) {
	pkgA, _ := model.NewPackageIdentity(model.EcosystemNPM, "a-pkg")
	pkgB, _ := model.NewPackageIdentity(model.EcosystemNPM, "b-pkg")
	provider := &mappingVulnerabilityProvider{
		results: map[string]engine.VulnerabilityResult{
			pkgA.String() + "@1.0.0": {
				Advisories: []engine.Advisory{{
					ID:            "GHSA-a",
					Summary:       "A advisory",
					Severity:      model.SeverityHigh,
					References:    []string{"https://osv.dev/GHSA-a"},
					FixedVersions: []string{"1.0.1"},
				}},
				Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: pkgA.String() + "@1.0.0", RetrievedAt: fixedNow},
			},
			pkgB.String() + "@2.0.0": {
				Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: pkgB.String() + "@2.0.0", RetrievedAt: fixedNow},
			},
		},
	}
	scanner := batchScanner(t, provider, nil)

	results, err := scanner.ScanResolvedBatch(context.Background(), []engine.ResolvedPackage{
		{Package: pkgB, Version: "2.0.0"},
		{Package: pkgA, Version: "1.0.0"},
		{Package: pkgA, Version: "1.0.0"},
	})
	if err != nil {
		t.Fatalf("ScanResolvedBatch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].Package != pkgA || results[1].Package != pkgB {
		t.Fatalf("results not sorted deterministically: %+v", results)
	}
	if provider.callCount() != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.callCount())
	}

	a := results[0]
	if len(a.Findings) != 1 {
		t.Fatalf("a findings = %d, want 1", len(a.Findings))
	}
	if a.Findings[0].Risk == nil || a.Findings[0].Risk.Score != 80 {
		t.Fatalf("a finding risk = %+v, want score 80", a.Findings[0].Risk)
	}
	if len(a.Checks) != 1 || a.Checks[0].ID != "vulnerabilities" || a.Checks[0].Status != model.CheckSucceeded {
		t.Fatalf("a checks = %+v, want one succeeded vulnerabilities check", a.Checks)
	}
	if len(a.Sources) != 1 || a.Sources[0].ID != "osv" {
		t.Fatalf("a sources = %+v, want one osv source", a.Sources)
	}

	b := results[1]
	if len(b.Findings) != 0 {
		t.Fatalf("b findings = %d, want 0", len(b.Findings))
	}
	if b.Err != nil {
		t.Fatalf("b err = %v", b.Err)
	}
}

func TestScanResolvedBatchKeepsPartialFailures(t *testing.T) {
	pkgA, _ := model.NewPackageIdentity(model.EcosystemNPM, "a-pkg")
	pkgB, _ := model.NewPackageIdentity(model.EcosystemNPM, "b-pkg")
	provider := &mappingVulnerabilityProvider{
		results: map[string]engine.VulnerabilityResult{
			pkgA.String() + "@1.0.0": {
				Advisories: []engine.Advisory{{
					ID:         "GHSA-a",
					Summary:    "A advisory",
					Severity:   model.SeverityCritical,
					References: []string{"https://osv.dev/GHSA-a"},
				}},
				Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: pkgA.String() + "@1.0.0", RetrievedAt: fixedNow},
			},
		},
		errs: map[string]error{
			pkgB.String() + "@2.0.0": &engine.ProviderError{Provider: "osv", Kind: engine.ProviderErrorUnavailable, Err: errors.New("outage")},
		},
	}
	scanner := batchScanner(t, provider, nil)

	results, err := scanner.ScanResolvedBatch(context.Background(), []engine.ResolvedPackage{
		{Package: pkgA, Version: "1.0.0"},
		{Package: pkgB, Version: "2.0.0"},
	})
	if err != nil {
		t.Fatalf("ScanResolvedBatch: %v", err)
	}
	a := batchResultByPackage(results, "a-pkg")
	if a == nil || len(a.Findings) != 1 {
		t.Fatalf("confirmed finding from successful query was lost: %+v", a)
	}
	b := batchResultByPackage(results, "b-pkg")
	if b == nil || b.Err == nil {
		t.Fatalf("failed query did not retain its error: %+v", b)
	}
	if len(b.Checks) != 1 || b.Checks[0].Status != model.CheckUnavailable {
		t.Fatalf("b checks = %+v, want unavailable", b.Checks)
	}
	if len(b.Limitations) != 1 || b.Limitations[0].Code != "VULNERABILITY_DATA_UNAVAILABLE" {
		t.Fatalf("b limitations = %+v", b.Limitations)
	}
}

func TestScanResolvedBatchRiskFailureKeepsFinding(t *testing.T) {
	pkg, _ := model.NewPackageIdentity(model.EcosystemNPM, "a-pkg")
	provider := &mappingVulnerabilityProvider{
		results: map[string]engine.VulnerabilityResult{
			pkg.String() + "@1.0.0": {
				Advisories: []engine.Advisory{{
					ID:         "GHSA-a",
					Summary:    "A advisory",
					Severity:   model.SeverityHigh,
					References: []string{"https://osv.dev/GHSA-a"},
				}},
				Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: pkg.String() + "@1.0.0", RetrievedAt: fixedNow},
			},
		},
	}
	scanner := batchScanner(t, provider, failingRiskEvaluator{})

	results, err := scanner.ScanResolvedBatch(context.Background(), []engine.ResolvedPackage{{Package: pkg, Version: "1.0.0"}})
	if err != nil {
		t.Fatalf("ScanResolvedBatch: %v", err)
	}
	if len(results[0].Findings) != 1 || results[0].Findings[0].Risk != nil {
		t.Fatalf("finding was not retained without risk: %+v", results[0].Findings)
	}
	if len(results[0].Checks) != 2 {
		t.Fatalf("checks = %+v, want vulnerabilities and risk-evaluation", results[0].Checks)
	}
	riskCheck := checkByIDForBatch(results[0].Checks, "risk-evaluation")
	if riskCheck == nil || riskCheck.Status != model.CheckFailed {
		t.Fatalf("risk check = %+v", riskCheck)
	}
}

func TestScanResolvedBatchHonorsCancellation(t *testing.T) {
	pkg, _ := model.NewPackageIdentity(model.EcosystemNPM, "a-pkg")
	provider := &mappingVulnerabilityProvider{block: true}
	scanner := batchScanner(t, provider, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := scanner.ScanResolvedBatch(ctx, []engine.ResolvedPackage{{Package: pkg, Version: "1.0.0"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestScanResolvedBatchRejectsInvalidPackage(t *testing.T) {
	scanner := batchScanner(t, &mappingVulnerabilityProvider{}, nil)
	_, err := scanner.ScanResolvedBatch(context.Background(), []engine.ResolvedPackage{{Package: model.PackageIdentity{}, Version: "1.0.0"}})
	if err == nil {
		t.Fatal("expected invalid package error")
	}
}

func TestScanResolvedBatchEmptyInput(t *testing.T) {
	scanner := batchScanner(t, &mappingVulnerabilityProvider{}, nil)
	results, err := scanner.ScanResolvedBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("ScanResolvedBatch: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %d, want 0", len(results))
	}
}

func TestScanResolvedBatchFiniteTimeout(t *testing.T) {
	pkg, _ := model.NewPackageIdentity(model.EcosystemNPM, "a-pkg")
	provider := &mappingVulnerabilityProvider{block: true}
	scanner := batchScanner(t, provider, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := scanner.ScanResolvedBatch(ctx, []engine.ResolvedPackage{{Package: pkg, Version: "1.0.0"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

// TestBatchScanMergesAliasDuplicates 验证批量路径（项目扫描与全局清单使用）
// 同样按别名合并同一漏洞的多条记录，而不会为每条记录各生成一条发现。
func TestBatchScanMergesAliasDuplicates(t *testing.T) {
	t.Parallel()

	pkg, err := model.NewPackageIdentity(model.EcosystemPyPI, "urllib3")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	provider := &mappingVulnerabilityProvider{
		results: map[string]engine.VulnerabilityResult{
			"pypi:urllib3@1.26.5": {
				Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: "pypi:urllib3@1.26.5", RetrievedAt: fixedNow},
				Advisories: []engine.Advisory{
					{ID: "GHSA-qccp-gfcp-xxvc", Aliases: []string{"CVE-2026-44431", "PYSEC-2026-141"}, Summary: "Redirect header leak", Severity: model.SeverityHigh},
					{ID: "PYSEC-2026-141", Aliases: []string{"CVE-2026-44431", "GHSA-qccp-gfcp-xxvc"}, Summary: "Redirect header leak", Severity: model.SeverityHigh},
					{ID: "GHSA-2xpw-w6gg-jr37", Aliases: []string{"CVE-2025-66471", "PYSEC-2026-1994"}, Summary: "Decompress bomb", Severity: model.SeverityMedium},
				},
			},
		},
	}
	// 批量扫描仍需为已配置的生态提供注册表实现；PyPI 条目用于满足构造校验。
	opts := scannerOptions(&fakeRegistry{ecosystem: model.EcosystemNPM}, provider)
	opts.Registries[model.EcosystemPyPI] = &fakeRegistry{ecosystem: model.EcosystemPyPI}
	scanner := buildScanner(t, opts)

	results, err := scanner.ScanResolvedBatch(context.Background(), []engine.ResolvedPackage{{Package: pkg, Version: "1.26.5"}})
	if err != nil {
		t.Fatalf("ScanResolvedBatch: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	// 3 条记录，2 个不同漏洞。
	if len(results[0].Findings) != 2 {
		t.Fatalf("findings = %d, want 2: %+v", len(results[0].Findings), results[0].Findings)
	}
	if results[0].Version != "1.26.5" {
		t.Fatalf("result version = %q, want the scanned version", results[0].Version)
	}
	merged := false
	for _, limitation := range results[0].Limitations {
		if limitation.Code == "OSV_ALIAS_MERGED" {
			merged = true
		}
	}
	if !merged {
		t.Fatalf("missing OSV_ALIAS_MERGED limitation: %+v", results[0].Limitations)
	}
	// 别名合并只是同一漏洞的规范化，不构成覆盖缺口；
	// 漏洞检查必须仍为 succeeded，否则清单/项目扫描会被错误降级为 incomplete。
	sawVulnerabilityCheck := false
	for _, check := range results[0].Checks {
		if check.ID == "vulnerabilities" {
			sawVulnerabilityCheck = true
			if check.Status != model.CheckSucceeded {
				t.Fatalf("vulnerabilities check = %q, want succeeded: alias merging must not mark coverage incomplete", check.Status)
			}
		}
	}
	if !sawVulnerabilityCheck {
		t.Fatalf("missing vulnerabilities check: %+v", results[0].Checks)
	}
}
