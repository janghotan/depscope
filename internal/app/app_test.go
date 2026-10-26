package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"depscope/internal/engine"
	"depscope/internal/inventory"
	"depscope/internal/model"
	"depscope/internal/project"
)

var testTime = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

func TestParsePackageSpec(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		ecosystem model.Ecosystem
		want      PackageRequest
		wantError bool
	}{
		{name: "npm version", input: "lodash@4.17.21", ecosystem: model.EcosystemNPM, want: PackageRequest{Ecosystem: model.EcosystemNPM, Name: "lodash", Version: "4.17.21"}},
		{name: "npm latest", input: "lodash", ecosystem: model.EcosystemNPM, want: PackageRequest{Ecosystem: model.EcosystemNPM, Name: "lodash"}},
		{name: "scoped latest", input: "@scope/pkg", ecosystem: model.EcosystemNPM, want: PackageRequest{Ecosystem: model.EcosystemNPM, Name: "@scope/pkg"}},
		{name: "scoped version", input: "@scope/pkg@1.2.3", ecosystem: model.EcosystemNPM, want: PackageRequest{Ecosystem: model.EcosystemNPM, Name: "@scope/pkg", Version: "1.2.3"}},
		{name: "npm prefix", input: "npm:@scope/pkg@1.2.3", ecosystem: model.EcosystemPyPI, want: PackageRequest{Ecosystem: model.EcosystemNPM, Name: "@scope/pkg", Version: "1.2.3"}},
		{name: "pypi prefix", input: "pypi:requests==2.31.0", ecosystem: model.EcosystemNPM, want: PackageRequest{Ecosystem: model.EcosystemPyPI, Name: "requests", Version: "2.31.0"}},
		{name: "pypi triple equals", input: "requests===2.31.0", ecosystem: model.EcosystemPyPI, want: PackageRequest{Ecosystem: model.EcosystemPyPI, Name: "requests", Version: "2.31.0"}},
		{name: "pypi canonical", input: "pypi:Safe_Pkg", ecosystem: model.EcosystemNPM, want: PackageRequest{Ecosystem: model.EcosystemPyPI, Name: "safe-pkg"}},
		{name: "empty", input: "", wantError: true},
		{name: "empty prefix", input: "npm:", wantError: true},
		{name: "empty version", input: "npm:pkg@", wantError: true},
		{name: "pypi empty version", input: "pypi:requests==", wantError: true},
		{name: "invalid characters", input: "npm:bad name", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := ParsePackageSpec(test.input, test.ecosystem)
			if test.wantError {
				if err == nil {
					t.Fatalf("expected error, got %+v", request)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !reflect.DeepEqual(request, test.want) {
				t.Fatalf("request = %+v, want %+v", request, test.want)
			}
		})
	}
}

