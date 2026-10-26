package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"depscope/internal/engine"
	"depscope/internal/model"
)

func TestScanPythonPinnedRequirements(t *testing.T) {
	pypiParent := model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "parent-pkg"}
	vulnerabilities := &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
		"pypi:parent-pkg@2.0.0": vulnerabilityResult(pypiParent, "2.0.0", testAdvisory("TEST-2026-0100", model.SeverityHigh)),
	}}
	result := scanFixture(t, "python-pinned", vulnerabilities)

	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
	if status := checkByID(t, result, "python:manifest").Status; status != model.CheckSucceeded {
		t.Fatalf("manifest check = %q, want succeeded", status)
	}
	if status := checkByID(t, result, "python:dependency-resolution").Status; status != model.CheckUnavailable {
		t.Fatalf("dependency-resolution = %q, want unavailable", status)
	}
	if !hasLimitation(result, "PYTHON_TRANSITIVE_UNAVAILABLE", "") {
		t.Fatalf("missing PYTHON_TRANSITIVE_UNAVAILABLE limitation: %+v", result.Limitations)
	}

	safe := nodeByID(t, result, "pypi:safe-pkg")
	if safe.Version == nil || *safe.Version != "1.0.0" {
		t.Fatalf("safe-pkg version = %v, want 1.0.0", safe.Version)
	}
	parent := nodeByID(t, result, "pypi:parent-pkg")
	if parent.Version == nil || *parent.Version != "2.0.0" {
		t.Fatalf("parent-pkg version = %v, want 2.0.0", parent.Version)
	}
	if parent.Constraint != "==2.0.0" {
		t.Fatalf("parent-pkg constraint = %q", parent.Constraint)
	}
	if !strings.Contains(evidenceSummary(findingsFor(result, "pypi:parent-pkg", "2.0.0")[0], model.EvidenceDependencyPath), "project -> pypi:parent-pkg@2.0.0") {
		t.Fatalf("missing direct dependency path evidence: %+v", result.Findings)
	}
	if calls := vulnerabilities.calledKeys(); len(calls) != 2 {
		t.Fatalf("provider calls = %v, want two exact pins", calls)
	}
	assertEvidenceSourcesReferenced(t, result)
}

func TestScanPythonUnpinnedRequirementsStayUnresolved(t *testing.T) {
	vulnerabilities := &testVulnerabilities{}
	result := scanFixture(t, "python-unpinned", vulnerabilities)

	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
	if calls := vulnerabilities.calledKeys(); len(calls) != 0 {
		t.Fatalf("unresolved requirements were scanned: %v", calls)
	}
	for _, name := range []string{"parent-pkg", "safe-pkg", "unpinned-lib"} {
		node := nodeByID(t, result, "pypi:"+name)
		if node.Version != nil {
			t.Fatalf("%s version = %v, want nil", name, node.Version)
		}
	}
	if status := checkByID(t, result, "python:manifest").Status; status != model.CheckUnavailable {
		t.Fatalf("manifest check = %q, want unavailable", status)
	}
	if !hasLimitation(result, "UNRESOLVED_VERSION", "parent-pkg") {
		t.Fatalf("missing UNRESOLVED_VERSION limitation: %+v", result.Limitations)
	}
	if result.Formats[0].UnresolvedPackages != 3 {
		t.Fatalf("unresolved count = %d, want 3", result.Formats[0].UnresolvedPackages)
	}
}

func TestScanPythonUnsupportedSyntax(t *testing.T) {
	vulnerabilities := &testVulnerabilities{}
	result := scanFixture(t, "python-unsupported", vulnerabilities)

	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
	for _, code := range []string{
		"EDITABLE_UNSUPPORTED",
		"UNSUPPORTED_REQUIREMENT_SOURCE",
		"ENVIRONMENT_MARKER_UNSUPPORTED",
		"UNSUPPORTED_REQUIREMENTS_OPTION",
	} {
		if !hasLimitation(result, code, "") {
			t.Fatalf("missing %s limitation: %+v", code, result.Limitations)
		}
	}
	nested := nodeByID(t, result, "pypi:nested-pkg")
	if nested.Version == nil || *nested.Version != "3.0.0" {
		t.Fatalf("nested include version = %v, want 3.0.0", nested.Version)
	}
	marked := nodeByID(t, result, "pypi:marked-pkg")
	if marked.Version != nil {
		t.Fatalf("environment-marked requirement version = %v, want nil", marked.Version)
	}
	if calls := vulnerabilities.calledKeys(); len(calls) != 1 || calls[0] != "pypi:nested-pkg@3.0.0" {
		t.Fatalf("provider calls = %v, want only the supported nested pin", calls)
	}
	if result.Status == model.StatusClean {
		t.Fatal("unsupported syntax produced a clean result")
	}
}

func TestScanPythonConflictingRequirements(t *testing.T) {
	result := scanFixture(t, "python-conflict", &testVulnerabilities{})
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
	for _, name := range []string{"conflict-pkg", "range-pkg"} {
		node := nodeByID(t, result, "pypi:"+name)
		if node.Version != nil {
			t.Fatalf("%s version = %v, want nil because requirements conflict", name, node.Version)
		}
	}
	if !hasLimitation(result, "CONFLICTING_REQUIREMENTS", "conflict-pkg") {
		t.Fatalf("missing conflict limitation for conflict-pkg: %+v", result.Limitations)
	}
	if !hasLimitation(result, "CONFLICTING_REQUIREMENTS", "range-pkg") {
		t.Fatalf("missing conflict limitation for range-pkg: %+v", result.Limitations)
	}
}

