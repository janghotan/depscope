package project

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/model"
)

func scopeStrings(scopes []DependencyScope) []string {
	values := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		values = append(values, string(scope))
	}
	return values
}

func evidenceStrings(items []DependencyEvidence) []string {
	labels := make([]string, 0, len(items))
	for _, item := range items {
		label := item.Path
		if item.Field != "" {
			label += "#" + item.Field
		}
		labels = append(labels, label)
	}
	return labels
}

func contextForFinding(t *testing.T, result Result, pkg model.PackageIdentity, version string) (model.Finding, DependencyContext) {
	t.Helper()
	findings := findingsFor(result, pkg.String(), version)
	if len(findings) != 1 {
		t.Fatalf("findings for %s@%s = %+v, want one", pkg, version, findings)
	}
	context, exists := result.FindingContexts[findings[0].ID]
	if !exists {
		t.Fatalf("finding %q has no dependency context; contexts=%+v", findings[0].ID, result.FindingContexts)
	}
	return findings[0], context
}

func formatFor(t *testing.T, result Result, ecosystem model.Ecosystem) FormatInfo {
	t.Helper()
	for _, format := range result.Formats {
		if format.Ecosystem == ecosystem {
			return format
		}
	}
	t.Fatalf("no format for ecosystem %q in %+v", ecosystem, result.Formats)
	return FormatInfo{}
}

func dependencyPathSummaries(finding model.Finding) []string {
	summaries := []string{}
	for _, evidence := range finding.Evidence {
		if evidence.Kind != model.EvidenceDependencyPath {
			continue
		}
		if !strings.HasPrefix(evidence.Summary, "Dependency path: ") {
			continue
		}
		summaries = append(summaries, strings.TrimPrefix(evidence.Summary, "Dependency path: "))
	}
	return summaries
}

func assertPathSet(t *testing.T, finding model.Finding, want []string) {
	t.Helper()
	got := dependencyPathSummaries(finding)
	sort.Strings(got)
	expected := append([]string(nil), want...)
	sort.Strings(expected)
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("dependency paths for %q = %v, want %v", finding.ID, got, expected)
	}
}

