package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/model"
)

var testNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

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
	return append([]string(nil), v.calls...)
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

func projectFixture(name string) string {
	return fixturePath("projects", name)
}

func scanFixture(t *testing.T, name string, vulnerabilities engine.VulnerabilityProvider) Result {
	t.Helper()
	return scanDirectory(t, projectFixture(name), vulnerabilities, Options{})
}

func scanDirectory(t *testing.T, dir string, vulnerabilities engine.VulnerabilityProvider, opts Options) Result {
	t.Helper()
	if opts.Scanner == nil {
		opts.Scanner = newTestScanner(t, vulnerabilities)
	}
	if opts.Clock == nil {
		opts.Clock = func() time.Time { return testNow }
	}
	result, err := Scan(context.Background(), dir, opts)
	if err != nil {
		t.Fatalf("Scan(%s): %v", dir, err)
	}
	if !result.ScannedAt.Equal(testNow) {
		t.Fatalf("ScannedAt = %v, want %v", result.ScannedAt, testNow)
	}
	return result
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

func hasDetectionLimitation(detection Detection, code string) bool {
	for _, limitation := range detection.Limitations {
		if limitation.Code == code {
			return true
		}
	}
	return false
}

func nodeByID(t *testing.T, result Result, id string) Node {
	t.Helper()
	for _, node := range result.Dependencies.Nodes {
		if node.ID == id {
			return node
		}
	}
	ids := make([]string, 0, len(result.Dependencies.Nodes))
	for _, node := range result.Dependencies.Nodes {
		ids = append(ids, node.ID)
	}
	t.Fatalf("node %q not found in %v", id, ids)
	return Node{}
}

func findingsFor(result Result, pkg string, version string) []model.Finding {
	var findings []model.Finding
	for _, finding := range result.Findings {
		if finding.Package.String() != pkg {
			continue
		}
		if version != "" && (finding.Version == nil || *finding.Version != version) {
			continue
		}
		findings = append(findings, finding)
	}
	return findings
}

func evidenceSummary(finding model.Finding, kind model.EvidenceKind) string {
	for _, evidence := range finding.Evidence {
		if evidence.Kind == kind {
			return evidence.Summary
		}
	}
	return ""
}

func assertEvidenceSourcesReferenced(t *testing.T, result Result) {
	t.Helper()
	known := make(map[string]struct{}, len(result.Sources))
	for _, source := range result.Sources {
		known[source.ID] = struct{}{}
	}
	for _, finding := range result.Findings {
		for _, evidence := range finding.Evidence {
			if _, ok := known[evidence.SourceID]; !ok {
				t.Fatalf("finding %q references unknown source %q", finding.ID, evidence.SourceID)
			}
		}
	}
	for _, check := range result.Checks {
		for _, sourceID := range check.SourceIDs {
			if _, ok := known[sourceID]; !ok {
				t.Fatalf("check %q references unknown source %q", check.ID, sourceID)
			}
		}
	}
}

func writeTestFile(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestDetectSupportedProjects(t *testing.T) {
	tests := []struct {
		name        string
		ecosystem   model.Ecosystem
		format      Format
		manifest    string
		lockfile    string
		wantMissing bool
	}{
		{
			name:      "npm-healthy",
			ecosystem: model.EcosystemNPM,
			format:    FormatNPMPackageLock,
			manifest:  "package.json",
			lockfile:  "package-lock.json",
		},
		{
			name:        "npm-missing-lock",
			ecosystem:   model.EcosystemNPM,
			format:      FormatNPMPackageLock,
			manifest:    "package.json",
			wantMissing: true,
		},
		{
			name:      "python-pinned",
			ecosystem: model.EcosystemPyPI,
			format:    FormatPythonRequirements,
			manifest:  "requirements.txt",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detection, err := Detect(projectFixture(test.name), DefaultLimits())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if len(detection.Candidates) != 1 {
				t.Fatalf("candidates = %+v, want exactly one", detection.Candidates)
			}
			candidate := detection.Candidates[0]
			if candidate.Ecosystem != test.ecosystem {
				t.Fatalf("ecosystem = %q, want %q", candidate.Ecosystem, test.ecosystem)
			}
			if candidate.Format != test.format {
				t.Fatalf("format = %q, want %q", candidate.Format, test.format)
			}
			if candidate.ManifestPath != test.manifest {
				t.Fatalf("manifest = %q, want %q", candidate.ManifestPath, test.manifest)
			}
			if candidate.LockfilePath != test.lockfile {
				t.Fatalf("lockfile = %q, want %q", candidate.LockfilePath, test.lockfile)
			}
			if test.wantMissing {
				if !hasDetectionLimitation(detection, "LOCKFILE_MISSING") {
					t.Fatalf("missing LOCKFILE_MISSING limitation: %+v", detection.Limitations)
				}
			}
			if len(detection.Unsupported) != 0 {
				t.Fatalf("unsupported = %v, want none", detection.Unsupported)
			}
		})
	}
}

func TestDetectReportsUnsupportedFormats(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"name":"demo-app","dependencies":{}}`)
	writeTestFile(t, dir, "yarn.lock", "# synthetic yarn lockfile\n")

	detection, err := Detect(dir, DefaultLimits())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(detection.Unsupported) != 1 || detection.Unsupported[0] != "yarn.lock" {
		t.Fatalf("unsupported = %v, want [yarn.lock]", detection.Unsupported)
	}
	if !hasDetectionLimitation(detection, "UNSUPPORTED_FORMAT") {
		t.Fatalf("missing UNSUPPORTED_FORMAT limitation: %+v", detection.Limitations)
	}
}

func TestDetectLockfileWithoutManifest(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package-lock.json", `{"lockfileVersion":3,"packages":{}}`)

	detection, err := Detect(dir, DefaultLimits())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(detection.Candidates) != 0 {
		t.Fatalf("candidates = %+v, want none", detection.Candidates)
	}
	if !hasDetectionLimitation(detection, "MANIFEST_MISSING") {
		t.Fatalf("missing MANIFEST_MISSING limitation: %+v", detection.Limitations)
	}
}

func TestDetectNoSupportedManifest(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "README.md", "nothing to scan\n")

	detection, err := Detect(dir, DefaultLimits())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(detection.Candidates) != 0 {
		t.Fatalf("candidates = %+v, want none", detection.Candidates)
	}
	if !hasDetectionLimitation(detection, "NO_SUPPORTED_MANIFEST") {
		t.Fatalf("missing NO_SUPPORTED_MANIFEST limitation: %+v", detection.Limitations)
	}
}

func TestDetectOnlyInspectsProjectRoot(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, filepath.Join("sub", "package.json"), `{"name":"nested","dependencies":{}}`)
	writeTestFile(t, dir, filepath.Join("node_modules", "pkg", "package.json"), `{"name":"pkg","version":"1.0.0"}`)
	writeTestFile(t, dir, filepath.Join("sub", "requirements.txt"), "safe-pkg==1.0.0\n")

	detection, err := Detect(dir, DefaultLimits())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(detection.Candidates) != 0 {
		t.Fatalf("candidates = %+v, want none", detection.Candidates)
	}
}

func TestDetectRejectsSymlinkedManifest(t *testing.T) {
	outside := t.TempDir()
	target := writeTestFile(t, outside, "package.json", `{"name":"outside","dependencies":{}}`)
	dir := t.TempDir()
	if err := os.Symlink(target, filepath.Join(dir, "package.json")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	detection, err := Detect(dir, DefaultLimits())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(detection.Candidates) != 0 {
		t.Fatalf("candidates = %+v, want none", detection.Candidates)
	}
	if !hasDetectionLimitation(detection, "SYMLINKED_FILE_UNSUPPORTED") {
		t.Fatalf("missing SYMLINKED_FILE_UNSUPPORTED limitation: %+v", detection.Limitations)
	}
}

func TestDetectRejectsInvalidRoot(t *testing.T) {
	file := writeTestFile(t, t.TempDir(), "not-a-directory", "x")
	if _, err := Detect(file, DefaultLimits()); err == nil {
		t.Fatal("Detect accepted a regular file as root")
	}
	if _, err := Detect(filepath.Join(t.TempDir(), "missing"), DefaultLimits()); err == nil {
		t.Fatal("Detect accepted a missing root")
	}
}

func TestScanNPMHealthyProject(t *testing.T) {
	vulnerabilities := &testVulnerabilities{}
	result := scanFixture(t, "npm-healthy", vulnerabilities)

	if result.Status != model.StatusClean {
		t.Fatalf("status = %q, want clean; checks=%+v limitations=%+v", result.Status, result.Checks, result.Limitations)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("findings = %+v, want none", result.Findings)
	}
	if result.Dependencies.RootID != "project" {
		t.Fatalf("rootNodeId = %q, want project", result.Dependencies.RootID)
	}
	if len(result.Formats) != 1 {
		t.Fatalf("formats = %+v, want one", result.Formats)
	}
	if result.Formats[0].ResolvedPackages != 3 || result.Formats[0].UnresolvedPackages != 0 {
		t.Fatalf("format counts = %+v, want 3 resolved and 0 unresolved", result.Formats[0])
	}

	parent := nodeByID(t, result, "npm:node_modules/parent-pkg")
	if parent.Relationship != model.DependencyDirect {
		t.Fatalf("parent relationship = %q, want direct", parent.Relationship)
	}
	if parent.Version == nil || *parent.Version != "1.0.0" {
		t.Fatalf("parent version = %v, want 1.0.0", parent.Version)
	}
	if parent.Constraint != "^1.0.0" {
		t.Fatalf("parent constraint = %q, want ^1.0.0", parent.Constraint)
	}
	if !reflect.DeepEqual(parent.DependencyPath, []string{"project", "npm:node_modules/parent-pkg"}) {
		t.Fatalf("parent path = %v", parent.DependencyPath)
	}
	if !reflect.DeepEqual(parent.Dependencies, []string{"npm:node_modules/child-pkg"}) {
		t.Fatalf("parent dependencies = %v", parent.Dependencies)
	}

	child := nodeByID(t, result, "npm:node_modules/child-pkg")
	if child.Relationship != model.DependencyTransitive {
		t.Fatalf("child relationship = %q, want transitive", child.Relationship)
	}
	if child.Version == nil || *child.Version != "2.1.0" {
		t.Fatalf("child version = %v, want 2.1.0", child.Version)
	}
	wantPath := []string{"project", "npm:node_modules/parent-pkg", "npm:node_modules/child-pkg"}
	if !reflect.DeepEqual(child.DependencyPath, wantPath) {
		t.Fatalf("child path = %v, want %v", child.DependencyPath, wantPath)
	}

	for _, id := range []string{"project:detection", "npm:manifest", "npm:lockfile", "npm:dependency-resolution", "npm:vulnerabilities"} {
		if status := checkByID(t, result, id).Status; status != model.CheckSucceeded {
			t.Fatalf("check %s status = %q, want succeeded", id, status)
		}
	}
	assertEvidenceSourcesReferenced(t, result)
}

func TestScanNPMVulnerableTransitiveDependency(t *testing.T) {
	vulnerabilities := &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
		"npm:vulnerable-lib@1.2.3": vulnerabilityResult(
			model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "vulnerable-lib"},
			"1.2.3",
			testAdvisory("TEST-2026-0001", model.SeverityHigh),
		),
	}}
	result := scanFixture(t, "npm-vulnerable", vulnerabilities)

	if result.Status != model.StatusFindings {
		t.Fatalf("status = %q, want findings; checks=%+v", result.Status, result.Checks)
	}
	pkg := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "vulnerable-lib"}
	findings := findingsFor(result, pkg.String(), "1.2.3")
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one", findings)
	}
	finding := findings[0]
	if finding.Kind != model.FindingConfirmed || finding.Severity != model.SeverityHigh {
		t.Fatalf("finding = %+v, want confirmed high", finding)
	}
	if finding.Risk == nil || finding.Risk.Score != 80 {
		t.Fatalf("risk = %+v, want score 80", finding.Risk)
	}
	pathEvidence := evidenceSummary(finding, model.EvidenceDependencyPath)
	wantPath := "project -> npm:app-pkg@1.0.0 -> npm:vulnerable-lib@1.2.3"
	if !strings.Contains(pathEvidence, wantPath) {
		t.Fatalf("dependency path evidence = %q, want it to contain %q", pathEvidence, wantPath)
	}
	if summary := result.Summary; summary.ConfirmedFindings != 1 || summary.HighestSeverity == nil || *summary.HighestSeverity != model.SeverityHigh {
		t.Fatalf("summary = %+v", summary)
	}
	assertEvidenceSourcesReferenced(t, result)
}

func TestScanNPMMissingLockfileNeverUsesLatest(t *testing.T) {
	vulnerabilities := &testVulnerabilities{}
	result := scanFixture(t, "npm-missing-lock", vulnerabilities)

	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
	if status := checkByID(t, result, "npm:lockfile").Status; status != model.CheckUnavailable {
		t.Fatalf("lockfile check = %q, want unavailable", status)
	}
	if !hasLimitation(result, "LOCKFILE_MISSING", "") {
		t.Fatalf("missing LOCKFILE_MISSING limitation: %+v", result.Limitations)
	}
	if calls := vulnerabilities.calledKeys(); len(calls) != 0 {
		t.Fatalf("vulnerability provider was called with %v; unresolved packages must not be scanned", calls)
	}
	for _, id := range []string{"npm:unresolved:node_modules/parent-pkg", "npm:unresolved:node_modules/safe-pkg"} {
		node := nodeByID(t, result, id)
		if node.Version != nil {
			t.Fatalf("%s version = %v, want nil", id, node.Version)
		}
		if node.Relationship != model.DependencyDirect {
			t.Fatalf("%s relationship = %q, want direct", id, node.Relationship)
		}
	}
	if result.Formats[0].UnresolvedPackages != 2 {
		t.Fatalf("unresolved count = %d, want 2", result.Formats[0].UnresolvedPackages)
	}
}

func TestScanNPMMalformedLockfile(t *testing.T) {
	result := scanFixture(t, "npm-malformed-lock", &testVulnerabilities{})
	if result.Status != model.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if status := checkByID(t, result, "npm:lockfile").Status; status != model.CheckFailed {
		t.Fatalf("lockfile check = %q, want failed", status)
	}
	if !hasLimitation(result, "MALFORMED_LOCKFILE", "") {
		t.Fatalf("missing MALFORMED_LOCKFILE limitation: %+v", result.Limitations)
	}
}

func TestScanNPMUnsupportedLockfileVersion(t *testing.T) {
	result := scanFixture(t, "npm-unsupported-lock", &testVulnerabilities{})
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
	check := checkByID(t, result, "npm:lockfile")
	if check.Status != model.CheckUnavailable {
		t.Fatalf("lockfile check = %q, want unavailable", check.Status)
	}
	if !strings.Contains(check.Message, "lockfileVersion 1") {
		t.Fatalf("lockfile message = %q, want explicit version 1", check.Message)
	}
	if !hasLimitation(result, "UNSUPPORTED_LOCKFILE_VERSION", "") {
		t.Fatalf("missing UNSUPPORTED_LOCKFILE_VERSION limitation: %+v", result.Limitations)
	}
}

func TestScanNPMConflictingMetadata(t *testing.T) {
	result := scanFixture(t, "npm-conflict", &testVulnerabilities{})
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
	if !hasLimitation(result, "CONFLICTING_DEPENDENCY_METADATA", "parent-pkg") {
		t.Fatalf("missing conflict limitation: %+v", result.Limitations)
	}
	if status := checkByID(t, result, "npm:dependency-resolution").Status; status != model.CheckUnavailable {
		t.Fatalf("dependency-resolution = %q, want unavailable", status)
	}
	node := nodeByID(t, result, "npm:node_modules/parent-pkg")
	if node.Constraint != "^2.0.0" {
		t.Fatalf("constraint = %q, want manifest constraint ^2.0.0", node.Constraint)
	}
	if node.Version == nil || *node.Version != "1.0.0" {
		t.Fatalf("version = %v, want locked version 1.0.0", node.Version)
	}
}

func TestScanNPMNestedDependencyVersions(t *testing.T) {
	shared := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "shared-pkg"}
	vulnerabilities := &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
		"npm:shared-pkg@1.5.0": vulnerabilityResult(shared, "1.5.0", testAdvisory("TEST-2026-0002", model.SeverityHigh)),
		"npm:shared-pkg@2.0.0": vulnerabilityResult(shared, "2.0.0", testAdvisory("TEST-2026-0003", model.SeverityLow)),
	}}
	result := scanFixture(t, "npm-nested", vulnerabilities)

	nested := nodeByID(t, result, "npm:node_modules/a-pkg/node_modules/shared-pkg")
	wantNestedPath := []string{"project", "npm:node_modules/a-pkg", "npm:node_modules/a-pkg/node_modules/shared-pkg"}
	if !reflect.DeepEqual(nested.DependencyPath, wantNestedPath) {
		t.Fatalf("nested path = %v, want %v", nested.DependencyPath, wantNestedPath)
	}
	hoisted := nodeByID(t, result, "npm:node_modules/shared-pkg")
	wantHoistedPath := []string{"project", "npm:node_modules/b-pkg", "npm:node_modules/shared-pkg"}
	if !reflect.DeepEqual(hoisted.DependencyPath, wantHoistedPath) {
		t.Fatalf("hoisted path = %v, want %v", hoisted.DependencyPath, wantHoistedPath)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("findings = %+v, want two", result.Findings)
	}
	if result.Findings[0].Severity != model.SeverityHigh {
		t.Fatalf("first finding severity = %q, want high", result.Findings[0].Severity)
	}
	if result.Findings[0].ID == result.Findings[1].ID {
		t.Fatalf("finding ids collide: %q", result.Findings[0].ID)
	}
	if !strings.Contains(evidenceSummary(result.Findings[0], model.EvidenceDependencyPath), "a-pkg") {
		t.Fatalf("high finding lacks nested path evidence: %+v", result.Findings[0].Evidence)
	}
}

func TestScanNPMHandlesCycles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"name":"cycle-app","version":"1.0.0","dependencies":{"a-pkg":"^1.0.0"}}`)
	writeTestFile(t, dir, "package-lock.json", `{
	  "name": "cycle-app",
	  "version": "1.0.0",
	  "lockfileVersion": 3,
	  "packages": {
	    "": {"name": "cycle-app", "version": "1.0.0", "dependencies": {"a-pkg": "^1.0.0"}},
	    "node_modules/a-pkg": {"version": "1.0.0", "dependencies": {"b-pkg": "^1.0.0"}},
	    "node_modules/b-pkg": {"version": "1.0.0", "dependencies": {"a-pkg": "^1.0.0"}}
	  }
	}`)

	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{})
	if !hasLimitation(result, "CYCLIC_DEPENDENCY", "") {
		t.Fatalf("missing CYCLIC_DEPENDENCY limitation: %+v", result.Limitations)
	}
	a := nodeByID(t, result, "npm:node_modules/a-pkg")
	if !reflect.DeepEqual(a.Dependencies, []string{"npm:node_modules/b-pkg"}) {
		t.Fatalf("a dependencies = %v", a.Dependencies)
	}
	b := nodeByID(t, result, "npm:node_modules/b-pkg")
	if !reflect.DeepEqual(b.Dependencies, []string{"npm:node_modules/a-pkg"}) {
		t.Fatalf("b dependencies = %v", b.Dependencies)
	}
}

