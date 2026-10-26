package model_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/model"
)

var fixtureTime = time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)

func ptr[T any](value T) *T {
	return &value
}

func demoTarget() model.PackageIdentity {
	return model.PackageIdentity{
		Ecosystem: model.EcosystemNPM,
		Name:      "@depscope-demo/clean-package",
	}
}

func demoSources() []model.SourceReference {
	return []model.SourceReference{
		{
			ID:          "src-demo-registry",
			Provider:    model.SourceProviderDemo,
			Identifier:  "DEMO-REGISTRY-SNAPSHOT-1",
			RetrievedAt: fixtureTime,
		},
		{
			ID:          "src-demo-osv",
			Provider:    model.SourceProviderDemo,
			Identifier:  "DEMO-OSV-SNAPSHOT-1",
			RetrievedAt: fixtureTime,
		},
	}
}

func demoChecks() []model.CheckResult {
	return []model.CheckResult{
		{
			ID:        "dependency-resolution",
			Name:      "Dependency resolution",
			Status:    model.CheckSucceeded,
			Message:   "Resolved 1 dependency node.",
			SourceIDs: []string{"src-demo-registry"},
		},
		{
			ID:        "registry-metadata",
			Name:      "Registry metadata",
			Status:    model.CheckSucceeded,
			Message:   "Registry metadata was available.",
			SourceIDs: []string{"src-demo-registry"},
		},
		{
			ID:        "vulnerability-lookup",
			Name:      "Vulnerability lookup",
			Status:    model.CheckSucceeded,
			Message:   "Vulnerability lookup completed.",
			SourceIDs: []string{"src-demo-osv"},
		},
		{
			ID:        "risk-evaluation",
			Name:      "Risk evaluation",
			Status:    model.CheckSucceeded,
			Message:   "Risk evaluation completed.",
			SourceIDs: []string{},
		},
	}
}

func validReport() model.ScanReport {
	target := demoTarget()
	version := "1.0.0"
	notice := "DEMO DATA - synthetic fixture; not a real security result."

	return model.ScanReport{
		SchemaVersion: model.SchemaVersion,
		DataMode:      model.DataModeDemo,
		DemoNotice:    &notice,
		ScannedAt:     fixtureTime,
		Status:        model.StatusClean,
		Target: model.ScanTarget{
			Package:         target,
			ResolvedVersion: &version,
		},
		Summary: model.Summary{
			ConfirmedFindings: 0,
			HeuristicWarnings: 0,
			HighestSeverity:   nil,
		},
		Findings: []model.Finding{},
		Checks:   demoChecks(),
		Dependencies: model.DependencyGraph{
			RootNodeID: "root",
			Nodes: []model.DependencyNode{
				{
					ID:             "root",
					Package:        target,
					Version:        &version,
					Relationship:   model.DependencyRoot,
					DependencyPath: []string{"root"},
					Dependencies:   []string{},
				},
			},
		},
		Sources: demoSources(),
		Limitations: []model.Limitation{
			{
				Code:    "DEMO_DATA",
				Message: "All data in this report is synthetic and must not be used as a real security assessment.",
			},
		},
	}
}

func confirmedFinding() model.Finding {
	version := "1.0.0"
	return model.Finding{
		ID:          "finding-confirmed-1",
		Kind:        model.FindingConfirmed,
		Title:       "Synthetic demo vulnerability",
		Description: "DEMO DATA: this advisory and package are synthetic.",
		Severity:    model.SeverityHigh,
		Package:     demoTarget(),
		Version:     &version,
		Evidence: []model.Evidence{
			{
				Kind:      model.EvidenceAdvisory,
				Summary:   "DEMO DATA: synthetic advisory match.",
				SourceID:  "src-demo-osv",
				Reference: "DEMO-ADVISORY-0001",
			},
		},
		Risk: &model.RiskAssessment{
			Score:     80,
			Rationale: "DEMO DATA: synthetic risk rationale.",
		},
	}
}

func heuristicFinding() model.Finding {
	return model.Finding{
		ID:          "finding-heuristic-1",
		Kind:        model.FindingHeuristic,
		Title:       "Synthetic demo heuristic warning",
		Description: "DEMO DATA: this warning is synthetic and has unknown severity.",
		Severity:    model.SeverityUnknown,
		Package:     demoTarget(),
		Version:     nil,
		Evidence: []model.Evidence{
			{
				Kind:     model.EvidenceHeuristic,
				Summary:  "DEMO DATA: synthetic heuristic signal.",
				SourceID: "src-demo-registry",
			},
		},
		Risk: nil,
	}
}