func TestScanPythonIncludeOutsideRootIsRejected(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeTestFile(t, parent, "outside.txt", "outside-pkg==9.9.9\n")
	writeTestFile(t, root, "requirements.txt", "-r ../outside.txt\n")

	result := scanDirectory(t, root, &testVulnerabilities{}, Options{})
	if !hasLimitation(result, "INCLUDE_OUTSIDE_ROOT", "") {
		t.Fatalf("missing INCLUDE_OUTSIDE_ROOT limitation: %+v", result.Limitations)
	}
	for _, node := range result.Dependencies.Nodes {
		if node.Package.Name == "outside-pkg" {
			t.Fatalf("outside include was parsed: %+v", node)
		}
	}
}

func TestScanPythonSymlinkedIncludeIsRejected(t *testing.T) {
	outside := t.TempDir()
	target := writeTestFile(t, outside, "linked.txt", "linked-pkg==9.9.9\n")
	dir := t.TempDir()
	writeTestFile(t, dir, "requirements.txt", "-r linked.txt\n")
	if err := os.Symlink(target, filepath.Join(dir, "linked.txt")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{})
	if !hasLimitation(result, "SYMLINKED_FILE_UNSUPPORTED", "linked.txt") {
		t.Fatalf("missing symlink limitation: %+v", result.Limitations)
	}
	for _, node := range result.Dependencies.Nodes {
		if node.Package.Name == "linked-pkg" {
			t.Fatalf("symlinked include was parsed: %+v", node)
		}
	}
}

func TestScanPythonIncludeLimit(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "requirements.txt", "-r first.txt\n")
	writeTestFile(t, dir, "first.txt", "-r second.txt\n")
	writeTestFile(t, dir, "second.txt", "deep-pkg==1.0.0\n")
	limits := DefaultLimits()
	limits.MaxIncludes = 1

	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{Limits: limits})
	if !hasLimitation(result, "INCLUDE_LIMIT_EXCEEDED", "") {
		t.Fatalf("missing INCLUDE_LIMIT_EXCEEDED limitation: %+v", result.Limitations)
	}
	for _, node := range result.Dependencies.Nodes {
		if node.Package.Name == "deep-pkg" {
			t.Fatalf("include beyond limit was parsed: %+v", node)
		}
	}
}

func TestScanPythonOversizedRequirements(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "requirements.txt", "safe-pkg==1.0.0\n#"+strings.Repeat("x", 1024)+"\n")
	limits := DefaultLimits()
	limits.MaxFileSize = 128

	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{Limits: limits})
	if result.Status != model.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !hasLimitation(result, "FILE_TOO_LARGE", "requirements.txt") {
		t.Fatalf("missing FILE_TOO_LARGE limitation: %+v", result.Limitations)
	}
}

func TestScanPythonEmptyRequirementsIsIncomplete(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "requirements.txt", "# no requirements\n")
	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{})
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
	if !hasLimitation(result, "PYTHON_TRANSITIVE_UNAVAILABLE", "") {
		t.Fatalf("missing PYTHON_TRANSITIVE_UNAVAILABLE limitation: %+v", result.Limitations)
	}
}

func TestScanPythonInvalidRequirementLine(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "requirements.txt", "this is not a requirement !!!\n")
	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{})
	if !hasLimitation(result, "INVALID_REQUIREMENT", "") {
		t.Fatalf("missing INVALID_REQUIREMENT limitation: %+v", result.Limitations)
	}
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
}

func TestScanPythonMissingIncludeExplainsFailure(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "requirements.txt", "safe-pkg==1.0.0\n-r missing-requirements.txt\n")
	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{})
	if !hasLimitation(result, "INCLUDE_UNREADABLE", "missing-requirements.txt") {
		t.Fatalf("missing INCLUDE_UNREADABLE limitation: %+v", result.Limitations)
	}
	if status := checkByID(t, result, "python:manifest").Status; status != model.CheckUnavailable {
		t.Fatalf("manifest check = %q, want unavailable", status)
	}
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
}

func TestScanPythonEnforcesNodeLimit(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "requirements.txt", "a-pkg==1.0.0\nb-pkg==1.0.0\nc-pkg==1.0.0\n")
	limits := DefaultLimits()
	limits.MaxNodes = 1
	result := scanDirectory(t, dir, &testVulnerabilities{}, Options{Limits: limits})
	if !hasLimitation(result, "NODE_LIMIT_EXCEEDED", "") {
		t.Fatalf("missing NODE_LIMIT_EXCEEDED limitation: %+v", result.Limitations)
	}
	if len(result.Dependencies.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(result.Dependencies.Nodes))
	}
	if status := checkByID(t, result, "python:manifest").Status; status != model.CheckUnavailable {
		t.Fatalf("manifest check = %q, want unavailable", status)
	}
	if result.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", result.Status)
	}
}