// TestNPMDependencyContextClassifiesSupportedMetadata 覆盖直接/传递、生产/开发/可选
// 分类、多父节点路径与同一包的直接加传递路径。
func TestNPMDependencyContextClassifiesSupportedMetadata(t *testing.T) {
	vulnerabilities := &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
		"npm:dev-child@1.2.0": vulnerabilityResult(
			model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "dev-child"}, "1.2.0",
			testAdvisory("TEST-2026-0201", model.SeverityHigh)),
		"npm:optional-addon@3.0.0": vulnerabilityResult(
			model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "optional-addon"}, "3.0.0",
			testAdvisory("TEST-2026-0202", model.SeverityMedium)),
		"npm:multi-parent-target@4.0.0": vulnerabilityResult(
			model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "multi-parent-target"}, "4.0.0",
			testAdvisory("TEST-2026-0203", model.SeverityCritical)),
		"npm:shared-lib@1.0.0": vulnerabilityResult(
			model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "shared-lib"}, "1.0.0",
			testAdvisory("TEST-2026-0204", model.SeverityLow)),
		"npm:shared-dev@1.0.0": vulnerabilityResult(
			model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "shared-dev"}, "1.0.0",
			testAdvisory("TEST-2026-0205", model.SeverityHigh)),
		"npm:optional-child@1.3.0": vulnerabilityResult(
			model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "optional-child"}, "1.3.0",
			testAdvisory("TEST-2026-0206", model.SeverityLow)),
	}}
	result := scanFixture(t, "npm-context", vulnerabilities)

	if format := formatFor(t, result, model.EcosystemNPM); format.Resolution != DependencyResolutionComplete {
		t.Fatalf("format resolution = %q, want complete", format.Resolution)
	}

	nodeScopes := []struct {
		id   string
		want []string
	}{
		{"npm:node_modules/prod-parent", []string{"production"}},
		{"npm:node_modules/dev-tool", []string{"development"}},
		{"npm:node_modules/optional-addon", []string{"optional"}},
		{"npm:node_modules/dev-child", []string{"development"}},
		{"npm:node_modules/optional-child", []string{"optional"}},
		{"npm:node_modules/multi-parent-target", []string{"production"}},
		// 同一包同时是清单开发依赖和锁文件中的生产依赖时，两种分类都要保留。
		{"npm:node_modules/shared-dev", []string{"production", "development"}},
		{"npm:node_modules/shared-lib", []string{"production"}},
	}
	for _, tc := range nodeScopes {
		node := nodeByID(t, result, tc.id)
		if got := scopeStrings(node.Scopes); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s scopes = %v, want %v", tc.id, got, tc.want)
		}
	}

	cases := []struct {
		pkg          string
		version      string
		advisory     string
		severity     model.Severity
		findingID    string
		relationship model.DependencyRelationship
		scopes       []string
		evidence     []string
	}{
		{
			pkg: "dev-child", version: "1.2.0", advisory: "TEST-2026-0201", severity: model.SeverityHigh,
			findingID:    "VULN-TEST-2026-0201@npm:dev-child@1.2.0",
			relationship: model.DependencyTransitive,
			scopes:       []string{"development"},
			evidence:     []string{"package-lock.json#dev"},
		},
		{
			pkg: "optional-addon", version: "3.0.0", advisory: "TEST-2026-0202", severity: model.SeverityMedium,
			findingID:    "VULN-TEST-2026-0202@npm:optional-addon@3.0.0",
			relationship: model.DependencyDirect,
			scopes:       []string{"optional"},
			evidence:     []string{"package-lock.json#optional", "package.json#optionalDependencies"},
		},
		{
			pkg: "multi-parent-target", version: "4.0.0", advisory: "TEST-2026-0203", severity: model.SeverityCritical,
			findingID:    "VULN-TEST-2026-0203@npm:multi-parent-target@4.0.0",
			relationship: model.DependencyTransitive,
			scopes:       []string{"production"},
			evidence:     []string{"package-lock.json"},
		},
		{
			pkg: "shared-lib", version: "1.0.0", advisory: "TEST-2026-0204", severity: model.SeverityLow,
			findingID:    "VULN-TEST-2026-0204@npm:shared-lib@1.0.0",
			relationship: model.DependencyDirect,
			scopes:       []string{"production"},
			evidence:     []string{"package-lock.json", "package.json#dependencies"},
		},
		{
			pkg: "shared-dev", version: "1.0.0", advisory: "TEST-2026-0205", severity: model.SeverityHigh,
			findingID:    "VULN-TEST-2026-0205@npm:shared-dev@1.0.0",
			relationship: model.DependencyDirect,
			scopes:       []string{"production", "development"},
			evidence:     []string{"package-lock.json", "package.json#devDependencies"},
		},
		{
			pkg: "optional-child", version: "1.3.0", advisory: "TEST-2026-0206", severity: model.SeverityLow,
			findingID:    "VULN-TEST-2026-0206@npm:optional-child@1.3.0",
			relationship: model.DependencyTransitive,
			scopes:       []string{"optional"},
			evidence:     []string{"package-lock.json#optional"},
		},
	}
	for _, tc := range cases {
		pkg := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: tc.pkg}
		finding, context := contextForFinding(t, result, pkg, tc.version)
		if finding.ID != tc.findingID {
			t.Fatalf("%s finding ID = %q, want %q", tc.pkg, finding.ID, tc.findingID)
		}
		if finding.Severity != tc.severity {
			t.Fatalf("%s severity = %q, want %q", tc.pkg, finding.Severity, tc.severity)
		}
		if finding.Title != "Synthetic advisory "+tc.advisory {
			t.Fatalf("%s title = %q, want the raw advisory title", tc.pkg, finding.Title)
		}
		if context.Relationship != tc.relationship {
			t.Fatalf("%s relationship = %q, want %q", tc.pkg, context.Relationship, tc.relationship)
		}
		if got := scopeStrings(context.Scopes); !reflect.DeepEqual(got, tc.scopes) {
			t.Fatalf("%s context scopes = %v, want %v", tc.pkg, got, tc.scopes)
		}
		if context.Resolution != DependencyResolutionComplete {
			t.Fatalf("%s context resolution = %q, want complete", tc.pkg, context.Resolution)
		}
		if got := evidenceStrings(context.Evidence); !reflect.DeepEqual(got, tc.evidence) {
			t.Fatalf("%s context evidence = %v, want %v", tc.pkg, got, tc.evidence)
		}
	}
	if len(result.FindingContexts) != len(result.Findings) {
		t.Fatalf("contexts = %d, findings = %d; every resolved finding must have a context", len(result.FindingContexts), len(result.Findings))
	}

	multiFinding, _ := contextForFinding(t, result, model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "multi-parent-target"}, "4.0.0")
	assertPathSet(t, multiFinding, []string{
		"project -> npm:dev-tool@2.0.0 -> npm:multi-parent-target@4.0.0",
		"project -> npm:prod-parent@1.0.0 -> npm:multi-parent-target@4.0.0",
	})
	sharedFinding, _ := contextForFinding(t, result, model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "shared-lib"}, "1.0.0")
	assertPathSet(t, sharedFinding, []string{
		"project -> npm:shared-lib@1.0.0",
		"project -> npm:prod-parent@1.0.0 -> npm:shared-lib@1.0.0",
	})
	devChildFinding, _ := contextForFinding(t, result, model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "dev-child"}, "1.2.0")
	assertPathSet(t, devChildFinding, []string{
		"project -> npm:dev-tool@2.0.0 -> npm:dev-child@1.2.0",
	})
	optionalChildFinding, _ := contextForFinding(t, result, model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "optional-child"}, "1.3.0")
	assertPathSet(t, optionalChildFinding, []string{
		"project -> npm:optional-addon@3.0.0 -> npm:optional-child@1.3.0",
	})
}