func TestScanReportJSONContractFields(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(validReport())
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	for _, key := range []string{
		"schemaVersion",
		"dataMode",
		"demoNotice",
		"scannedAt",
		"status",
		"target",
		"summary",
		"findings",
		"checks",
		"dependencies",
		"sources",
		"limitations",
	} {
		if _, ok := document[key]; !ok {
			t.Fatalf("serialized report is missing key %q", key)
		}
	}

	findings, ok := document["findings"].([]any)
	if !ok {
		t.Fatalf("findings type = %T, want array", document["findings"])
	}
	if len(findings) != 0 {
		t.Fatalf("findings length = %d, want 0", len(findings))
	}

	summary, ok := document["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary type = %T, want object", document["summary"])
	}
	if summary["highestSeverity"] != nil {
		t.Fatalf("summary.highestSeverity = %v, want null", summary["highestSeverity"])
	}

	target, ok := document["target"].(map[string]any)
	if !ok {
		t.Fatalf("target type = %T, want object", document["target"])
	}
	if target["resolvedVersion"] != "1.0.0" {
		t.Fatalf("target.resolvedVersion = %v, want 1.0.0", target["resolvedVersion"])
	}

	sources, ok := document["sources"].([]any)
	if !ok || len(sources) == 0 {
		t.Fatalf("sources = %T, want non-empty array", document["sources"])
	}
	source, ok := sources[0].(map[string]any)
	if !ok {
		t.Fatalf("sources[0] type = %T, want object", sources[0])
	}
	if _, exists := source["url"]; exists {
		t.Fatalf("sources[0] unexpectedly contains a synthesized url: %v", source["url"])
	}
}

func TestScanReportValidateAcceptsValidDemoReport(t *testing.T) {
	t.Parallel()

	if err := validReport().Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestScanReportValidateRejectsConfirmedFindingWithoutEvidence(t *testing.T) {
	t.Parallel()

	report := validReport()
	finding := confirmedFinding()
	finding.Evidence = []model.Evidence{}
	report.Findings = []model.Finding{finding}
	report.Status = model.StatusFindings
	report.Summary = model.Summary{
		ConfirmedFindings: 1,
		HeuristicWarnings: 0,
		HighestSeverity:   ptr(model.SeverityHigh),
	}

	err := report.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing-evidence error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "evidence") {
		t.Fatalf("Validate() error = %q, want evidence error", err)
	}
}

func TestScanReportValidateRejectsConfirmedFindingWithoutVersion(t *testing.T) {
	t.Parallel()

	report := validReport()
	finding := confirmedFinding()
	finding.Version = nil
	report.Findings = []model.Finding{finding}
	report.Status = model.StatusFindings
	report.Summary = model.Summary{
		ConfirmedFindings: 1,
		HighestSeverity:   ptr(model.SeverityHigh),
	}

	err := report.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want resolved-version error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "version") {
		t.Fatalf("Validate() error = %q, want version error", err)
	}
}

func TestScanReportAcceptsUnknownSeverityHeuristicWarning(t *testing.T) {
	t.Parallel()

	report := validReport()
	report.Findings = []model.Finding{heuristicFinding()}
	report.Status = model.StatusFindings
	report.Summary = model.Summary{
		ConfirmedFindings: 0,
		HeuristicWarnings: 1,
		HighestSeverity:   ptr(model.SeverityUnknown),
	}

	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if report.Status == model.StatusClean {
		t.Fatal("a report with an unknown-severity warning must not be clean")
	}
	if got := model.DeriveFinalStatus(report.Checks, report.Findings); got != model.StatusFindings {
		t.Fatalf("DeriveFinalStatus() = %q, want %q", got, model.StatusFindings)
	}
}

func TestScanReportValidateRejectsCleanStatusWithIncompleteChecks(t *testing.T) {
	t.Parallel()

	report := validReport()
	report.Checks[2].Status = model.CheckUnavailable
	report.Checks[2].Message = "DEMO DATA: vulnerability source unavailable."

	err := report.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want status error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "status") {
		t.Fatalf("Validate() error = %q, want status error", err)
	}
	if got := model.DeriveFinalStatus(report.Checks, report.Findings); got != model.StatusIncomplete {
		t.Fatalf("DeriveFinalStatus() = %q, want %q", got, model.StatusIncomplete)
	}
}