func TestScanNPMEnforcesNodeLimit(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"name":"limit-app","version":"1.0.0","dependencies":{"a-pkg":"^1.0.0"}}`)
	writeTestFile(t, dir, "package-lock.json", `{
	  "lockfileVersion": 3,
	  "packages": {
	    "": {"dependencies": {"a-pkg": "^1.0.0"}},
	    "node_modules/a-pkg": {"version": "1.0.0", "dependencies": {"b-pkg": "^1.0.0"}},
	    "node_modules/b-pkg": {"version": "1.0.0", "dependencies": {"c-pkg": "^1.0.0"}},
	    "node_modules/c-pkg": {"version": "1.0.0"}
	  }
	}`)
	limits := DefaultLimits()
	limits.MaxNodes = 2

	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{Limits: limits})
	if !hasLimitation(result, "NODE_LIMIT_EXCEEDED", "") {
		t.Fatalf("missing NODE_LIMIT_EXCEEDED limitation: %+v", result.Limitations)
	}
	if len(result.Dependencies.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(result.Dependencies.Nodes))
	}
	if status := checkByID(t, result, "npm:dependency-resolution").Status; status != model.CheckUnavailable {
		t.Fatalf("dependency-resolution = %q, want unavailable", status)
	}
}

func TestScanNPMEnforcesDepthLimit(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"name":"depth-app","version":"1.0.0","dependencies":{"a-pkg":"^1.0.0"}}`)
	writeTestFile(t, dir, "package-lock.json", `{
	  "lockfileVersion": 3,
	  "packages": {
	    "": {"dependencies": {"a-pkg": "^1.0.0"}},
	    "node_modules/a-pkg": {"version": "1.0.0", "dependencies": {"b-pkg": "^1.0.0"}},
	    "node_modules/b-pkg": {"version": "1.0.0", "dependencies": {"c-pkg": "^1.0.0"}},
	    "node_modules/c-pkg": {"version": "1.0.0"}
	  }
	}`)
	limits := DefaultLimits()
	limits.MaxDepth = 2

	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{Limits: limits})
	if !hasLimitation(result, "DEPENDENCY_DEPTH_EXCEEDED", "") {
		t.Fatalf("missing DEPENDENCY_DEPTH_EXCEEDED limitation: %+v", result.Limitations)
	}
	for _, node := range result.Dependencies.Nodes {
		if node.ID == "npm:node_modules/c-pkg" {
			t.Fatalf("node beyond depth limit was added: %+v", node)
		}
	}
}

