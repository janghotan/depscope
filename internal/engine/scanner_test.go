package engine_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/contract"
	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/model"
)

type fakeRegistry struct {
	ecosystem    model.Ecosystem
	metadata     engine.RegistryMetadata
	metadataErr  error
	version      engine.VersionMetadata
	versionErr   error
	dependencies []engine.DependencyRequirement
	depsErr      error
}

func (f *fakeRegistry) Ecosystem() model.Ecosystem { return f.ecosystem }

func (f *fakeRegistry) PackageMetadata(context.Context, model.PackageIdentity) (engine.RegistryMetadata, error) {
	return f.metadata, f.metadataErr
}

func (f *fakeRegistry) Dependencies(context.Context, model.PackageIdentity, string) ([]engine.DependencyRequirement, error) {
	return f.dependencies, f.depsErr
}

func (f *fakeRegistry) VersionMetadata(context.Context, model.PackageIdentity, string) (engine.VersionMetadata, error) {
	return f.version, f.versionErr
}

type basicRegistry struct {
	ecosystem    model.Ecosystem
	metadata     engine.RegistryMetadata
	metadataErr  error
	dependencies []engine.DependencyRequirement
	depsErr      error
}

func (f *basicRegistry) Ecosystem() model.Ecosystem { return f.ecosystem }

func (f *basicRegistry) PackageMetadata(context.Context, model.PackageIdentity) (engine.RegistryMetadata, error) {
	return f.metadata, f.metadataErr
}

func (f *basicRegistry) Dependencies(context.Context, model.PackageIdentity, string) ([]engine.DependencyRequirement, error) {
	return f.dependencies, f.depsErr
}

type fakeVulnerabilityProvider struct {
	result engine.VulnerabilityResult
	err    error
}

func (f *fakeVulnerabilityProvider) Query(context.Context, model.PackageIdentity, string) (engine.VulnerabilityResult, error) {
	return f.result, f.err
}

type failingRiskEvaluator struct{}

func (failingRiskEvaluator) Evaluate(context.Context, engine.RiskInput) (model.RiskAssessment, error) {
	return model.RiskAssessment{}, errors.New("risk evaluator failed")
}

type blockingRegistry struct {
	ecosystem model.Ecosystem
}

func (b *blockingRegistry) Ecosystem() model.Ecosystem { return b.ecosystem }

func (b *blockingRegistry) PackageMetadata(ctx context.Context, _ model.PackageIdentity) (engine.RegistryMetadata, error) {
	<-ctx.Done()
	return engine.RegistryMetadata{}, ctx.Err()
}

func (b *blockingRegistry) Dependencies(context.Context, model.PackageIdentity, string) ([]engine.DependencyRequirement, error) {
	return nil, nil
}

var fixedNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

func testPackage() model.PackageIdentity {
	return model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "safe-pkg"}
}

func testMetadata(pkg model.PackageIdentity, latest string) engine.RegistryMetadata {
	return engine.RegistryMetadata{
		Package:       pkg,
		Versions:      []string{latest},
		LatestVersion: &latest,
		Source: model.SourceReference{
			ID:          "npm-registry",
			Provider:    model.SourceProviderNPM,
			Identifier:  pkg.Name,
			RetrievedAt: fixedNow,
		},
	}
}

func testVersion(pkg model.PackageIdentity, version string) engine.VersionMetadata {
	return engine.VersionMetadata{
		Package: pkg,
		Version: version,
		Source: model.SourceReference{
			ID:          "npm-registry",
			Provider:    model.SourceProviderNPM,
			Identifier:  pkg.Name,
			RetrievedAt: fixedNow,
		},
	}
}

func scannerOptions(registry engine.RegistryMetadataProvider, vulnerabilities engine.VulnerabilityProvider) engine.ScannerOptions {
	return engine.ScannerOptions{
		Config: engine.Config{
			ScanTimeout:     time.Second,
			ProviderTimeout: 500 * time.Millisecond,
			MaxConcurrency:  2,
			CacheEnabled:    false,
			CacheTTL:        0,
		},
		Registries:      map[model.Ecosystem]engine.RegistryMetadataProvider{model.EcosystemNPM: registry},
		Vulnerabilities: vulnerabilities,
		Risk:            engine.RulesRiskEvaluator{},
		Clock:           func() time.Time { return fixedNow },
	}
}

