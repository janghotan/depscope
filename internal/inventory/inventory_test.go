package inventory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"depscope/internal/engine"
	"depscope/internal/model"
)

var testNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

// fakeNPMRoot 为测试提供固定的 npm 全局根目录或检测错误。
type fakeNPMRoot struct {
	root string
	err  error
}

func (f fakeNPMRoot) NPMGlobalRoot(context.Context) (string, error) {
	return f.root, f.err
}

// fakePythonLister 为测试提供固定的 pip 包列表或检测错误。
type fakePythonLister struct {
	packages []Package
	err      error
}

func (f fakePythonLister) PipPackages(context.Context) ([]Package, error) {
	return f.packages, f.err
}

type testVulnerabilities struct {
	mu      sync.Mutex
	results map[string]engine.VulnerabilityResult
	errs    map[string]error
	calls   []string
}

func (v *testVulnerabilities) Query(ctx context.Context, pkg model.PackageIdentity, version string) (engine.VulnerabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return engine.VulnerabilityResult{}, err
	}
	key := pkg.String() + "@" + version
	v.mu.Lock()
	v.calls = append(v.calls, key)
	v.mu.Unlock()
	if err, ok := v.errs[key]; ok {
		return engine.VulnerabilityResult{}, err
	}
	if result, ok := v.results[key]; ok {
		return result, nil
	}
	return engine.VulnerabilityResult{Source: testOSVSource(pkg, version)}, nil
}

func (v *testVulnerabilities) calledKeys() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	keys := append([]string(nil), v.calls...)
	sort.Strings(keys)
	return keys
}

func testOSVSource(pkg model.PackageIdentity, version string) model.SourceReference {
	return model.SourceReference{
		ID:          "osv",
		Provider:    model.SourceProviderOSV,
		Identifier:  pkg.String() + "@" + version,
		RetrievedAt: testNow,
	}
}

func vulnerabilityResult(pkg model.PackageIdentity, version string, advisories ...engine.Advisory) engine.VulnerabilityResult {
	return engine.VulnerabilityResult{Advisories: advisories, Source: testOSVSource(pkg, version)}
}

func testAdvisory(id string, severity model.Severity) engine.Advisory {
	return engine.Advisory{
		ID:             id,
		Summary:        "Synthetic advisory " + id,
		Details:        "Synthetic advisory used by deterministic tests.",
		Severity:       severity,
		References:     []string{"https://example.invalid/advisories/" + id},
		AffectedRanges: []string{"<2.0.0"},
	}
}

// testRegistry 是漏洞扫描不需要的注册表占位实现。
type testRegistry struct {
	ecosystem model.Ecosystem
}

func (r testRegistry) Ecosystem() model.Ecosystem { return r.ecosystem }

func (r testRegistry) PackageMetadata(context.Context, model.PackageIdentity) (engine.RegistryMetadata, error) {
	return engine.RegistryMetadata{}, nil
}

func (r testRegistry) Dependencies(context.Context, model.PackageIdentity, string) ([]engine.DependencyRequirement, error) {
	return nil, nil
}

