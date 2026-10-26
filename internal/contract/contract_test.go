package contract_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janghotan/depscope/internal/contract"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/schemas"
)

func fixtureDirectory() string {
	return filepath.Join("..", "..", "testdata", "fixtures", "demo")
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(fixtureDirectory(), name))
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", name, err)
	}
	return data
}

func loadFixtureReport(t *testing.T, name string) model.ScanReport {
	t.Helper()

	var report model.ScanReport
	if err := json.Unmarshal(loadFixture(t, name), &report); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", name, err)
	}
	return report
}

func TestSchemaVersionMatchesModel(t *testing.T) {
	t.Parallel()

	var document map[string]any
	if err := json.Unmarshal(schemas.ReportV1(), &document); err != nil {
		t.Fatalf("json.Unmarshal(schema) error = %v", err)
	}
	properties, ok := document["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties type = %T, want object", document["properties"])
	}
	versionProperty, ok := properties["schemaVersion"].(map[string]any)
	if !ok {
		t.Fatalf("schemaVersion property type = %T, want object", properties["schemaVersion"])
	}
	if got := versionProperty["const"]; got != model.SchemaVersion {
		t.Fatalf("schema const = %v, want %q", got, model.SchemaVersion)
	}
}

func TestSchemaIDMatchesConstant(t *testing.T) {
	t.Parallel()

	var document map[string]any
	if err := json.Unmarshal(schemas.ReportV1(), &document); err != nil {
		t.Fatalf("json.Unmarshal(schema) error = %v", err)
	}
	if got := document["$id"]; got != schemas.ReportV1ID {
		t.Fatalf("schema $id = %v, want %q", got, schemas.ReportV1ID)
	}
}

func TestValidateJSONRejectsUnknownField(t *testing.T) {
	t.Parallel()

	var document map[string]any
	if err := json.Unmarshal(loadFixture(t, "demo-clean-report.json"), &document); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	document["unexpectedField"] = true
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if err := contract.ValidateJSON(data); err == nil {
		t.Fatal("ValidateJSON() error = nil, want unknown-field error")
	}
}

func TestValidateJSONRejectsMissingRequiredField(t *testing.T) {
	t.Parallel()

	var document map[string]any
	if err := json.Unmarshal(loadFixture(t, "demo-clean-report.json"), &document); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	delete(document, "status")
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if err := contract.ValidateJSON(data); err == nil {
		t.Fatal("ValidateJSON() error = nil, want missing-field error")
	}
}

func TestValidateJSONEnforcesCleanStatusInvariant(t *testing.T) {
	t.Parallel()

	var document map[string]any
	if err := json.Unmarshal(loadFixture(t, "demo-clean-report.json"), &document); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	checks := document["checks"].([]any)
	checks[0].(map[string]any)["status"] = "skipped"
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if err := contract.ValidateJSON(data); err == nil {
		t.Fatal("ValidateJSON() error = nil, want clean-status invariant error")
	}
}

func TestValidateJSONRejectsIncompleteWithOnlySuccessfulChecks(t *testing.T) {
	t.Parallel()

	var document map[string]any
	if err := json.Unmarshal(loadFixture(t, "demo-clean-report.json"), &document); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	document["status"] = "incomplete"
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if err := contract.ValidateJSON(data); err == nil {
		t.Fatal("ValidateJSON() error = nil, want incomplete-status invariant error")
	}
}

func TestValidateJSONEnforcesDemoLabelAndSources(t *testing.T) {
	t.Parallel()

	t.Run("demo notice required", func(t *testing.T) {
		t.Parallel()

		var document map[string]any
		if err := json.Unmarshal(loadFixture(t, "demo-clean-report.json"), &document); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		document["demoNotice"] = nil
		data, err := json.Marshal(document)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if err := contract.ValidateJSON(data); err == nil {
			t.Fatal("ValidateJSON() error = nil, want demoNotice error")
		}
	})

	t.Run("demo sources only", func(t *testing.T) {
		t.Parallel()

		var document map[string]any
		if err := json.Unmarshal(loadFixture(t, "demo-clean-report.json"), &document); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		document["sources"].([]any)[0].(map[string]any)["provider"] = "osv"
		data, err := json.Marshal(document)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if err := contract.ValidateJSON(data); err == nil {
			t.Fatal("ValidateJSON() error = nil, want demo-provider error")
		}
	})
}