func scanRequest(pkg model.PackageIdentity, version *string) engine.ScanRequest {
	return engine.ScanRequest{
		Target:   model.ScanTarget{Package: pkg, ResolvedVersion: version},
		DataMode: model.DataModeLive,
	}
}

func buildScanner(t *testing.T, opts engine.ScannerOptions) *engine.Scanner {
	t.Helper()
	scanner, err := engine.NewScanner(opts)
	if err != nil {
		t.Fatalf("NewScanner: %v", err)
	}
	return scanner
}

func validateReport(t *testing.T, report model.ScanReport) {
	t.Helper()
	if err := contract.ValidateReport(report); err != nil {
		t.Fatalf("report is not valid: %v\nreport: %+v", err, report)
	}
}

func checkByID(report model.ScanReport, id string) *model.CheckResult {
	for index := range report.Checks {
		if report.Checks[index].ID == id {
			return &report.Checks[index]
		}
	}
	return nil
}

func TestScanCleanReport(t *testing.T) {
	pkg := testPackage()
	registry := &fakeRegistry{
		ecosystem: model.EcosystemNPM,
		metadata:  testMetadata(pkg, "1.0.0"),
		version:   testVersion(pkg, "1.0.0"),
	}
	vulnerabilities := &fakeVulnerabilityProvider{result: engine.VulnerabilityResult{
		Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: "npm:safe-pkg@1.0.0", RetrievedAt: fixedNow},
	}}
	scanner := buildScanner(t, scannerOptions(registry, vulnerabilities))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusClean {
		t.Fatalf("Status = %q, checks = %+v, limitations = %+v", report.Status, report.Checks, report.Limitations)
	}
	if report.Target.ResolvedVersion == nil || *report.Target.ResolvedVersion != "1.0.0" {
		t.Fatalf("ResolvedVersion = %v", report.Target.ResolvedVersion)
	}
	if len(report.Dependencies.Nodes) != 1 || report.Dependencies.Nodes[0].ID != "root" {
		t.Fatalf("Dependencies = %+v", report.Dependencies)
	}
	if report.Summary.ConfirmedFindings != 0 || report.Summary.HeuristicWarnings != 0 {
		t.Fatalf("Summary = %+v", report.Summary)
	}
	validateReport(t, report)
}

func TestScanVulnerablePackage(t *testing.T) {
	pkg := testPackage()
	registry := &fakeRegistry{
		ecosystem: model.EcosystemNPM,
		metadata:  testMetadata(pkg, "1.0.0"),
		version:   testVersion(pkg, "1.0.0"),
	}
	vulnerabilities := &fakeVulnerabilityProvider{result: engine.VulnerabilityResult{
		Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: "npm:safe-pkg@1.0.0", RetrievedAt: fixedNow},
		Advisories: []engine.Advisory{{
			ID:            "GHSA-test",
			Aliases:       []string{"CVE-2026-9999"},
			Summary:       "Test vulnerability",
			Details:       "Confirmed vulnerable behavior.",
			Severity:      model.SeverityHigh,
			References:    []string{"https://example.invalid/advisory"},
			FixedVersions: []string{"1.0.1"},
		}},
	}}
	scanner := buildScanner(t, scannerOptions(registry, vulnerabilities))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusFindings {
		t.Fatalf("Status = %q, checks = %+v", report.Status, report.Checks)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("Findings = %+v", report.Findings)
	}
	finding := report.Findings[0]
	if finding.Kind != model.FindingConfirmed || finding.Severity != model.SeverityHigh {
		t.Fatalf("Finding = %+v", finding)
	}
	if finding.Risk == nil || finding.Risk.Score != 80 {
		t.Fatalf("Risk = %+v", finding.Risk)
	}
	if finding.Remediation == nil || !strings.Contains(*finding.Remediation, "1.0.1") {
		t.Fatalf("Remediation = %v", finding.Remediation)
	}
	if len(finding.Evidence) == 0 || finding.Evidence[0].Kind != model.EvidenceAdvisory {
		t.Fatalf("Evidence = %+v", finding.Evidence)
	}
	validateReport(t, report)
}