// TestNPMDependencyContextMissingMetadataIsExplicit 覆盖缺失锁文件、缺失传递条目与
// 损坏锁文件：上下文必须明确为 partial/unknown，不能猜测。
func TestNPMDependencyContextMissingMetadataIsExplicit(t *testing.T) {
	t.Run("missing lockfile", func(t *testing.T) {
		result := scanFixture(t, "npm-missing-lock", &testVulnerabilities{})
		if format := formatFor(t, result, model.EcosystemNPM); format.Resolution != DependencyResolutionPartial {
			t.Fatalf("format resolution = %q, want partial", format.Resolution)
		}
		for _, id := range []string{"npm:unresolved:node_modules/parent-pkg", "npm:unresolved:node_modules/safe-pkg"} {
			node := nodeByID(t, result, id)
			if got := scopeStrings(node.Scopes); !reflect.DeepEqual(got, []string{"production"}) {
				t.Fatalf("%s scopes = %v, want manifest-declared production", id, got)
			}
			if got := evidenceStrings(node.ScopeEvidence); !reflect.DeepEqual(got, []string{"package.json#dependencies"}) {
				t.Fatalf("%s evidence = %v, want package.json dependencies", id, got)
			}
		}
		if len(result.FindingContexts) != 0 {
			t.Fatalf("contexts = %+v, want none without resolved findings", result.FindingContexts)
		}
	})

	t.Run("missing transitive entry", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "package.json", `{"name":"partial-app","version":"1.0.0","dependencies":{"a-pkg":"^1.0.0"}}`)
		writeTestFile(t, dir, "package-lock.json", `{
		  "name": "partial-app",
		  "version": "1.0.0",
		  "lockfileVersion": 3,
		  "packages": {
		    "": {"name": "partial-app", "version": "1.0.0", "dependencies": {"a-pkg": "^1.0.0"}},
		    "node_modules/a-pkg": {"version": "1.0.0", "dependencies": {"missing-pkg": "^2.0.0"}}
		  }
		}`)
		aPkg := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "a-pkg"}
		vulnerabilities := &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
			"npm:a-pkg@1.0.0": vulnerabilityResult(aPkg, "1.0.0", testAdvisory("TEST-2026-0210", model.SeverityHigh)),
		}}
		result := scanDirectory(t, dir, vulnerabilities, Options{})
		if format := formatFor(t, result, model.EcosystemNPM); format.Resolution != DependencyResolutionPartial {
			t.Fatalf("format resolution = %q, want partial", format.Resolution)
		}
		if !hasLimitation(result, "TRANSITIVE_DEPENDENCY_UNRESOLVED", "missing-pkg") {
			t.Fatalf("missing TRANSITIVE_DEPENDENCY_UNRESOLVED limitation: %+v", result.Limitations)
		}
		_, context := contextForFinding(t, result, aPkg, "1.0.0")
		if context.Resolution != DependencyResolutionPartial {
			t.Fatalf("context resolution = %q, want partial", context.Resolution)
		}

		missing := nodeByID(t, result, "npm:unresolved:node_modules/a-pkg/missing-pkg")
		if got := scopeStrings(missing.Scopes); !reflect.DeepEqual(got, []string{"unknown"}) {
			t.Fatalf("unresolved transitive scopes = %v, want unknown", got)
		}
		if got := evidenceStrings(missing.ScopeEvidence); !reflect.DeepEqual(got, []string{"package-lock.json"}) {
			t.Fatalf("unresolved transitive evidence = %v, want the lockfile only", got)
		}
		if _, exists := result.FindingContexts[missing.ID]; exists {
			t.Fatalf("unresolved node %q must not receive a finding context", missing.ID)
		}
	})

	t.Run("malformed lockfile", func(t *testing.T) {
		result := scanFixture(t, "npm-malformed-lock", &testVulnerabilities{})
		if result.Status != model.StatusFailed {
			t.Fatalf("status = %q, want failed", result.Status)
		}
		if format := formatFor(t, result, model.EcosystemNPM); format.Resolution != DependencyResolutionPartial {
			t.Fatalf("format resolution = %q, want partial", format.Resolution)
		}
		if len(result.FindingContexts) != 0 || len(result.Findings) != 0 {
			t.Fatalf("findings = %+v, contexts = %+v; malformed lockfiles must not fabricate context", result.Findings, result.FindingContexts)
		}
	})
}

