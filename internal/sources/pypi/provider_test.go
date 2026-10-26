package pypi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/fetch"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/sources/pypi"
)

const latestFixture = `{
  "info": {
    "name": "example-pkg",
    "version": "2.0.0",
    "home_page": "https://example.invalid/home",
    "project_urls": {"Source": "https://github.com/example/example-pkg"},
    "requires_dist": ["requests (>=2.0)", "urllib3>=1.21.1,<3"],
    "yanked": false
  },
  "releases": {
    "1.0.0": [{"upload_time_iso_8601": "2020-01-01T00:00:00.000000Z"}],
    "2.0.0": [{"upload_time_iso_8601": "2026-01-01T00:00:00.000000Z"}]
  },
  "urls": [{"upload_time_iso_8601": "2026-01-01T00:00:00.000000Z"}]
}`

const versionFixture = `{
  "info": {
    "name": "example-pkg",
    "version": "2.0.0",
    "project_urls": {"Repository": "https://github.com/example/example-pkg"},
    "requires_dist": [
      "requests (>=2.0)",
      "urllib3>=1.21.1,<3",
      "pytest; extra == \"dev\"",
      "weird name>=1.0"
    ],
    "yanked": true
  },
  "urls": [
    {"upload_time_iso_8601": "2026-01-01T00:00:00.123456Z"},
    {"upload_time_iso_8601": "2026-01-02T00:00:00.000000Z"}
  ]
}`

func newProvider(t *testing.T, handler http.Handler, now time.Time) *pypi.Provider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return pypi.NewProvider(pypi.Options{
		BaseURL: server.URL,
		Client: fetch.New(fetch.Config{
			CacheEnabled: true,
			CacheTTL:     time.Minute,
			Now:          func() time.Time { return now },
		}),
		Now: func() time.Time { return now },
	})
}

func TestPackageMetadataLatestVersion(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pypi/example-pkg/json" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(latestFixture))
	}), now)

	pkg := model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "example-pkg"}
	metadata, err := provider.PackageMetadata(context.Background(), pkg)
	if err != nil {
		t.Fatalf("PackageMetadata: %v", err)
	}
	if metadata.LatestVersion == nil || *metadata.LatestVersion != "2.0.0" {
		t.Fatalf("LatestVersion = %v", metadata.LatestVersion)
	}
	if len(metadata.Versions) != 2 || metadata.Versions[0] != "1.0.0" || metadata.Versions[1] != "2.0.0" {
		t.Fatalf("Versions = %v", metadata.Versions)
	}
	if metadata.LatestPublishedAt == nil || !metadata.LatestPublishedAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("LatestPublishedAt = %v", metadata.LatestPublishedAt)
	}
	if metadata.RepositoryURL == nil || *metadata.RepositoryURL != "https://github.com/example/example-pkg" {
		t.Fatalf("RepositoryURL = %v", metadata.RepositoryURL)
	}
	if metadata.Source.Provider != model.SourceProviderPyPI || metadata.Source.ID == "" {
		t.Fatalf("Source = %+v", metadata.Source)
	}
}

func TestVersionMetadataAndDependencies(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pypi/example-pkg/2.0.0/json" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(versionFixture))
	}), now)

	pkg := model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "example-pkg"}
	metadata, err := provider.VersionMetadata(context.Background(), pkg, "2.0.0")
	if err != nil {
		t.Fatalf("VersionMetadata: %v", err)
	}
	if metadata.PublishedAt == nil || !metadata.PublishedAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 123456000, time.UTC)) {
		t.Fatalf("PublishedAt = %v", metadata.PublishedAt)
	}
	if len(metadata.Dependencies) != 3 {
		t.Fatalf("Dependencies = %+v", metadata.Dependencies)
	}
	optionalFound := false
	for _, dependency := range metadata.Dependencies {
		if dependency.Package.Name == "pytest" && dependency.Optional {
			optionalFound = true
		}
		if dependency.Package.Name == "weird-name" {
			t.Fatalf("invalid dependency name should have been rejected: %+v", dependency)
		}
	}
	if !optionalFound {
		t.Fatalf("extra dependency not marked optional: %+v", metadata.Dependencies)
	}
	foundYanked := false
	for _, limitation := range metadata.Limitations {
		if limitation.Code == "YANKED_RELEASE" {
			foundYanked = true
		}
	}
	if !foundYanked {
		t.Fatalf("Limitations = %+v, want YANKED_RELEASE", metadata.Limitations)
	}
}

func TestMissingProject(t *testing.T) {
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}), time.Now())

	_, err := provider.PackageMetadata(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "missing-pkg"})
	var providerErr *engine.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != engine.ProviderErrorNotFound {
		t.Fatalf("error = %v, want not_found", err)
	}
}

func TestMissingVersion(t *testing.T) {
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}), time.Now())

	_, err := provider.VersionMetadata(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "example-pkg"}, "9.9.9")
	var providerErr *engine.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != engine.ProviderErrorNotFound {
		t.Fatalf("error = %v, want not_found", err)
	}
}

func TestMalformedProjectResponse(t *testing.T) {
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"info":`))
	}), time.Now())

	_, err := provider.PackageMetadata(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "example-pkg"})
	var providerErr *engine.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != engine.ProviderErrorInvalidResponse {
		t.Fatalf("error = %v, want invalid_response", err)
	}
}