func newTestScanner(t *testing.T, vulnerabilities engine.VulnerabilityProvider) *engine.Scanner {
	t.Helper()
	scanner, err := engine.NewScanner(engine.ScannerOptions{
		Config: engine.Config{
			ScanTimeout:     5 * time.Second,
			ProviderTimeout: time.Second,
			MaxConcurrency:  4,
			CacheEnabled:    false,
			CacheTTL:        0,
		},
		Registries: map[model.Ecosystem]engine.RegistryMetadataProvider{
			model.EcosystemNPM:  testRegistry{ecosystem: model.EcosystemNPM},
			model.EcosystemPyPI: testRegistry{ecosystem: model.EcosystemPyPI},
		},
		Vulnerabilities: vulnerabilities,
		Risk:            engine.RulesRiskEvaluator{},
		Clock:           func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatalf("engine.NewScanner: %v", err)
	}
	return scanner
}

func fixturePath(parts ...string) string {
	base := []string{"..", "..", "testdata", "fixtures"}
	return filepath.Join(append(base, parts...)...)
}

func npmFixtureRoot() string {
	return fixturePath("inventory", "npm", "node_modules")
}

func pythonFixturePackages(t *testing.T) []Package {
	t.Helper()
	data, err := os.ReadFile(fixturePath("inventory", "python", "pip-list.json"))
	if err != nil {
		t.Fatalf("read pip fixture: %v", err)
	}
	packages, err := parsePipList(data)
	if err != nil {
		t.Fatalf("parsePipList fixture: %v", err)
	}
	return packages
}

func installedKeys(result Result) []string {
	keys := make([]string, 0, len(result.Packages))
	for _, item := range result.Packages {
		keys = append(keys, item.Package.String()+"@"+item.Version)
	}
	sort.Strings(keys)
	return keys
}

func checkByID(t *testing.T, result Result, id string) model.CheckResult {
	t.Helper()
	for _, check := range result.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("check %q not found in %v", id, checkIDs(result.Checks))
	return model.CheckResult{}
}

func checkIDs(checks []model.CheckResult) []string {
	ids := make([]string, 0, len(checks))
	for _, check := range checks {
		ids = append(ids, check.ID)
	}
	return ids
}

func hasLimitation(result Result, code string, messagePart string) bool {
	for _, limitation := range result.Limitations {
		if limitation.Code != code {
			continue
		}
		if messagePart == "" || strings.Contains(limitation.Message, messagePart) {
			return true
		}
	}
	return false
}

func resultSourceExists(result Result, id string) bool {
	for _, source := range result.Sources {
		if source.ID == id {
			return true
		}
	}
	return false
}

// writeInstalledPackage 创建一个最小的全局 npm 包元数据文件。
func writeInstalledPackage(t *testing.T, root, rel, name, version string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := `{"name":"` + name + `","version":"` + version + `"}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write package metadata: %v", err)
	}
}

func scanInventory(t *testing.T, opts Options) Result {
	t.Helper()
	if opts.Clock == nil {
		opts.Clock = func() time.Time { return testNow }
	}
	result, err := Scan(context.Background(), opts)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !result.ScannedAt.Equal(testNow) {
		t.Fatalf("ScannedAt = %v, want %v", result.ScannedAt, testNow)
	}
	return result
}

func TestScanInventoriesNPMAndPython(t *testing.T) {
	t.Parallel()

	npmVulnerable, err := model.NewPackageIdentity(model.EcosystemNPM, "vulnerable-pkg")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	pypiVulnerable, err := model.NewPackageIdentity(model.EcosystemPyPI, "vulnerable-pkg")
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	vulnerabilities := &testVulnerabilities{
		results: map[string]engine.VulnerabilityResult{
			"npm:vulnerable-pkg@2.3.4":  vulnerabilityResult(npmVulnerable, "2.3.4", testAdvisory("DEMO-NPM-1", model.SeverityHigh)),
			"pypi:vulnerable-pkg@2.3.4": vulnerabilityResult(pypiVulnerable, "2.3.4", testAdvisory("DEMO-PYPI-1", model.SeverityCritical)),
		},
	}
	var stages []Stage
	result := scanInventory(t, Options{
		Scanner: newTestScanner(t, vulnerabilities),
		NPM:     fakeNPMRoot{root: npmFixtureRoot()},
		Python:  fakePythonLister{packages: pythonFixturePackages(t)},
		Progress: func(event Progress) {
			stages = append(stages, event.Stage)
		},
	})

	wantPackages := []string{
		"npm:@scope/scoped-pkg@0.5.0",
		"npm:safe-pkg@1.0.0",
		"npm:vulnerable-pkg@2.3.4",
		"pypi:pip@24.0",
		"pypi:safe-pkg@1.0.0",
		"pypi:vulnerable-pkg@2.3.4",
	}
	if got := installedKeys(result); !reflect.DeepEqual(got, wantPackages) {
		t.Fatalf("packages = %v, want %v", got, wantPackages)
	}
	for _, item := range result.Packages {
		if strings.Contains(item.Package.Name, "hidden-pkg") {
			t.Fatalf("nested package %s must not be inventoried", item.Package.String())
		}
		if item.Package.Ecosystem == model.EcosystemNPM && item.SourcePath == "" {
			t.Fatalf("npm package %s is missing its metadata path", item.Package.String())
		}
	}

	if result.Status != model.StatusFindings {
		t.Fatalf("status = %q, want %q", result.Status, model.StatusFindings)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(result.Findings))
	}
	if result.Findings[0].Package != pypiVulnerable || result.Findings[1].Package != npmVulnerable {
		t.Fatalf("findings order = %v, want pypi critical before npm high", findingKeys(result.Findings))
	}
	if result.Summary.ConfirmedFindings != 2 || result.Summary.HighestSeverity == nil || *result.Summary.HighestSeverity != model.SeverityCritical {
		t.Fatalf("summary = %+v, want two confirmed findings with critical severity", result.Summary)
	}

	if check := checkByID(t, result, "npm:global-detection"); check.Status != model.CheckSucceeded {
		t.Fatalf("npm detection = %+v, want succeeded", check)
	}
	if check := checkByID(t, result, "python:global-detection"); check.Status != model.CheckSucceeded {
		t.Fatalf("python detection = %+v, want succeeded", check)
	}
	if check := checkByID(t, result, "npm:vulnerabilities"); check.Status != model.CheckSucceeded {
		t.Fatalf("npm vulnerabilities = %+v, want succeeded", check)
	}
	if check := checkByID(t, result, "python:vulnerabilities"); check.Status != model.CheckSucceeded {
		t.Fatalf("python vulnerabilities = %+v, want succeeded", check)
	}
	if check := checkByID(t, result, "npm:risk-evaluation"); check.Status != model.CheckSucceeded {
		t.Fatalf("npm risk = %+v, want succeeded", check)
	}
	if check := checkByID(t, result, "python:risk-evaluation"); check.Status != model.CheckSucceeded {
		t.Fatalf("python risk = %+v, want succeeded", check)
	}

	if !resultSourceExists(result, "inventory:npm") || !resultSourceExists(result, "inventory:pypi") {
		t.Fatalf("sources = %v, want inventory:npm and inventory:pypi", sourceIDs(result.Sources))
	}
	if !resultSourceExists(result, "osv:npm:vulnerable-pkg@2.3.4") || !resultSourceExists(result, "osv:pypi:vulnerable-pkg@2.3.4") {
		t.Fatalf("sources = %v, want per-package OSV provenance", sourceIDs(result.Sources))
	}
	for _, finding := range result.Findings {
		registryEvidence := 0
		for _, evidence := range finding.Evidence {
			if !resultSourceExists(result, evidence.SourceID) {
				t.Fatalf("finding %q references unknown source %q", finding.ID, evidence.SourceID)
			}
			if evidence.Kind == model.EvidenceRegistry {
				registryEvidence++
			}
		}
		if registryEvidence == 0 {
			t.Fatalf("finding %q has no installed-metadata evidence", finding.ID)
		}
	}

	if !hasLimitation(result, "NPM_GLOBAL_TOP_LEVEL_ONLY", "") {
		t.Fatalf("limitations = %+v, want NPM_GLOBAL_TOP_LEVEL_ONLY", result.Limitations)
	}
	if !hasLimitation(result, "PYTHON_SINGLE_ENVIRONMENT", "") {
		t.Fatalf("limitations = %+v, want PYTHON_SINGLE_ENVIRONMENT", result.Limitations)
	}

	wantCalls := []string{
		"npm:@scope/scoped-pkg@0.5.0",
		"npm:safe-pkg@1.0.0",
		"npm:vulnerable-pkg@2.3.4",
		"pypi:pip@24.0",
		"pypi:safe-pkg@1.0.0",
		"pypi:vulnerable-pkg@2.3.4",
	}
	if got := vulnerabilities.calledKeys(); !reflect.DeepEqual(got, wantCalls) {
		t.Fatalf("OSV queries = %v, want %v", got, wantCalls)
	}

	wantStages := []Stage{StageList, StageScan, StageScan, StageAssemble}
	if !reflect.DeepEqual(stages, wantStages) {
		t.Fatalf("progress stages = %v, want %v", stages, wantStages)
	}

	repeat := scanInventory(t, Options{
		Scanner: newTestScanner(t, vulnerabilities),
		NPM:     fakeNPMRoot{root: npmFixtureRoot()},
		Python:  fakePythonLister{packages: pythonFixturePackages(t)},
	})
	if !reflect.DeepEqual(result, repeat) {
		t.Fatal("inventory result is not deterministic across runs")
	}
}

func TestScanExplainsPartialDetectionFailure(t *testing.T) {
	t.Parallel()

	vulnerabilities := &testVulnerabilities{}
	result := scanInventory(t, Options{
		Scanner: newTestScanner(t, vulnerabilities),
		NPM:     fakeNPMRoot{err: errors.New("npm executable not found")},
		Python:  fakePythonLister{packages: pythonFixturePackages(t)},
	})

	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want %q", result.Status, model.StatusIncomplete)
	}
	check := checkByID(t, result, "npm:global-detection")
	if check.Status != model.CheckUnavailable || !strings.Contains(check.Message, "npm") {
		t.Fatalf("npm detection = %+v, want unavailable with npm explanation", check)
	}
	if check := checkByID(t, result, "python:global-detection"); check.Status != model.CheckSucceeded {
		t.Fatalf("python detection = %+v, want succeeded", check)
	}
	if !hasLimitation(result, "NPM_GLOBAL_DETECTION_FAILED", "npm executable not found") {
		t.Fatalf("limitations = %+v, want failure explanation", result.Limitations)
	}
	for _, key := range vulnerabilities.calledKeys() {
		if strings.HasPrefix(key, "npm:") {
			t.Fatalf("OSV was queried for npm package %q despite npm detection failure", key)
		}
	}
}

func TestScanFailsWhenBothManagersFailDetection(t *testing.T) {
	t.Parallel()

	result := scanInventory(t, Options{
		Scanner: newTestScanner(t, &testVulnerabilities{}),
		NPM:     fakeNPMRoot{err: errors.New("npm root failed")},
		Python:  fakePythonLister{err: errors.New("python failed")},
	})

	if result.Status != model.StatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, model.StatusFailed)
	}
	if len(result.Packages) != 0 || len(result.Findings) != 0 {
		t.Fatalf("packages = %d, findings = %d, want none", len(result.Packages), len(result.Findings))
	}
	if !hasLimitation(result, "NPM_GLOBAL_DETECTION_FAILED", "") || !hasLimitation(result, "PYTHON_GLOBAL_DETECTION_FAILED", "") {
		t.Fatalf("limitations = %+v, want both detection failures", result.Limitations)
	}
}

func TestScanExplainsMissingManager(t *testing.T) {
	t.Parallel()

	result := scanInventory(t, Options{
		Scanner: newTestScanner(t, &testVulnerabilities{}),
		NPM:     fakeNPMRoot{root: npmFixtureRoot()},
	})

	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want %q", result.Status, model.StatusIncomplete)
	}
	check := checkByID(t, result, "python:global-detection")
	if check.Status != model.CheckUnavailable {
		t.Fatalf("python detection = %+v, want unavailable", check)
	}
	if !hasLimitation(result, "PYTHON_NOT_CONFIGURED", "") {
		t.Fatalf("limitations = %+v, want PYTHON_NOT_CONFIGURED", result.Limitations)
	}
}

func TestScanReportsMetadataProblems(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeInstalledPackage(t, root, "good-pkg", "good-pkg", "1.0.0")
	writeInstalledPackage(t, root, "good-pkg/node_modules/hidden-pkg", "hidden-pkg", "9.9.9")
	if err := os.MkdirAll(filepath.Join(root, "missing-meta"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeInstalledPackage(t, root, "malformed", "", "")
	if err := os.WriteFile(filepath.Join(root, "malformed", "package.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("write malformed: %v", err)
	}
	writeInstalledPackage(t, root, "bad-name", "Bad Name", "1.0.0")
	writeInstalledPackage(t, root, "no-version", "no-version", "")
	oversized := filepath.Join(root, "oversized")
	if err := os.MkdirAll(oversized, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	padding := strings.Repeat("x", 2048)
	if err := os.WriteFile(filepath.Join(oversized, "package.json"), []byte(`{"name":"oversized","version":"1.0.0","description":"`+padding+`"}`), 0o644); err != nil {
		t.Fatalf("write oversized: %v", err)
	}
	symlinkCreated := true
	if err := os.Symlink(filepath.Join(root, "good-pkg"), filepath.Join(root, "linked-pkg")); err != nil {
		symlinkCreated = false
		t.Logf("symlink unavailable: %v", err)
	}

	pythonPackages := []Package{
		{Name: "Safe_Pkg", Version: "1.0.0"},
		{Name: "bad name", Version: "1.0.0"},
		{Name: "missing-version", Version: ""},
		{Name: "dup", Version: "1.0.0"},
		{Name: "DUP", Version: "2.0.0"},
	}
	result := scanInventory(t, Options{
		Scanner: newTestScanner(t, &testVulnerabilities{}),
		Limits:  Limits{MaxPackages: 100, MaxFileSize: 512},
		NPM:     fakeNPMRoot{root: root},
		Python:  fakePythonLister{packages: pythonPackages},
	})

	wantPackages := []string{"npm:good-pkg@1.0.0", "pypi:safe-pkg@1.0.0"}
	if got := installedKeys(result); !reflect.DeepEqual(got, wantPackages) {
		t.Fatalf("packages = %v, want %v", got, wantPackages)
	}
	if check := checkByID(t, result, "npm:global-detection"); check.Status != model.CheckUnavailable {
		t.Fatalf("npm detection = %+v, want unavailable", check)
	}
	if check := checkByID(t, result, "python:global-detection"); check.Status != model.CheckUnavailable {
		t.Fatalf("python detection = %+v, want unavailable", check)
	}
	for _, code := range []string{
		"PACKAGE_METADATA_MISSING",
		"MALFORMED_PACKAGE_METADATA",
		"PACKAGE_METADATA_TOO_LARGE",
		"INVALID_PACKAGE_METADATA",
		"UNRESOLVED_VERSION",
		"CONFLICTING_PACKAGE_METADATA",
	} {
		if !hasLimitation(result, code, "") {
			t.Fatalf("limitations = %+v, want %s", result.Limitations, code)
		}
	}
	if symlinkCreated && !hasLimitation(result, "SYMLINKED_PACKAGE_SKIPPED", "") {
		t.Fatalf("limitations = %+v, want SYMLINKED_PACKAGE_SKIPPED", result.Limitations)
	}
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want %q", result.Status, model.StatusIncomplete)
	}
}

func TestScanHonorsPackageLimit(t *testing.T) {
	t.Parallel()

	result := scanInventory(t, Options{
		Scanner: newTestScanner(t, &testVulnerabilities{}),
		Limits:  Limits{MaxPackages: 1, MaxFileSize: 1 << 20},
		NPM:     fakeNPMRoot{root: npmFixtureRoot()},
	})

	wantPackages := []string{"npm:@scope/scoped-pkg@0.5.0"}
	if got := installedKeys(result); !reflect.DeepEqual(got, wantPackages) {
		t.Fatalf("packages = %v, want %v", got, wantPackages)
	}
	if !hasLimitation(result, "PACKAGE_LIMIT_EXCEEDED", "") {
		t.Fatalf("limitations = %+v, want PACKAGE_LIMIT_EXCEEDED", result.Limitations)
	}
	if check := checkByID(t, result, "npm:global-detection"); check.Status != model.CheckUnavailable {
		t.Fatalf("npm detection = %+v, want unavailable", check)
	}
}

func TestScanKeepsFindingsWhenVulnerabilityLookupFails(t *testing.T) {
	t.Parallel()

	vulnerabilities := &testVulnerabilities{
		errs: map[string]error{
			"npm:safe-pkg@1.0.0": errors.New("OSV request failed"),
		},
	}
	result := scanInventory(t, Options{
		Scanner: newTestScanner(t, vulnerabilities),
		NPM:     fakeNPMRoot{root: npmFixtureRoot()},
		Python:  fakePythonLister{packages: pythonFixturePackages(t)},
	})

	check := checkByID(t, result, "npm:vulnerabilities")
	if check.Status != model.CheckUnavailable {
		t.Fatalf("npm vulnerabilities = %+v, want unavailable", check)
	}
	if !hasLimitation(result, "VULNERABILITY_DATA_UNAVAILABLE", "") {
		t.Fatalf("limitations = %+v, want VULNERABILITY_DATA_UNAVAILABLE", result.Limitations)
	}
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want %q", result.Status, model.StatusIncomplete)
	}
}

func TestScanRequiresScanner(t *testing.T) {
	t.Parallel()

	_, err := Scan(context.Background(), Options{
		NPM:    fakeNPMRoot{root: npmFixtureRoot()},
		Python: fakePythonLister{packages: []Package{}},
	})
	if err == nil || !strings.Contains(err.Error(), "scanner") {
		t.Fatalf("Scan error = %v, want scanner requirement", err)
	}
}

func TestScanReportsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := Scan(ctx, Options{
		Scanner: newTestScanner(t, &testVulnerabilities{}),
		NPM:     fakeNPMRoot{root: npmFixtureRoot()},
		Python:  fakePythonLister{packages: pythonFixturePackages(t)},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Scan error = %v, want context.Canceled", err)
	}
	if result.Status != model.StatusCancelled {
		t.Fatalf("status = %q, want %q", result.Status, model.StatusCancelled)
	}
}

func findingKeys(findings []model.Finding) []string {
	keys := make([]string, 0, len(findings))
	for _, finding := range findings {
		version := ""
		if finding.Version != nil {
			version = *finding.Version
		}
		keys = append(keys, finding.ID+" "+finding.Package.String()+"@"+version)
	}
	return keys
}

func sourceIDs(sources []model.SourceReference) []string {
	ids := make([]string, 0, len(sources))
	for _, source := range sources {
		ids = append(ids, source.ID)
	}
	return ids
}
