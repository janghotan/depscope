package npm_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/cache"
	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/fetch"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/sources/npm"
)

const registryFixture = `{
  "name": "example-pkg",
  "dist-tags": {"latest": "2.0.0"},
  "versions": {
    "2.0.0": {
      "name": "example-pkg",
      "version": "2.0.0",
      "dependencies": {"left-pad": "^1.3.0", "Bad Name": "1.0.0"},
      "optionalDependencies": {"fsevents": "^2.3.0"},
      "scripts": {"preinstall": "node pre.js", "install": "", "postinstall": "node post.js"},
      "deprecated": "use something else",
      "repository": {"type": "git", "url": "git+https://github.com/example/example-pkg.git"}
    },
    "1.0.0": {"name": "example-pkg", "version": "1.0.0"}
  },
  "time": {
    "created": "2020-01-01T00:00:00.000Z",
    "modified": "2026-01-01T00:00:00.000Z",
    "1.0.0": "2020-01-01T00:00:00.000Z",
    "2.0.0": "2026-01-01T00:00:00.000Z"
  },
  "repository": {"type": "git", "url": "git+https://github.com/example/example-pkg.git"}
}`

func newProvider(t *testing.T, handler http.Handler, now time.Time) *npm.Provider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return npm.NewProvider(npm.Options{
		BaseURL: server.URL,
		Client: fetch.New(fetch.Config{
			CacheEnabled: true,
			CacheTTL:     time.Minute,
			Now:          func() time.Time { return now },
		}),
		Now: func() time.Time { return now },
	})
}

func TestPackageMetadata(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/example-pkg" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(registryFixture))
	}), now)

	pkg, err := model.NewPackageIdentity(model.EcosystemNPM, "Example-Pkg")
	if err != nil {
		t.Fatal(err)
	}
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
	if metadata.Source.Provider != model.SourceProviderNPM || metadata.Source.ID == "" {
		t.Fatalf("Source = %+v", metadata.Source)
	}
	if !metadata.Source.RetrievedAt.Equal(now) {
		t.Fatalf("RetrievedAt = %v", metadata.Source.RetrievedAt)
	}
}

func TestPackageMetadataCachePreservesOriginalRetrievedAt(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(registryFixture))
	}))
	defer server.Close()

	client := fetch.New(fetch.Config{
		Cache:        cache.NewMemoryCache(8, func() time.Time { return now }),
		CacheEnabled: true,
		CacheTTL:     time.Minute,
		Now:          func() time.Time { return now },
	})
	provider := npm.NewProvider(npm.Options{BaseURL: server.URL, Client: client})
	pkg, err := model.NewPackageIdentity(model.EcosystemNPM, "example-pkg")
	if err != nil {
		t.Fatal(err)
	}

	first, err := provider.PackageMetadata(context.Background(), pkg)
	if err != nil {
		t.Fatalf("first PackageMetadata: %v", err)
	}
	now = now.Add(30 * time.Second)
	second, err := provider.PackageMetadata(context.Background(), pkg)
	if err != nil {
		t.Fatalf("cached PackageMetadata: %v", err)
	}
	if !second.Source.RetrievedAt.Equal(first.Source.RetrievedAt) {
		t.Fatalf("cached RetrievedAt = %v, want original fetch time %v", second.Source.RetrievedAt, first.Source.RetrievedAt)
	}
	if calls != 1 {
		t.Fatalf("registry calls = %d, want 1", calls)
	}
}

func TestVersionMetadata(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(registryFixture))
	}), now)

	pkg := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "example-pkg"}
	version, err := provider.VersionMetadata(context.Background(), pkg, "2.0.0")
	if err != nil {
		t.Fatalf("VersionMetadata: %v", err)
	}
	if version.Version != "2.0.0" {
		t.Fatalf("Version = %q", version.Version)
	}
	if version.PublishedAt == nil || !version.PublishedAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("PublishedAt = %v", version.PublishedAt)
	}
	if version.Deprecated == nil || *version.Deprecated != "use something else" {
		t.Fatalf("Deprecated = %v", version.Deprecated)
	}
	if len(version.LifecycleScripts) != 2 ||
		version.LifecycleScripts[0].Name != "preinstall" ||
		version.LifecycleScripts[1].Name != "postinstall" {
		t.Fatalf("LifecycleScripts = %+v", version.LifecycleScripts)
	}
	if version.LifecycleScripts[0].Command != "node pre.js" {
		t.Fatalf("command = %q", version.LifecycleScripts[0].Command)
	}
	if len(version.Dependencies) != 2 {
		t.Fatalf("Dependencies = %+v", version.Dependencies)
	}
	optionalFound := false
	for _, dependency := range version.Dependencies {
		if dependency.Package.Name == "fsevents" && dependency.Optional {
			optionalFound = true
		}
	}
	if !optionalFound {
		t.Fatalf("optional dependency not marked: %+v", version.Dependencies)
	}
}

func TestScopedPackagePath(t *testing.T) {
	var gotPath string
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"name":"@scope/pkg","dist-tags":{"latest":"1.0.0"},"versions":{"1.0.0":{"version":"1.0.0"}}}`))
	}), time.Now())

	metadata, err := provider.PackageMetadata(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "@scope/pkg"})
	if err != nil {
		t.Fatalf("PackageMetadata: %v", err)
	}
	if gotPath == "" || gotPath == "/@scope/pkg" {
		t.Fatalf("scoped package path = %q, want encoded slash", gotPath)
	}
	if metadata.Package.Name != "@scope/pkg" {
		t.Fatalf("Package = %+v", metadata.Package)
	}
}

func TestMissingVersion(t *testing.T) {
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(registryFixture))
	}), time.Now())

	_, err := provider.VersionMetadata(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "example-pkg"}, "9.9.9")
	var providerErr *engine.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != engine.ProviderErrorNotFound {
		t.Fatalf("error = %v, want not_found", err)
	}
}

func TestRegistryOutage(t *testing.T) {
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}), time.Now())

	_, err := provider.PackageMetadata(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "example-pkg"})
	var providerErr *engine.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != engine.ProviderErrorUnavailable {
		t.Fatalf("error = %v, want unavailable", err)
	}
}

func TestMalformedRegistryResponse(t *testing.T) {
	provider := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"versions":`))
	}), time.Now())

	_, err := provider.PackageMetadata(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "example-pkg"})
	var providerErr *engine.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Kind != engine.ProviderErrorInvalidResponse {
		t.Fatalf("error = %v, want invalid_response", err)
	}
}