func TestScanNPMEnforcesFileSizeLimit(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"name":"large-app","version":"1.0.0","description":"`+strings.Repeat("x", 1024)+`"}`)
	writeTestFile(t, dir, "package-lock.json", `{"lockfileVersion":3,"packages":{}}`)
	limits := DefaultLimits()
	limits.MaxFileSize = 128

	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{Limits: limits})
	if result.Status != model.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !hasLimitation(result, "FILE_TOO_LARGE", "package.json") {
		t.Fatalf("missing FILE_TOO_LARGE limitation: %+v", result.Limitations)
	}
	if status := checkByID(t, result, "npm:manifest").Status; status != model.CheckFailed {
		t.Fatalf("manifest check = %q, want failed", status)
	}
}

func TestScanNPMSkipsWorkspaceLinks(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"name":"workspace-app","version":"1.0.0","dependencies":{"workspace-a":"*"}}`)
	writeTestFile(t, dir, "package-lock.json", `{
	  "lockfileVersion": 3,
	  "packages": {
	    "": {"dependencies": {"workspace-a": "*"}},
	    "packages/workspace-a": {"name": "workspace-a", "version": "1.0.0"},
	    "node_modules/workspace-a": {"resolved": "packages/workspace-a", "link": true}
	  }
	}`)

	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{})
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
	if !hasLimitation(result, "LINKED_DEPENDENCY_UNSUPPORTED", "") {
		t.Fatalf("missing LINKED_DEPENDENCY_UNSUPPORTED limitation: %+v", result.Limitations)
	}
	node := nodeByID(t, result, "npm:unresolved:node_modules/workspace-a")
	if node.Version != nil {
		t.Fatalf("workspace link version = %v, want nil", node.Version)
	}
}

func TestScanHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := Scan(ctx, projectFixture("npm-healthy"), Options{
		Scanner: newTestScanner(t, &testVulnerabilities{}),
		Clock:   func() time.Time { return testNow },
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if result.Status != model.StatusCancelled {
		t.Fatalf("status = %q, want cancelled", result.Status)
	}
}

func TestScanReportsProgressStages(t *testing.T) {
	var mu sync.Mutex
	stages := make([]Stage, 0, 8)
	result := scanDirectory(t, projectFixture("npm-vulnerable"), &testVulnerabilities{}, Options{
		Progress: func(progress Progress) {
			mu.Lock()
			defer mu.Unlock()
			stages = append(stages, progress.Stage)
		},
	})
	if result.Status != model.StatusClean && result.Status != model.StatusFindings {
		t.Fatalf("unexpected status %q", result.Status)
	}
	wanted := []Stage{StageDetect, StageParse, StageResolve, StageScan, StageAssemble}
	for _, stage := range wanted {
		found := false
		for _, observed := range stages {
			if observed == stage {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("stage %q missing from %v", stage, stages)
		}
	}
}

func TestScanIsDeterministic(t *testing.T) {
	first := scanFixture(t, "npm-nested", &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
		"npm:shared-pkg@1.5.0": vulnerabilityResult(
			model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "shared-pkg"}, "1.5.0",
			testAdvisory("TEST-2026-0004", model.SeverityMedium)),
	}})
	second := scanFixture(t, "npm-nested", &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
		"npm:shared-pkg@1.5.0": vulnerabilityResult(
			model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "shared-pkg"}, "1.5.0",
			testAdvisory("TEST-2026-0004", model.SeverityMedium)),
	}})
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("project scans are not deterministic:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestScanWithoutManifestIsIncomplete(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "README.md", "nothing\n")
	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{})
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
	if !hasLimitation(result, "NO_SUPPORTED_MANIFEST", "") {
		t.Fatalf("missing NO_SUPPORTED_MANIFEST limitation: %+v", result.Limitations)
	}
	if status := checkByID(t, result, "project:detection").Status; status != model.CheckUnavailable {
		t.Fatalf("detection check = %q, want unavailable", status)
	}
}