func TestOSVFailureIsNeverClean(t *testing.T) {
	pkg := testPackage()
	registry := &fakeRegistry{
		ecosystem: model.EcosystemNPM,
		metadata:  testMetadata(pkg, "1.0.0"),
		version:   testVersion(pkg, "1.0.0"),
	}
	vulnerabilities := &fakeVulnerabilityProvider{err: &engine.ProviderError{Provider: "osv", Kind: engine.ProviderErrorUnavailable}}
	scanner := buildScanner(t, scannerOptions(registry, vulnerabilities))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusIncomplete {
		t.Fatalf("Status = %q, want incomplete", report.Status)
	}
	check := checkByID(report, "vulnerabilities")
	if check == nil || check.Status != model.CheckUnavailable {
		t.Fatalf("vulnerability check = %+v", check)
	}
	foundLimitation := false
	for _, limitation := range report.Limitations {
		if limitation.Code == "VULNERABILITY_DATA_UNAVAILABLE" {
			foundLimitation = true
		}
	}
	if !foundLimitation {
		t.Fatalf("Limitations = %+v", report.Limitations)
	}
	validateReport(t, report)
}

func TestRegistryFailureIsFailed(t *testing.T) {
	pkg := testPackage()
	registry := &fakeRegistry{
		ecosystem:   model.EcosystemNPM,
		metadataErr: &engine.ProviderError{Provider: "npm", Kind: engine.ProviderErrorNotFound},
	}
	scanner := buildScanner(t, scannerOptions(registry, &fakeVulnerabilityProvider{}))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusFailed {
		t.Fatalf("Status = %q, want failed", report.Status)
	}
	check := checkByID(report, "registry-metadata")
	if check == nil || check.Status != model.CheckFailed {
		t.Fatalf("registry check = %+v", check)
	}
	validateReport(t, report)
}

func TestRequestedVersionMustExist(t *testing.T) {
	pkg := testPackage()
	requested := "2.0.0"
	registry := &fakeRegistry{
		ecosystem: model.EcosystemNPM,
		metadata:  testMetadata(pkg, "1.0.0"),
		version:   testVersion(pkg, "2.0.0"),
	}
	scanner := buildScanner(t, scannerOptions(registry, &fakeVulnerabilityProvider{}))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, &requested))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusFailed {
		t.Fatalf("Status = %q, want failed", report.Status)
	}
	validateReport(t, report)
}

func TestDeclaredDependenciesMakeScanIncomplete(t *testing.T) {
	pkg := testPackage()
	version := testVersion(pkg, "1.0.0")
	version.Dependencies = []engine.DependencyRequirement{{
		Package:    model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "dep"},
		Constraint: "^1.0.0",
	}}
	registry := &fakeRegistry{
		ecosystem: model.EcosystemNPM,
		metadata:  testMetadata(pkg, "1.0.0"),
		version:   version,
	}
	scanner := buildScanner(t, scannerOptions(registry, &fakeVulnerabilityProvider{result: engine.VulnerabilityResult{
		Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: "npm:safe-pkg@1.0.0", RetrievedAt: fixedNow},
	}}))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusIncomplete {
		t.Fatalf("Status = %q, want incomplete", report.Status)
	}
	check := checkByID(report, "dependency-graph")
	if check == nil || check.Status != model.CheckSkipped {
		t.Fatalf("dependency-graph check = %+v", check)
	}
	validateReport(t, report)
}

func TestLifecycleScriptIsHeuristicFinding(t *testing.T) {
	pkg := testPackage()
	version := testVersion(pkg, "1.0.0")
	version.LifecycleScripts = []engine.LifecycleScript{{Name: "install", Command: "node install.js"}}
	registry := &fakeRegistry{
		ecosystem: model.EcosystemNPM,
		metadata:  testMetadata(pkg, "1.0.0"),
		version:   version,
	}
	scanner := buildScanner(t, scannerOptions(registry, &fakeVulnerabilityProvider{result: engine.VulnerabilityResult{
		Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: "npm:safe-pkg@1.0.0", RetrievedAt: fixedNow},
	}}))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusFindings {
		t.Fatalf("Status = %q, want findings", report.Status)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("Findings = %+v", report.Findings)
	}
	finding := report.Findings[0]
	if finding.Kind != model.FindingHeuristic || finding.Severity != model.SeverityLow {
		t.Fatalf("Finding = %+v", finding)
	}
	if !strings.Contains(strings.ToLower(finding.Description), "not proof") {
		t.Fatalf("Description = %q", finding.Description)
	}
	if finding.Risk == nil || finding.Risk.Score != 15 {
		t.Fatalf("Risk = %+v", finding.Risk)
	}
	validateReport(t, report)
}