func TestScanPackageUsesLiveModeAndReportsProgress(t *testing.T) {
	scanner := buildTestScanner(t, testVulnerabilities{})
	service := &LiveService{scanner: scanner, clock: func() time.Time { return testTime }}
	request, err := NewPackageRequest(model.EcosystemNPM, "@fixture/known-package", "")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	progress := []Progress{}
	report, err := service.ScanPackage(context.Background(), request, func(item Progress) {
		progress = append(progress, item)
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if report.DataMode != model.DataModeLive {
		t.Fatalf("dataMode = %q, want live", report.DataMode)
	}
	if report.Summary.ConfirmedFindings != 1 {
		t.Fatalf("confirmed findings = %d, want 1", report.Summary.ConfirmedFindings)
	}
	if len(progress) < 2 || progress[0].Stage != "scan" {
		t.Fatalf("progress = %+v", progress)
	}
}

func TestScanPackageCancellation(t *testing.T) {
	scanner := buildBlockingScanner(t)
	service := &LiveService{scanner: scanner, clock: func() time.Time { return testTime }}
	request, err := NewPackageRequest(model.EcosystemNPM, "@fixture/known-package", "")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := service.ScanPackage(ctx, request, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if report.Status != model.StatusCancelled {
		t.Fatalf("status = %q, want cancelled", report.Status)
	}
}

func TestComparePreservesBothSides(t *testing.T) {
	scanner := buildTestScanner(t, testVulnerabilities{})
	service := &LiveService{scanner: scanner, clock: func() time.Time { return testTime }}
	left, _ := NewPackageRequest(model.EcosystemNPM, "@fixture/known-package", "")
	right, _ := NewPackageRequest(model.EcosystemNPM, "@fixture/missing-package", "")
	result, err := service.Compare(context.Background(), CompareRequest{Left: left, Right: right}, nil)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if result.Left == nil || result.Left.Summary.ConfirmedFindings != 1 {
		t.Fatalf("left report = %+v", result.Left)
	}
	if result.Right == nil {
		t.Fatal("right report was discarded")
	}
	if result.Right.Status != model.StatusFailed {
		t.Fatalf("right status = %q, want failed", result.Right.Status)
	}
}

func TestDemoIsDeterministicAndOffline(t *testing.T) {
	service := &LiveService{}
	first, err := service.Demo(context.Background(), "clean", nil)
	if err != nil {
		t.Fatalf("demo: %v", err)
	}
	second, err := service.Demo(context.Background(), "clean", nil)
	if err != nil {
		t.Fatalf("demo: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("demo reports are not deterministic")
	}
	if first.DataMode != model.DataModeDemo {
		t.Fatalf("dataMode = %q, want demo", first.DataMode)
	}
	scenarios := service.DemoScenarios()
	if len(scenarios) != 3 {
		t.Fatalf("scenarios = %d, want 3", len(scenarios))
	}
	if scenarios[0].ID == "" || scenarios[0].Title == "" || scenarios[0].Description == "" {
		t.Fatalf("scenario metadata is incomplete: %+v", scenarios[0])
	}
	defaultReport, err := service.Demo(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("default demo: %v", err)
	}
	if defaultReport.Target.Package.Name != "@depscope-demo/vulnerable-package" {
		t.Fatalf("default scenario target = %s", defaultReport.Target.Package)
	}
}

func TestScanProjectWiring(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"fixture-project","version":"1.0.0"}`)
	writeFile(t, dir, "package-lock.json", `{"lockfileVersion":3,"packages":{"":{"name":"fixture-project","version":"1.0.0"}}}`)
	scanner := buildTestScanner(t, testVulnerabilities{})
	service := &LiveService{
		scanner:       scanner,
		clock:         func() time.Time { return testTime },
		projectLimits: projectLimitsForTest(),
	}
	progress := []Progress{}
	result, err := service.ScanProject(context.Background(), ProjectRequest{Path: dir}, func(item Progress) {
		progress = append(progress, item)
	})
	if err != nil {
		t.Fatalf("project scan: %v", err)
	}
	if result.Root != dir {
		t.Fatalf("root = %q, want %q", result.Root, dir)
	}
	if len(progress) == 0 {
		t.Fatal("project scan emitted no progress")
	}
}

func TestScanInventoryWiring(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "fixture-global"), "package.json", `{"name":"@fixture/global-package","version":"1.0.0"}`)
	scanner := buildTestScanner(t, testVulnerabilities{})
	service := &LiveService{
		scanner: scanner,
		clock:   func() time.Time { return testTime },
		npm:     fakeNPMRoot{root: root},
		python:  fakePython{packages: []inventory.Package{{Name: "fixture-package", Version: "2.0.0"}}},
	}
	progress := []Progress{}
	result, err := service.ScanInventory(context.Background(), func(item Progress) {
		progress = append(progress, item)
	})
	if err != nil {
		t.Fatalf("inventory scan: %v", err)
	}
	if len(result.Packages) != 2 {
		t.Fatalf("packages = %d, want 2", len(result.Packages))
	}
	if len(progress) == 0 {
		t.Fatal("inventory scan emitted no progress")
	}
}

func buildTestScanner(t *testing.T, vuln testVulnerabilities) *engine.Scanner {
	t.Helper()
	registry := &testRegistry{
		ecosystem: model.EcosystemNPM,
		metadata: map[string]engine.RegistryMetadata{
			"npm:@fixture/known-package":  registryMetadata(t, "@fixture/known-package"),
			"npm:@fixture/global-package": registryMetadata(t, "@fixture/global-package"),
		},
		versions: map[string]engine.VersionMetadata{
			"npm:@fixture/known-package@1.0.0":  versionMetadata(t, "@fixture/known-package"),
			"npm:@fixture/global-package@1.0.0": versionMetadata(t, "@fixture/global-package"),
		},
	}
	scanner, err := engine.NewScanner(engine.ScannerOptions{
		Config:          engine.DefaultConfig(),
		Registries:      map[model.Ecosystem]engine.RegistryMetadataProvider{model.EcosystemNPM: registry},
		Vulnerabilities: vuln,
		Clock:           func() time.Time { return testTime },
	})
	if err != nil {
		t.Fatalf("build scanner: %v", err)
	}
	return scanner
}

func buildBlockingScanner(t *testing.T) *engine.Scanner {
	t.Helper()
	registry := &blockingRegistry{}
	scanner, err := engine.NewScanner(engine.ScannerOptions{
		Config:          engine.DefaultConfig(),
		Registries:      map[model.Ecosystem]engine.RegistryMetadataProvider{model.EcosystemNPM: registry},
		Vulnerabilities: testVulnerabilities{},
		Clock:           func() time.Time { return testTime },
	})
	if err != nil {
		t.Fatalf("build blocking scanner: %v", err)
	}
	return scanner
}

type testRegistry struct {
	ecosystem model.Ecosystem
	metadata  map[string]engine.RegistryMetadata
	versions  map[string]engine.VersionMetadata
}

func (r *testRegistry) Ecosystem() model.Ecosystem { return r.ecosystem }

func (r *testRegistry) PackageMetadata(_ context.Context, pkg model.PackageIdentity) (engine.RegistryMetadata, error) {
	metadata, ok := r.metadata[pkg.String()]
	if !ok {
		return engine.RegistryMetadata{}, &engine.ProviderError{Provider: "fixture", Kind: engine.ProviderErrorNotFound, Err: fmt.Errorf("package %s not found", pkg)}
	}
	return metadata, nil
}

func (r *testRegistry) Dependencies(context.Context, model.PackageIdentity, string) ([]engine.DependencyRequirement, error) {
	return []engine.DependencyRequirement{}, nil
}

func (r *testRegistry) VersionMetadata(_ context.Context, pkg model.PackageIdentity, version string) (engine.VersionMetadata, error) {
	metadata, ok := r.versions[pkg.String()+"@"+version]
	if !ok {
		return engine.VersionMetadata{}, &engine.ProviderError{Provider: "fixture", Kind: engine.ProviderErrorNotFound, Err: fmt.Errorf("version %s@%s not found", pkg, version)}
	}
	return metadata, nil
}

type blockingRegistry struct{}

func (b *blockingRegistry) Ecosystem() model.Ecosystem { return model.EcosystemNPM }

func (b *blockingRegistry) PackageMetadata(ctx context.Context, _ model.PackageIdentity) (engine.RegistryMetadata, error) {
	<-ctx.Done()
	return engine.RegistryMetadata{}, ctx.Err()
}

func (b *blockingRegistry) Dependencies(context.Context, model.PackageIdentity, string) ([]engine.DependencyRequirement, error) {
	return []engine.DependencyRequirement{}, nil
}

type testVulnerabilities struct{}

func (testVulnerabilities) Query(ctx context.Context, pkg model.PackageIdentity, version string) (engine.VulnerabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return engine.VulnerabilityResult{}, err
	}
	result := engine.VulnerabilityResult{
		Source: model.SourceReference{
			ID:          "fixture:osv",
			Provider:    model.SourceProviderOSV,
			Identifier:  "fixture-osv",
			RetrievedAt: testTime,
		},
		Limitations: []model.Limitation{},
	}
	if pkg.Name == "@fixture/known-package" {
		result.Advisories = []engine.Advisory{{
			ID:            "FIXTURE-2026-0001",
			Summary:       "Fixture advisory",
			Severity:      model.SeverityHigh,
			FixedVersions: []string{"1.0.1"},
		}}
	}
	return result, nil
}

type fakeNPMRoot struct {
	root string
}

func (f fakeNPMRoot) NPMGlobalRoot(context.Context) (string, error) { return f.root, nil }

type fakePython struct {
	packages []inventory.Package
}

func (f fakePython) PipPackages(context.Context) ([]inventory.Package, error) {
	return f.packages, nil
}

func registryMetadata(t *testing.T, name string) engine.RegistryMetadata {
	t.Helper()
	identity, err := model.NewPackageIdentity(model.EcosystemNPM, name)
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	latest := "1.0.0"
	published := testTime
	return engine.RegistryMetadata{
		Package:           identity,
		Versions:          []string{"1.0.0"},
		LatestVersion:     &latest,
		LatestPublishedAt: &published,
		Source: model.SourceReference{
			ID:          "fixture:npm",
			Provider:    model.SourceProviderNPM,
			Identifier:  "fixture-registry",
			RetrievedAt: testTime,
		},
		Limitations: []model.Limitation{},
	}
}

func versionMetadata(t *testing.T, name string) engine.VersionMetadata {
	t.Helper()
	identity, err := model.NewPackageIdentity(model.EcosystemNPM, name)
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	published := testTime
	return engine.VersionMetadata{
		Package:          identity,
		Version:          "1.0.0",
		PublishedAt:      &published,
		LifecycleScripts: []engine.LifecycleScript{},
		Dependencies:     []engine.DependencyRequirement{},
		Source: model.SourceReference{
			ID:          "fixture:npm",
			Provider:    model.SourceProviderNPM,
			Identifier:  "fixture-registry",
			RetrievedAt: testTime,
		},
		Limitations: []model.Limitation{},
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func projectLimitsForTest() (limits project.Limits) {
	return project.DefaultLimits()
}
