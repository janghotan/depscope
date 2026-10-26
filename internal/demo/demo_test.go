package demo_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"depscope/internal/contract"
	"depscope/internal/demo"
	"depscope/internal/model"
)

func TestScenariosAreDeterministicAndContractValid(t *testing.T) {
	for _, scenario := range demo.Scenarios() {
		scenario := scenario
		t.Run(scenario.ID, func(t *testing.T) {
			first, err := demo.Scan(context.Background(), scenario.ID)
			if err != nil {
				t.Fatalf("first scan: %v", err)
			}
			second, err := demo.Scan(context.Background(), scenario.ID)
			if err != nil {
				t.Fatalf("second scan: %v", err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatal("demo report is not deterministic")
			}
			if first.DataMode != model.DataModeDemo {
				t.Fatalf("dataMode = %q, want demo", first.DataMode)
			}
			if first.DemoNotice == nil || !strings.Contains(*first.DemoNotice, "DEMO DATA") {
				t.Fatalf("demo notice = %v", first.DemoNotice)
			}
			if first.ScannedAt.Format("2006-01-02T15:04:05Z07:00") != demo.DataTime {
				t.Fatalf("scannedAt = %s, want %s", first.ScannedAt, demo.DataTime)
			}
			if len(first.Sources) == 0 {
				t.Fatal("demo report has no sources")
			}
			for _, source := range first.Sources {
				if source.Provider != model.SourceProviderDemo {
					t.Fatalf("source %q provider = %q, want demo", source.ID, source.Provider)
				}
				if source.URL != nil {
					t.Fatalf("source %q has a URL in demo mode", source.ID)
				}
				if source.RetrievedAt != first.ScannedAt {
					t.Fatalf("source %q retrievedAt = %s, want fixed time", source.ID, source.RetrievedAt)
				}
			}
			for _, finding := range first.Findings {
				for _, evidence := range finding.Evidence {
					if strings.HasPrefix(evidence.Reference, "http://") || strings.HasPrefix(evidence.Reference, "https://") {
						t.Fatalf("finding %q contains fabricated URL %q", finding.ID, evidence.Reference)
					}
				}
			}
			if err := contract.ValidateReport(first); err != nil {
				t.Fatalf("demo report is not contract-valid: %v", err)
			}
		})
	}
}

func TestScenarioCoverage(t *testing.T) {
	vulnerable, err := demo.Scan(context.Background(), "vulnerable")
	if err != nil {
		t.Fatalf("vulnerable scan: %v", err)
	}
	if vulnerable.Status != model.StatusFindings {
		t.Fatalf("vulnerable status = %q, want findings", vulnerable.Status)
	}
	if vulnerable.Summary.ConfirmedFindings != 2 {
		t.Fatalf("confirmed findings = %d, want 2", vulnerable.Summary.ConfirmedFindings)
	}
	if vulnerable.Summary.HighestSeverity == nil || *vulnerable.Summary.HighestSeverity != model.SeverityCritical {
		t.Fatalf("highest severity = %v, want critical", vulnerable.Summary.HighestSeverity)
	}
	if !allChecksSucceeded(vulnerable) {
		t.Fatal("vulnerable scenario should have complete successful checks")
	}

	scripts, err := demo.Scan(context.Background(), "install-scripts")
	if err != nil {
		t.Fatalf("install-scripts scan: %v", err)
	}
	if scripts.Status != model.StatusFindings {
		t.Fatalf("install-scripts status = %q, want findings", scripts.Status)
	}
	if !hasFindingPrefix(scripts, "LIFECYCLE-") {
		t.Fatal("install-scripts scenario has no lifecycle warning")
	}
	if scripts.Summary.ConfirmedFindings != 0 {
		t.Fatalf("install-scripts confirmed findings = %d, want 0", scripts.Summary.ConfirmedFindings)
	}

	clean, err := demo.Scan(context.Background(), "clean")
	if err != nil {
		t.Fatalf("clean scan: %v", err)
	}
	if clean.Status != model.StatusClean {
		t.Fatalf("clean status = %q, want clean", clean.Status)
	}
	if len(clean.Findings) != 0 {
		t.Fatalf("clean findings = %d, want 0", len(clean.Findings))
	}
	if !allChecksSucceeded(clean) {
		t.Fatal("clean scenario should have complete successful checks")
	}
}

func TestUnknownScenario(t *testing.T) {
	_, err := demo.Scan(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected unknown scenario error")
	}
	if !strings.Contains(err.Error(), "vulnerable") {
		t.Fatalf("error should list available scenarios: %v", err)
	}
}

func TestCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := demo.Scan(ctx, "clean")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func allChecksSucceeded(report model.ScanReport) bool {
	for _, check := range report.Checks {
		if check.Status != model.CheckSucceeded {
			return false
		}
	}
	return len(report.Checks) > 0
}

func hasFindingPrefix(report model.ScanReport, prefix string) bool {
	for _, finding := range report.Findings {
		if strings.HasPrefix(finding.ID, prefix) {
			return true
		}
	}
	return false
}