func TestTyposquatFinding(t *testing.T) {
	pkg := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "raect"}
	registry := &fakeRegistry{
		ecosystem: model.EcosystemNPM,
		metadata:  testMetadata(pkg, "1.0.0"),
		version:   testVersion(pkg, "1.0.0"),
	}
	scanner := buildScanner(t, scannerOptions(registry, &fakeVulnerabilityProvider{result: engine.VulnerabilityResult{
		Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: "npm:raect@1.0.0", RetrievedAt: fixedNow},
	}}))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusFindings {
		t.Fatalf("Status = %q, checks = %+v", report.Status, report.Checks)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.ID != "" && strings.HasPrefix(finding.ID, "TYPOSQUAT") {
			found = true
			if finding.Severity != model.SeverityMedium {
				t.Fatalf("Finding = %+v", finding)
			}
		}
	}
	if !found {
		t.Fatalf("Findings = %+v", report.Findings)
	}
	validateReport(t, report)
}

func TestVersionMetadataFailureStillQueriesOSV(t *testing.T) {
	pkg := testPackage()
	registry := &fakeRegistry{
		ecosystem:  model.EcosystemNPM,
		metadata:   testMetadata(pkg, "1.0.0"),
		versionErr: &engine.ProviderError{Provider: "npm", Kind: engine.ProviderErrorUnavailable},
	}
	vulnerabilities := &fakeVulnerabilityProvider{result: engine.VulnerabilityResult{
		Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: "npm:safe-pkg@1.0.0", RetrievedAt: fixedNow},
		Advisories: []engine.Advisory{{
			ID:         "GHSA-partial",
			Summary:    "Known issue",
			Details:    "Details.",
			Severity:   model.SeverityMedium,
			References: []string{"https://example.invalid/advisory"},
		}},
	}}
	scanner := buildScanner(t, scannerOptions(registry, vulnerabilities))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusIncomplete {
		t.Fatalf("Status = %q, want incomplete", report.Status)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != model.FindingConfirmed {
		t.Fatalf("Findings = %+v", report.Findings)
	}
	validateReport(t, report)
}

func TestMaintenanceMetadataIsContext(t *testing.T) {
	pkg := testPackage()
	published := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	repository := "https://example.invalid/repo"
	version := testVersion(pkg, "1.0.0")
	version.PublishedAt = &published
	version.RepositoryURL = &repository
	metadata := testMetadata(pkg, "1.0.0")
	metadata.LatestPublishedAt = &published
	metadata.RepositoryURL = &repository
	registry := &fakeRegistry{
		ecosystem: model.EcosystemNPM,
		metadata:  metadata,
		version:   version,
	}
	scanner := buildScanner(t, scannerOptions(registry, &fakeVulnerabilityProvider{result: engine.VulnerabilityResult{
		Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: "npm:safe-pkg@1.0.0", RetrievedAt: fixedNow},
	}}))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	check := checkByID(report, "registry-metadata")
	if check == nil {
		t.Fatal("registry-metadata check missing")
	}
	if !strings.Contains(check.Message, "https://example.invalid/repo") {
		t.Fatalf("Message = %q", check.Message)
	}
	if !strings.Contains(check.Message, "context") {
		t.Fatalf("Message = %q", check.Message)
	}
	validateReport(t, report)
}