// TestPythonDependencyContextIsExplicitlyUnknown 确认 requirements.txt 只提供直接
// 依赖且没有分类依据，因此必须报告 unknown 与 partial。
func TestPythonDependencyContextIsExplicitlyUnknown(t *testing.T) {
	parent := model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "parent-pkg"}
	vulnerabilities := &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
		"pypi:parent-pkg@2.0.0": vulnerabilityResult(parent, "2.0.0", testAdvisory("TEST-2026-0220", model.SeverityMedium)),
	}}
	result := scanFixture(t, "python-pinned", vulnerabilities)

	if format := formatFor(t, result, model.EcosystemPyPI); format.Resolution != DependencyResolutionPartial {
		t.Fatalf("format resolution = %q, want partial", format.Resolution)
	}
	finding, context := contextForFinding(t, result, parent, "2.0.0")
	if finding.Severity != model.SeverityMedium {
		t.Fatalf("severity = %q, want medium", finding.Severity)
	}
	if finding.ID != "VULN-TEST-2026-0220@pypi:parent-pkg@2.0.0" {
		t.Fatalf("finding ID = %q, want unchanged advisory identity", finding.ID)
	}
	if context.Relationship != model.DependencyDirect {
		t.Fatalf("relationship = %q, want direct", context.Relationship)
	}
	if got := scopeStrings(context.Scopes); !reflect.DeepEqual(got, []string{"unknown"}) {
		t.Fatalf("scopes = %v, want unknown", got)
	}
	if context.Resolution != DependencyResolutionPartial {
		t.Fatalf("resolution = %q, want partial", context.Resolution)
	}
	if got := evidenceStrings(context.Evidence); !reflect.DeepEqual(got, []string{"requirements.txt"}) {
		t.Fatalf("evidence = %v, want requirements.txt", got)
	}
	node := nodeByID(t, result, "pypi:parent-pkg")
	if got := scopeStrings(node.Scopes); !reflect.DeepEqual(got, []string{"unknown"}) {
		t.Fatalf("node scopes = %v, want unknown", got)
	}
	if got := evidenceStrings(node.ScopeEvidence); !reflect.DeepEqual(got, []string{"requirements.txt"}) {
		t.Fatalf("node evidence = %v, want requirements.txt", got)
	}

	unpinned := scanFixture(t, "python-unpinned", &testVulnerabilities{})
	if format := formatFor(t, unpinned, model.EcosystemPyPI); format.Resolution != DependencyResolutionPartial {
		t.Fatalf("unpinned format resolution = %q, want partial", format.Resolution)
	}
	for _, node := range unpinned.Dependencies.Nodes {
		if got := scopeStrings(node.Scopes); !reflect.DeepEqual(got, []string{"unknown"}) {
			t.Fatalf("%s scopes = %v, want unknown", node.ID, got)
		}
	}
}