func TestScanReportValidateRejectsCleanStatusWithNoChecks(t *testing.T) {
	t.Parallel()

	report := validReport()
	report.Checks = []model.CheckResult{}

	if err := report.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want incomplete-status error")
	}
}

func TestScanReportValidateRejectsDanglingEvidenceSource(t *testing.T) {
	t.Parallel()

	report := validReport()
	finding := confirmedFinding()
	finding.Evidence[0].SourceID = "src-missing"
	report.Findings = []model.Finding{finding}
	report.Status = model.StatusFindings
	report.Summary = model.Summary{
		ConfirmedFindings: 1,
		HighestSeverity:   ptr(model.SeverityHigh),
	}

	err := report.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want dangling-source error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "source") {
		t.Fatalf("Validate() error = %q, want source error", err)
	}
}

func TestScanReportValidateRejectsSummaryMismatch(t *testing.T) {
	t.Parallel()

	report := validReport()
	report.Summary.ConfirmedFindings = 1

	err := report.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want summary error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "summary") {
		t.Fatalf("Validate() error = %q, want summary error", err)
	}
}

func TestScanReportValidateRejectsDuplicateSourceIDs(t *testing.T) {
	t.Parallel()

	report := validReport()
	report.Sources = append(report.Sources, report.Sources[0])

	err := report.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want duplicate-id error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Fatalf("Validate() error = %q, want duplicate error", err)
	}
}

func TestScanReportDemoRules(t *testing.T) {
	t.Parallel()

	t.Run("demo requires notice", func(t *testing.T) {
		t.Parallel()

		report := validReport()
		report.DemoNotice = nil
		if err := report.Validate(); err == nil || !strings.Contains(strings.ToLower(err.Error()), "demonotice") {
			t.Fatalf("Validate() error = %v, want demoNotice error", err)
		}
	})

	t.Run("live forbids notice", func(t *testing.T) {
		t.Parallel()

		report := validReport()
		report.DataMode = model.DataModeLive
		report.Sources = nil
		if err := report.Validate(); err == nil || !strings.Contains(strings.ToLower(err.Error()), "demonotice") {
			t.Fatalf("Validate() error = %v, want demoNotice error", err)
		}
	})

	t.Run("demo forbids real providers", func(t *testing.T) {
		t.Parallel()

		report := validReport()
		report.Sources[0].Provider = model.SourceProviderOSV
		if err := report.Validate(); err == nil || !strings.Contains(strings.ToLower(err.Error()), "demo") {
			t.Fatalf("Validate() error = %v, want demo-provider error", err)
		}
	})

	t.Run("live forbids demo providers", func(t *testing.T) {
		t.Parallel()

		report := validReport()
		report.DataMode = model.DataModeLive
		report.DemoNotice = nil
		if err := report.Validate(); err == nil || !strings.Contains(strings.ToLower(err.Error()), "demo") {
			t.Fatalf("Validate() error = %v, want demo-provider error", err)
		}
	})
}

func TestScanReportValidateRejectsInvalidDependencyPath(t *testing.T) {
	t.Parallel()

	report := validReport()
	report.Dependencies.Nodes[0].DependencyPath = []string{"missing"}

	err := report.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want dependency-path error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "path") {
		t.Fatalf("Validate() error = %q, want path error", err)
	}
}

func TestScanReportValidateRejectsInvalidRiskScore(t *testing.T) {
	t.Parallel()

	report := validReport()
	finding := confirmedFinding()
	finding.Risk.Score = 101
	report.Findings = []model.Finding{finding}
	report.Status = model.StatusFindings
	report.Summary = model.Summary{
		ConfirmedFindings: 1,
		HighestSeverity:   ptr(model.SeverityHigh),
	}

	err := report.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want risk-score error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "risk") {
		t.Fatalf("Validate() error = %q, want risk error", err)
	}
}

func TestScanReportValidateRejectsNilCollections(t *testing.T) {
	t.Parallel()

	report := validReport()
	report.Findings = nil
	if err := report.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want nil-collection error")
	}
}