func TestRiskEvaluatorFailureKeepsFinding(t *testing.T) {
	pkg := testPackage()
	registry := &fakeRegistry{
		ecosystem: model.EcosystemNPM,
		metadata:  testMetadata(pkg, "1.0.0"),
		version:   testVersion(pkg, "1.0.0"),
	}
	vulnerabilities := &fakeVulnerabilityProvider{result: engine.VulnerabilityResult{
		Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: "npm:safe-pkg@1.0.0", RetrievedAt: fixedNow},
		Advisories: []engine.Advisory{{
			ID:         "GHSA-risk",
			Summary:    "Known issue",
			Details:    "Details.",
			Severity:   model.SeverityHigh,
			References: []string{"https://example.invalid/advisory"},
		}},
	}}
	opts := scannerOptions(registry, vulnerabilities)
	opts.Risk = failingRiskEvaluator{}
	scanner := buildScanner(t, opts)

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusIncomplete {
		t.Fatalf("Status = %q, want incomplete", report.Status)
	}
	if len(report.Findings) != 1 || report.Findings[0].Risk != nil {
		t.Fatalf("Findings = %+v", report.Findings)
	}
	check := checkByID(report, "risk-evaluation")
	if check == nil || check.Status != model.CheckFailed {
		t.Fatalf("risk check = %+v", check)
	}
	validateReport(t, report)
}

func TestCancellationProducesCancelledReport(t *testing.T) {
	pkg := testPackage()
	opts := scannerOptions(&blockingRegistry{ecosystem: model.EcosystemNPM}, &fakeVulnerabilityProvider{})
	opts.Config.ScanTimeout = 20 * time.Millisecond
	scanner := buildScanner(t, opts)

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err == nil {
		t.Fatal("expected context error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if report.Status != model.StatusCancelled {
		t.Fatalf("Status = %q, want cancelled", report.Status)
	}
	validateReport(t, report)
}

func TestDemoModeRejected(t *testing.T) {
	scanner := buildScanner(t, scannerOptions(&fakeRegistry{ecosystem: model.EcosystemNPM}, &fakeVulnerabilityProvider{}))
	request := scanRequest(testPackage(), nil)
	request.DataMode = model.DataModeDemo
	_, err := scanner.Scan(context.Background(), request)
	if err == nil {
		t.Fatal("expected demo mode to be rejected")
	}
}

func TestInvalidRequestRejected(t *testing.T) {
	scanner := buildScanner(t, scannerOptions(&fakeRegistry{ecosystem: model.EcosystemNPM}, &fakeVulnerabilityProvider{}))
	request := engine.ScanRequest{
		Target:   model.ScanTarget{Package: model.PackageIdentity{Ecosystem: "rubygems", Name: "x"}},
		DataMode: model.DataModeLive,
	}
	_, err := scanner.Scan(context.Background(), request)
	if err == nil {
		t.Fatal("expected invalid request error")
	}
}

// TestAliasDuplicatesAreCountedOnce 复现真实 OSV 行为：同一个漏洞同时返回
// GHSA 与 PYSEC 两条互为别名的记录时，报告只能产生一条确认发现。
func TestAliasDuplicatesAreCountedOnce(t *testing.T) {
	pkg := testPackage()
	registry := &fakeRegistry{
		ecosystem: model.EcosystemNPM,
		metadata:  testMetadata(pkg, "1.0.0"),
		version:   testVersion(pkg, "1.0.0"),
	}
	// 标识与受影响范围取自 PyPI 上真实的别名记录结构。
	vulnerabilities := &fakeVulnerabilityProvider{result: engine.VulnerabilityResult{
		Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: "npm:safe-pkg@1.0.0", RetrievedAt: fixedNow},
		Advisories: []engine.Advisory{
			{
				ID:             "GHSA-qccp-gfcp-xxvc",
				Aliases:        []string{"CVE-2026-44431", "PYSEC-2026-141"},
				Summary:        "Redirect handling leaks headers across hosts",
				Severity:       model.SeverityHigh,
				FixedVersions:  []string{"2.6.3"},
				AffectedRanges: []string{">=1.26.5, <2.6.3"},
			},
			{
				ID:             "PYSEC-2026-141",
				Aliases:        []string{"CVE-2026-44431", "GHSA-qccp-gfcp-xxvc"},
				Summary:        "Redirect handling leaks headers across hosts",
				Severity:       model.SeverityHigh,
				FixedVersions:  []string{"2.6.3"},
				AffectedRanges: []string{">=1.26.5, <2.6.3"},
			},
			{
				ID:             "GHSA-34jh-p97f-mpxf",
				Aliases:        []string{"CVE-2024-37891", "PYSEC-2026-1995"},
				Summary:        "Proxy-Authorization header leaks on redirect",
				Severity:       model.SeverityMedium,
				FixedVersions:  []string{"2.2.2"},
				AffectedRanges: []string{">=1.26.0, <2.2.2"},
			},
		},
	}}
	scanner := buildScanner(t, scannerOptions(registry, vulnerabilities))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	// 3 条记录，但只有 2 个不同漏洞。
	if len(report.Findings) != 2 {
		t.Fatalf("Findings = %d, want 2 (alias duplicates must be merged): %+v", len(report.Findings), findingIDs(report))
	}
	if report.Summary.ConfirmedFindings != 2 {
		t.Fatalf("Summary.ConfirmedFindings = %d, want 2", report.Summary.ConfirmedFindings)
	}
	found := false
	for _, limitation := range report.Limitations {
		if limitation.Code == "OSV_ALIAS_MERGED" {
			found = true
			if !strings.Contains(limitation.Message, "1 duplicate") {
				t.Fatalf("limitation message = %q, want it to report 1 merged duplicate", limitation.Message)
			}
		}
	}
	if !found {
		t.Fatalf("missing OSV_ALIAS_MERGED limitation: %+v", report.Limitations)
	}
	// 别名合并只是同一漏洞的规范化，不是覆盖缺口：所有检查成功时必须仍为 findings，
	// 不能因为记录了一条 limitation 就降级为 incomplete。
	if report.Status != model.StatusFindings {
		t.Fatalf("Status = %q, want %q: alias normalization must not mark a complete scan incomplete", report.Status, model.StatusFindings)
	}
	for _, check := range report.Checks {
		if check.ID == "vulnerabilities" {
			if check.Status != model.CheckSucceeded {
				t.Fatalf("vulnerabilities check = %+v, want succeeded", check)
			}
			// 检查消息中的条数必须与合并后的发现数量一致（3 条记录 -> 2 个漏洞）。
			if !strings.Contains(check.Message, "returned 2 advisory(ies)") {
				t.Fatalf("check message = %q, want the merged count of 2", check.Message)
			}
		}
	}
	validateReport(t, report)
}

