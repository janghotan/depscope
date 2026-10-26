package live_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"depscope/internal/contract"
	"depscope/internal/engine"
	"depscope/internal/live"
	"depscope/internal/model"
)

func TestNewScannerWiresProvidersAndCache(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	const metadataFixture = `{
      "name": "factory-pkg",
      "dist-tags": {"latest": "1.0.0"},
      "versions": {"1.0.0": {"name": "factory-pkg", "version": "1.0.0"}},
      "time": {"1.0.0": "2024-01-01T00:00:00.000Z"}
    }`

	var npmRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/factory-pkg":
			npmRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(metadataFixture))
		case "/v1/query":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"vulns":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	checker, err := engine.NewTyposquatChecker([]byte(`{
      "datasetVersion": "factory-1",
      "updatedAt": "2026-10-25T00:00:00Z",
      "provenance": "Factory test reference seed.",
      "updateInstructions": "Verify names against the official registry before updating.",
      "ecosystems": {"npm": ["react"], "pypi": ["requests"]}
    }`))
	if err != nil {
		t.Fatalf("NewTyposquatChecker: %v", err)
	}

	scanner, err := live.NewScanner(live.Options{
		Config: engine.Config{
			ScanTimeout:     5 * time.Second,
			ProviderTimeout: 2 * time.Second,
			MaxConcurrency:  2,
			CacheEnabled:    true,
			CacheTTL:        time.Minute,
		},
		NPMBaseURL:  server.URL,
		PyPIBaseURL: server.URL,
		OSVBaseURL:  server.URL,
		Typosquat:   checker,
		Clock:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewScanner: %v", err)
	}

	pkg := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "factory-pkg"}
	report, err := scanner.Scan(context.Background(), engine.ScanRequest{
		Target:   model.ScanTarget{Package: pkg},
		DataMode: model.DataModeLive,
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Status != model.StatusClean {
		t.Fatalf("Status = %q, checks = %+v, limitations = %+v", report.Status, report.Checks, report.Limitations)
	}
	if npmRequests.Load() != 1 {
		t.Fatalf("npm requests = %d, want 1 cache hit", npmRequests.Load())
	}
	if err := contract.ValidateReport(report); err != nil {
		t.Fatalf("report is not valid: %v", err)
	}
}
