package engine_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"depscope/internal/cache"
	"depscope/internal/contract"
	"depscope/internal/engine"
	"depscope/internal/fetch"
	"depscope/internal/model"
	"depscope/internal/sources/npm"
	"depscope/internal/sources/osv"
	"depscope/internal/sources/pypi"
)

const integrationNow = "2026-10-25T12:00:00Z"

func integrationTyposquat(t *testing.T) *engine.TyposquatChecker {
	t.Helper()
	checker, err := engine.NewTyposquatChecker([]byte(`{
      "datasetVersion": "integration-1",
      "updatedAt": "2026-10-25T00:00:00Z",
      "provenance": "Integration test reference seed; not a popularity list.",
      "updateInstructions": "Replace after verifying names against the official registry.",
      "ecosystems": {"npm": ["react"], "pypi": ["requests"]}
    }`))
	if err != nil {
		t.Fatalf("NewTyposquatChecker: %v", err)
	}
	return checker
}

func integrationClient() *fetch.Client {
	now := func() time.Time { return fixedNow }
	return fetch.New(fetch.Config{
		Cache:        cache.NewMemoryCache(64, now),
		CacheEnabled: true,
		CacheTTL:     time.Minute,
		MaxRetries:   0,
		Now:          now,
	})
}

func integrationScanner(t *testing.T, registries map[model.Ecosystem]engine.RegistryMetadataProvider, vulnerabilities engine.VulnerabilityProvider) *engine.Scanner {
	t.Helper()
	scanner, err := engine.NewScanner(engine.ScannerOptions{
		Config: engine.Config{
			ScanTimeout:     5 * time.Second,
			ProviderTimeout: 2 * time.Second,
			MaxConcurrency:  2,
			CacheEnabled:    true,
			CacheTTL:        time.Minute,
		},
		Registries:      registries,
		Vulnerabilities: vulnerabilities,
		Risk:            engine.RulesRiskEvaluator{},
		Typosquat:       integrationTyposquat(t),
		Clock:           func() time.Time { return fixedNow },
	})
	if err != nil {
		t.Fatalf("NewScanner: %v", err)
	}
	return scanner
}

func validateIntegrationReport(t *testing.T, report model.ScanReport) {
	t.Helper()
	if err := contract.ValidateReport(report); err != nil {
		t.Fatalf("report is not valid: %v\nreport: %+v", err, report)
	}
}

