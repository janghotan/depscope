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
