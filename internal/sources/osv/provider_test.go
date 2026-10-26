package osv_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"depscope/internal/engine"
	"depscope/internal/fetch"
	"depscope/internal/model"
	"depscope/internal/sources/osv"
)

const affectedFixture = `{
  "vulns": [
    {
      "id": "GHSA-xxxx-yyyy-zzzz",
      "aliases": ["CVE-2026-0001"],
      "summary": "Prototype pollution",
      "details": "A crafted input can modify object prototypes.",
      "references": [
        {"type": "ADVISORY", "url": "https://example.invalid/advisory"},
        {"type": "WEB", "url": "https://example.invalid/writeup"}
      ],
      "database_specific": {"severity": "HIGH"},
      "severity": [{"type": "CVSS_V3", "score": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"}],
      "affected": [
        {
          "package": {"ecosystem": "npm", "name": "lodash"},
          "ranges": [
            {"type": "SEMVER", "events": [{"introduced": "0"}, {"fixed": "4.17.21"}]}
          ]
        },
        {
          "package": {"ecosystem": "npm", "name": "other-package"},
          "ranges": [
            {"type": "SEMVER", "events": [{"introduced": "0"}, {"fixed": "9.9.9"}]}
          ]
        }
      ]
    }
  ]
}`

func newProvider(t *testing.T, handler http.Handler, now time.Time) *osv.Provider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return osv.NewProvider(osv.Options{
		BaseURL: server.URL,
		Client: fetch.New(fetch.Config{
			CacheEnabled: true,
			CacheTTL:     time.Minute,
			Now:          func() time.Time { return now },
		}),
		Now: func() time.Time { return now },
	})
}

func TestQueryAffectedVersion(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/query" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		var request struct {
			Version string `json:"version"`
			Package struct {
				Ecosystem string `json:"ecosystem"`
				Name      string `json:"name"`
			} `json:"package"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if request.Version != "4.17.20" || request.Package.Ecosystem != "npm" || request.Package.Name != "lodash" {
			t.Errorf("request = %+v", request)
		}
		_, _ = w.Write([]byte(affectedFixture))
	}), now)

	result, err := provider.Query(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "lodash"}, "4.17.20")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.Advisories) != 1 {
		t.Fatalf("Advisories = %+v", result.Advisories)
	}
	advisory := result.Advisories[0]
	if advisory.ID != "GHSA-xxxx-yyyy-zzzz" {
		t.Fatalf("ID = %q", advisory.ID)
	}
	if len(advisory.Aliases) != 1 || advisory.Aliases[0] != "CVE-2026-0001" {
		t.Fatalf("Aliases = %v", advisory.Aliases)
	}
	if advisory.Severity != model.SeverityHigh {
		t.Fatalf("Severity = %q", advisory.Severity)
	}
	if len(advisory.FixedVersions) != 1 || advisory.FixedVersions[0] != "4.17.21" {
		t.Fatalf("FixedVersions = %v", advisory.FixedVersions)
	}
	if len(advisory.AffectedRanges) != 1 {
		t.Fatalf("AffectedRanges = %v", advisory.AffectedRanges)
	}
	if len(advisory.References) != 2 {
		t.Fatalf("References = %v", advisory.References)
	}
	if result.Source.Provider != model.SourceProviderOSV || result.Source.ID == "" {
		t.Fatalf("Source = %+v", result.Source)
	}
}

func TestQueryUnaffectedVersion(t *testing.T) {
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"vulns":[]}`))
	}), time.Now())

	result, err := provider.Query(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "safe-pkg"}, "1.0.0")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.Advisories) != 0 {
		t.Fatalf("Advisories = %+v", result.Advisories)
	}
}

func TestPyPIEcosystemValue(t *testing.T) {
	var ecosystem string
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Package struct {
				Ecosystem string `json:"ecosystem"`
			} `json:"package"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		ecosystem = request.Package.Ecosystem
		_, _ = w.Write([]byte(`{"vulns":[]}`))
	}), time.Now())

	if _, err := provider.Query(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "requests"}, "2.0.0"); err != nil {
		t.Fatalf("Query: %v", err)
	}
	if ecosystem != "PyPI" {
		t.Fatalf("ecosystem = %q, want PyPI", ecosystem)
	}
}

func TestOsvRateLimit(t *testing.T) {
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}), time.Now())

	_, err := provider.Query(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "lodash"}, "4.17.20")
	var providerErr *engine.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != engine.ProviderErrorRateLimited {
		t.Fatalf("error = %v, want rate_limited", err)
	}
}

func TestOsvMalformedResponse(t *testing.T) {
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"vulns":`))
	}), time.Now())

	_, err := provider.Query(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "lodash"}, "4.17.20")
	var providerErr *engine.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != engine.ProviderErrorInvalidResponse {
		t.Fatalf("error = %v, want invalid_response", err)
	}
}

func TestUnknownSeverity(t *testing.T) {
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"vulns":[{"id":"OSV-1","summary":"Unrated issue","affected":[{"package":{"ecosystem":"npm","name":"lodash"},"ranges":[{"type":"SEMVER","events":[{"introduced":"0"}]}]}]}]}`))
	}), time.Now())

	result, err := provider.Query(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "lodash"}, "1.0.0")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.Advisories) != 1 || result.Advisories[0].Severity != model.SeverityUnknown {
		t.Fatalf("Advisories = %+v", result.Advisories)
	}
}

func TestNumericCvssSeverity(t *testing.T) {
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"vulns":[{"id":"OSV-2","summary":"Critical issue","severity":[{"type":"CVSS_V3","score":"9.8"}],"affected":[{"package":{"ecosystem":"npm","name":"lodash"}}]}]}`))
	}), time.Now())

	result, err := provider.Query(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "lodash"}, "1.0.0")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.Advisories) != 1 || result.Advisories[0].Severity != model.SeverityCritical {
		t.Fatalf("Advisories = %+v", result.Advisories)
	}
}