func TestIntegrationNPMVulnerablePackage(t *testing.T) {
	const metadataFixture = `{
      "name": "integration-npm",
      "dist-tags": {"latest": "1.0.0"},
      "versions": {
        "1.0.0": {"name": "integration-npm", "version": "1.0.0", "repository": {"url": "git+https://example.invalid/integration-npm.git"}},
        "1.0.1": {"name": "integration-npm", "version": "1.0.1"}
      },
      "time": {
        "1.0.0": "2024-01-01T00:00:00.000Z",
        "1.0.1": "2024-02-01T00:00:00.000Z"
      },
      "repository": {"url": "git+https://example.invalid/integration-npm.git"}
    }`
	const vulnerabilityFixture = `{
      "vulns": [{
        "id": "GHSA-integration-npm",
        "summary": "Integration vulnerability",
        "details": "A confirmed vulnerability used by the integration test.",
        "aliases": ["CVE-2026-1000"],
        "references": [{"type": "ADVISORY", "url": "https://example.invalid/GHSA-integration-npm"}],
        "affected": [{
          "package": {"ecosystem": "npm", "name": "integration-npm"},
          "ranges": [{"type": "SEMVER", "events": [{"introduced": "0"}, {"fixed": "1.0.1"}]}],
          "database_specific": {"severity": "HIGH"}
        }]
      }]
    }`

	var npmRequests atomic.Int32
	var osvRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/integration-npm":
			npmRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(metadataFixture))
		case "/v1/query":
			osvRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(vulnerabilityFixture))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := integrationClient()
	scanner := integrationScanner(t,
		map[model.Ecosystem]engine.RegistryMetadataProvider{
			model.EcosystemNPM: npm.NewProvider(npm.Options{BaseURL: server.URL, Client: client, Now: func() time.Time { return fixedNow }}),
		},
		osv.NewProvider(osv.Options{BaseURL: server.URL, Client: client, Now: func() time.Time { return fixedNow }}),
	)

	pkg := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "integration-npm"}
	report, err := scanner.Scan(context.Background(), engine.ScanRequest{
		Target:   model.ScanTarget{Package: pkg},
		DataMode: model.DataModeLive,
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusFindings {
		t.Fatalf("Status = %q, checks = %+v, limitations = %+v", report.Status, report.Checks, report.Limitations)
	}
	if report.Target.ResolvedVersion == nil || *report.Target.ResolvedVersion != "1.0.0" {
		t.Fatalf("ResolvedVersion = %v", report.Target.ResolvedVersion)
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
	if check := checkByID(report, "vulnerabilities"); check == nil || check.Status != model.CheckSucceeded {
		t.Fatalf("vulnerability check = %+v", check)
	}
	if check := checkByID(report, "registry-metadata"); check == nil || !strings.Contains(check.Message, "https://example.invalid/integration-npm") {
		t.Fatalf("registry check = %+v", check)
	}
	if npmRequests.Load() != 1 {
		t.Fatalf("npm requests = %d, want 1 cache hit across package and version metadata", npmRequests.Load())
	}
	if osvRequests.Load() != 1 {
		t.Fatalf("osv requests = %d, want 1", osvRequests.Load())
	}
	validateIntegrationReport(t, report)
}

func TestIntegrationPyPIPartialFailureKeepsConfirmedFinding(t *testing.T) {
	const projectFixture = `{
      "info": {
        "name": "example-pkg",
        "version": "2.0.0",
        "project_urls": {"Source": "https://example.invalid/example-pkg"},
        "requires_dist": ["requests>=2.0"],
        "yanked": false
      },
      "releases": {
        "1.0.0": [{"upload_time_iso_8601": "2024-01-01T00:00:00Z"}],
        "2.0.0": [{"upload_time_iso_8601": "2024-02-01T00:00:00Z"}]
      },
      "urls": [{"upload_time_iso_8601": "2024-02-01T00:00:00Z"}]
    }`
	const vulnerabilityFixture = `{
      "vulns": [{
        "id": "GHSA-integration-pypi",
        "summary": "PyPI integration vulnerability",
        "details": "A confirmed vulnerability used by the integration test.",
        "references": [{"type": "ADVISORY", "url": "https://example.invalid/GHSA-integration-pypi"}],
        "affected": [{
          "package": {"ecosystem": "PyPI", "name": "example-pkg"},
          "ranges": [{"type": "ECOSYSTEM", "events": [{"introduced": "0"}, {"fixed": "2.0.1"}]}],
          "database_specific": {"severity": "CRITICAL"}
        }]
      }]
    }`

	var versionRequests atomic.Int32
	var queryBody atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pypi/example-pkg/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(projectFixture))
		case "/pypi/example-pkg/2.0.0/json":
			versionRequests.Add(1)
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		case "/v1/query":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			queryBody.Store(body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(vulnerabilityFixture))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := integrationClient()
	scanner := integrationScanner(t,
		map[model.Ecosystem]engine.RegistryMetadataProvider{
			model.EcosystemPyPI: pypi.NewProvider(pypi.Options{BaseURL: server.URL, Client: client, Now: func() time.Time { return fixedNow }}),
		},
		osv.NewProvider(osv.Options{BaseURL: server.URL, Client: client, Now: func() time.Time { return fixedNow }}),
	)

	pkg := model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "example-pkg"}
	requested := "2.0.0"
	report, err := scanner.Scan(context.Background(), engine.ScanRequest{
		Target:   model.ScanTarget{Package: pkg, ResolvedVersion: &requested},
		DataMode: model.DataModeLive,
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusIncomplete {
		t.Fatalf("Status = %q, checks = %+v", report.Status, report.Checks)
	}
	if check := checkByID(report, "dependency-metadata"); check == nil || check.Status != model.CheckUnavailable {
		t.Fatalf("dependency check = %+v", check)
	}
	if check := checkByID(report, "dependency-graph"); check == nil || check.Status != model.CheckUnavailable {
		t.Fatalf("graph check = %+v", check)
	}
	if check := checkByID(report, "vulnerabilities"); check == nil || check.Status != model.CheckSucceeded {
		t.Fatalf("vulnerability check = %+v", check)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("Findings = %+v", report.Findings)
	}
	finding := report.Findings[0]
	if finding.Kind != model.FindingConfirmed || finding.Severity != model.SeverityCritical {
		t.Fatalf("Finding = %+v", finding)
	}
	if finding.Risk == nil || finding.Risk.Score != 95 {
		t.Fatalf("Risk = %+v", finding.Risk)
	}
	if versionRequests.Load() != 1 {
		t.Fatalf("version metadata requests = %d, want 1", versionRequests.Load())
	}

	body, ok := queryBody.Load().(map[string]any)
	if !ok {
		t.Fatal("OSV query body was not captured")
	}
	packageValue, ok := body["package"].(map[string]any)
	if !ok {
		t.Fatalf("OSV query package = %#v", body["package"])
	}
	if packageValue["ecosystem"] != "PyPI" || packageValue["name"] != "example-pkg" {
		t.Fatalf("OSV query package = %#v", packageValue)
	}
	if body["version"] != "2.0.0" {
		t.Fatalf("OSV query version = %#v", body["version"])
	}
	validateIntegrationReport(t, report)
}

func TestIntegrationNPMRegistryOutageIsFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "registry maintenance", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	client := integrationClient()
	scanner := integrationScanner(t,
		map[model.Ecosystem]engine.RegistryMetadataProvider{
			model.EcosystemNPM: npm.NewProvider(npm.Options{BaseURL: server.URL, Client: client, Now: func() time.Time { return fixedNow }}),
		},
		osv.NewProvider(osv.Options{BaseURL: server.URL, Client: client, Now: func() time.Time { return fixedNow }}),
	)

	pkg := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "integration-npm"}
	report, err := scanner.Scan(context.Background(), engine.ScanRequest{
		Target:   model.ScanTarget{Package: pkg},
		DataMode: model.DataModeLive,
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusFailed {
		t.Fatalf("Status = %q, want failed", report.Status)
	}
	if check := checkByID(report, "registry-metadata"); check == nil || check.Status != model.CheckFailed {
		t.Fatalf("registry check = %+v", check)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("Findings = %+v", report.Findings)
	}
	validateIntegrationReport(t, report)
}