func TestValidateJSONRequiresAdvisoryEvidenceForConfirmedFinding(t *testing.T) {
	t.Parallel()

	var document map[string]any
	if err := json.Unmarshal(loadFixture(t, "demo-findings-report.json"), &document); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	finding := document["findings"].([]any)[0].(map[string]any)
	finding["evidence"] = []any{
		map[string]any{
			"kind":     "heuristic",
			"summary":  "DEMO DATA: only heuristic evidence remains.",
			"sourceId": "demo-source-registry",
		},
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if err := contract.ValidateJSON(data); err == nil {
		t.Fatal("ValidateJSON() error = nil, want advisory-evidence error")
	}
}

func TestValidateJSONRejectsNonFinalStatus(t *testing.T) {
	t.Parallel()

	var document map[string]any
	if err := json.Unmarshal(loadFixture(t, "demo-clean-report.json"), &document); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	document["status"] = model.StatusRunning
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if err := contract.ValidateJSON(data); err == nil {
		t.Fatal("ValidateJSON() error = nil, want non-final-status error")
	}
}

func TestValidateReportAcceptsDemoFixtures(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"demo-clean-report.json",
		"demo-findings-report.json",
		"demo-incomplete-report.json",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			report := loadFixtureReport(t, name)
			if err := contract.ValidateReport(report); err != nil {
				t.Fatalf("ValidateReport() error = %v", err)
			}
			if report.Status == model.StatusClean && name != "demo-clean-report.json" {
				t.Fatalf("unexpected clean status for %s", name)
			}
		})
	}
}

func TestValidateReportRejectsMissingEvidence(t *testing.T) {
	t.Parallel()

	report := loadFixtureReport(t, "demo-findings-report.json")
	if len(report.Findings) == 0 {
		t.Fatal("fixture must contain at least one finding")
	}
	report.Findings[0].Evidence = []model.Evidence{}

	if err := contract.ValidateReport(report); err == nil {
		t.Fatal("ValidateReport() error = nil, want missing-evidence error")
	}
}

func TestValidateReportAcceptsUnknownSeverity(t *testing.T) {
	t.Parallel()

	report := loadFixtureReport(t, "demo-findings-report.json")
	report.Findings = []model.Finding{{
		ID:          "demo-finding-unknown",
		Kind:        model.FindingHeuristic,
		Title:       "DEMO DATA: unknown severity",
		Description: "Synthetic warning with unknown severity.",
		Severity:    model.SeverityUnknown,
		Package:     report.Target.Package,
		Version:     nil,
		Evidence: []model.Evidence{{
			Kind:     model.EvidenceHeuristic,
			Summary:  "DEMO DATA: synthetic heuristic signal.",
			SourceID: report.Sources[0].ID,
		}},
	}}
	report.Summary = model.Summary{
		ConfirmedFindings: 0,
		HeuristicWarnings: 1,
		HighestSeverity:   pointer(model.SeverityUnknown),
	}
	report.Status = model.StatusFindings

	if err := contract.ValidateReport(report); err != nil {
		t.Fatalf("ValidateReport() error = %v", err)
	}
}

func TestDemoFixturesAreDeterministicAndClearlyLabelled(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(fixtureDirectory())
	if err != nil {
		t.Fatalf("os.ReadDir() error = %v", err)
	}

	fixtureCount := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		fixtureCount++

		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			report := loadFixtureReport(t, name)
			if err := contract.ValidateReport(report); err != nil {
				t.Fatalf("ValidateReport() error = %v", err)
			}
			if report.DataMode != model.DataModeDemo {
				t.Fatalf("dataMode = %q, want demo", report.DataMode)
			}
			if report.DemoNotice == nil || !strings.Contains(strings.ToUpper(*report.DemoNotice), "DEMO DATA") {
				t.Fatalf("demoNotice = %v, want unmistakable DEMO DATA label", report.DemoNotice)
			}
			if !strings.Contains(report.Target.Package.Name, "depscope-demo") {
				t.Fatalf("target package %q is not clearly synthetic", report.Target.Package.Name)
			}
			for _, source := range report.Sources {
				if source.Provider != model.SourceProviderDemo {
					t.Fatalf("source %q provider = %q, want demo", source.ID, source.Provider)
				}
				if source.URL != nil {
					t.Fatalf("source %q must not fabricate a URL", source.ID)
				}
				if !strings.HasPrefix(source.Identifier, "DEMO-") {
					t.Fatalf("source %q identifier = %q, want DEMO- prefix", source.ID, source.Identifier)
				}
			}

			first, err := json.Marshal(report)
			if err != nil {
				t.Fatalf("first json.Marshal() error = %v", err)
			}
			second, err := json.Marshal(report)
			if err != nil {
				t.Fatalf("second json.Marshal() error = %v", err)
			}
			if !bytes.Equal(first, second) {
				t.Fatal("fixture serialization is not deterministic")
			}
		})
	}

	if fixtureCount < 3 {
		t.Fatalf("fixture count = %d, want at least 3", fixtureCount)
	}
}

func TestValidateReportRejectsRunningStatus(t *testing.T) {
	t.Parallel()

	report := loadFixtureReport(t, "demo-clean-report.json")
	report.Status = model.StatusRunning
	if err := contract.ValidateReport(report); err == nil {
		t.Fatal("ValidateReport() error = nil, want non-final-status error")
	}
}

func pointer[T any](value T) *T {
	return &value
}