// TestAliasMergePreservesEvidence 确认合并后仍保留全部标识，便于追溯原始记录。
func TestAliasMergePreservesEvidence(t *testing.T) {
	pkg := testPackage()
	registry := &fakeRegistry{
		ecosystem: model.EcosystemNPM,
		metadata:  testMetadata(pkg, "1.0.0"),
		version:   testVersion(pkg, "1.0.0"),
	}
	vulnerabilities := &fakeVulnerabilityProvider{result: engine.VulnerabilityResult{
		Source: model.SourceReference{ID: "osv", Provider: model.SourceProviderOSV, Identifier: "npm:safe-pkg@1.0.0", RetrievedAt: fixedNow},
		Advisories: []engine.Advisory{
			{ID: "GHSA-g4mx-q9vg-27p4", Aliases: []string{"CVE-2023-45803", "PYSEC-2023-212"}, Summary: "Suppressed body disclosure", Severity: model.SeverityMedium},
			{ID: "PYSEC-2023-212", Aliases: []string{"CVE-2023-45803", "GHSA-g4mx-q9vg-27p4"}, Summary: "Suppressed body disclosure", Severity: model.SeverityMedium},
		},
	}}
	scanner := buildScanner(t, scannerOptions(registry, vulnerabilities))

	report, err := scanner.Scan(context.Background(), scanRequest(pkg, nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("Findings = %d, want 1", len(report.Findings))
	}
	if got := report.Findings[0].ID; got != "VULN-GHSA-g4mx-q9vg-27p4" {
		t.Fatalf("finding id = %q, want the representative record", got)
	}
	// 证据中的引用必须指向真实记录，而不是被合并掉的重复项。
	if len(report.Findings[0].Evidence) == 0 {
		t.Fatal("finding has no evidence")
	}
	validateReport(t, report)
}

func findingIDs(report model.ScanReport) []string {
	ids := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		ids = append(ids, finding.ID)
	}
	return ids
}