func TestScanRequiresScanner(t *testing.T) {
	if _, err := Scan(context.Background(), projectFixture("npm-healthy"), Options{}); err == nil {
		t.Fatal("Scan accepted a nil engine scanner")
	}
}

func TestScanMultipleEcosystems(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"package.json", "package-lock.json"} {
		content, err := os.ReadFile(filepath.Join(projectFixture("npm-healthy"), name))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		writeTestFile(t, dir, name, string(content))
	}
	requirements, err := os.ReadFile(filepath.Join(projectFixture("python-pinned"), "requirements.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	writeTestFile(t, dir, "requirements.txt", string(requirements))

	pypiParent := model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "parent-pkg"}
	vulnerabilities := &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
		"pypi:parent-pkg@2.0.0": vulnerabilityResult(pypiParent, "2.0.0", testAdvisory("TEST-2026-0005", model.SeverityMedium)),
	}}
	result := scanDirectory(t, dir, vulnerabilities, Options{})
	if len(result.Formats) != 2 {
		t.Fatalf("formats = %+v, want two", result.Formats)
	}
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete because Python transitive coverage is unavailable", result.Status)
	}
	if len(findingsFor(result, "pypi:parent-pkg", "2.0.0")) != 1 {
		t.Fatalf("missing PyPI finding: %+v", result.Findings)
	}
	nodeByID(t, result, "pypi:parent-pkg")
	nodeByID(t, result, "npm:node_modules/safe-pkg")
	assertEvidenceSourcesReferenced(t, result)
}

func TestScanNPMMissingLockfileEnforcesNodeLimit(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"name":"limit-app","version":"1.0.0","dependencies":{"a-pkg":"^1.0.0","b-pkg":"^1.0.0","c-pkg":"^1.0.0"}}`)
	limits := DefaultLimits()
	limits.MaxNodes = 1
	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{Limits: limits})
	if !hasLimitation(result, "NODE_LIMIT_EXCEEDED", "") {
		t.Fatalf("missing NODE_LIMIT_EXCEEDED limitation: %+v", result.Limitations)
	}
	if len(result.Dependencies.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(result.Dependencies.Nodes))
	}
	if !hasLimitation(result, "LOCKFILE_MISSING", "") {
		t.Fatalf("missing LOCKFILE_MISSING limitation: %+v", result.Limitations)
	}
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
}