// TestDependencyContextPathsHandleCyclesAndNestedVersions 覆盖循环图、去重路径与
// 同一包不同版本各自独立的上下文。
func TestDependencyContextPathsHandleCyclesAndNestedVersions(t *testing.T) {
	t.Run("cycle", func(t *testing.T) {
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
		bPkg := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "b-pkg"}
		vulnerabilities := &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
			"npm:b-pkg@1.0.0": vulnerabilityResult(bPkg, "1.0.0", testAdvisory("TEST-2026-0230", model.SeverityHigh)),
		}}
		result := scanDirectory(t, dir, vulnerabilities, Options{})
		finding, context := contextForFinding(t, result, bPkg, "1.0.0")
		if context.Relationship != model.DependencyTransitive {
			t.Fatalf("relationship = %q, want transitive", context.Relationship)
		}
		assertPathSet(t, finding, []string{
			"project -> npm:a-pkg@1.0.0 -> npm:b-pkg@1.0.0",
		})
		if !hasLimitation(result, "CYCLIC_DEPENDENCY", "") {
			t.Fatalf("missing CYCLIC_DEPENDENCY limitation: %+v", result.Limitations)
		}
	})

	t.Run("nested versions", func(t *testing.T) {
		shared := model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "shared-pkg"}
		vulnerabilities := &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
			"npm:shared-pkg@1.5.0": vulnerabilityResult(shared, "1.5.0", testAdvisory("TEST-2026-0231", model.SeverityHigh)),
			"npm:shared-pkg@2.0.0": vulnerabilityResult(shared, "2.0.0", testAdvisory("TEST-2026-0232", model.SeverityLow)),
		}}
		result := scanFixture(t, "npm-nested", vulnerabilities)

		nested, nestedContext := contextForFinding(t, result, shared, "1.5.0")
		hoisted, hoistedContext := contextForFinding(t, result, shared, "2.0.0")
		if nested.ID == hoisted.ID {
			t.Fatalf("nested and hoisted finding IDs collide: %q", nested.ID)
		}
		for version, context := range map[string]DependencyContext{"1.5.0": nestedContext, "2.0.0": hoistedContext} {
			if context.Relationship != model.DependencyTransitive {
				t.Fatalf("%s relationship = %q, want transitive", version, context.Relationship)
			}
			if got := scopeStrings(context.Scopes); !reflect.DeepEqual(got, []string{"production"}) {
				t.Fatalf("%s scopes = %v, want production", version, got)
			}
			if context.Resolution != DependencyResolutionComplete {
				t.Fatalf("%s resolution = %q, want complete", version, context.Resolution)
			}
		}
		assertPathSet(t, nested, []string{
			"project -> npm:a-pkg@1.0.0 -> npm:shared-pkg@1.5.0",
		})
		assertPathSet(t, hoisted, []string{
			"project -> npm:b-pkg@1.0.0 -> npm:shared-pkg@2.0.0",
		})
	})
}

// TestDependencyContextIsDeterministic 确认上下文与路径在重复扫描中保持稳定。
func TestDependencyContextIsDeterministic(t *testing.T) {
	build := func() Result {
		return scanFixture(t, "npm-context", &testVulnerabilities{results: map[string]engine.VulnerabilityResult{
			"npm:multi-parent-target@4.0.0": vulnerabilityResult(
				model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "multi-parent-target"}, "4.0.0",
				testAdvisory("TEST-2026-0240", model.SeverityCritical)),
			"npm:shared-lib@1.0.0": vulnerabilityResult(
				model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "shared-lib"}, "1.0.0",
				testAdvisory("TEST-2026-0241", model.SeverityLow)),
		}})
	}
	first := build()
	second := build()
	if !reflect.DeepEqual(first.FindingContexts, second.FindingContexts) {
		t.Fatalf("dependency contexts are not deterministic:\nfirst=%+v\nsecond=%+v", first.FindingContexts, second.FindingContexts)
	}
	if !reflect.DeepEqual(first.Findings, second.Findings) {
		t.Fatalf("findings are not deterministic:\nfirst=%+v\nsecond=%+v", first.Findings, second.Findings)
	}
}
