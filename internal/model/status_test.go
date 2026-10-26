package model_test

import (
	"testing"

	"github.com/janghotan/depscope/internal/model"
)

func TestDeriveFinalStatusNeverTreatsMissingChecksAsClean(t *testing.T) {
	t.Parallel()

	confirmed := model.Finding{Kind: model.FindingConfirmed, Severity: model.SeverityHigh}
	heuristic := model.Finding{Kind: model.FindingHeuristic, Severity: model.SeverityUnknown}

	tests := []struct {
		name     string
		checks   []model.CheckResult
		findings []model.Finding
		want     model.ScanStatus
	}{
		{
			name: "no checks",
			want: model.StatusIncomplete,
		},
		{
			name: "all checks succeeded without findings",
			checks: []model.CheckResult{
				{Status: model.CheckSucceeded},
			},
			want: model.StatusClean,
		},
		{
			name: "all checks succeeded with confirmed finding",
			checks: []model.CheckResult{
				{Status: model.CheckSucceeded},
			},
			findings: []model.Finding{confirmed},
			want:     model.StatusFindings,
		},
		{
			name: "all checks succeeded with heuristic warning",
			checks: []model.CheckResult{
				{Status: model.CheckSucceeded},
			},
			findings: []model.Finding{heuristic},
			want:     model.StatusFindings,
		},
		{
			name: "unavailable check",
			checks: []model.CheckResult{
				{Status: model.CheckSucceeded},
				{Status: model.CheckUnavailable},
			},
			want: model.StatusIncomplete,
		},
		{
			name: "skipped check",
			checks: []model.CheckResult{
				{Status: model.CheckSkipped},
			},
			want: model.StatusIncomplete,
		},
		{
			name: "failed check",
			checks: []model.CheckResult{
				{Status: model.CheckFailed},
			},
			want: model.StatusIncomplete,
		},
		{
			name: "incomplete check with finding stays incomplete",
			checks: []model.CheckResult{
				{Status: model.CheckSucceeded},
				{Status: model.CheckUnavailable},
			},
			findings: []model.Finding{confirmed},
			want:     model.StatusIncomplete,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := model.DeriveFinalStatus(tc.checks, tc.findings); got != tc.want {
				t.Fatalf("DeriveFinalStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHighestSeverity(t *testing.T) {
	t.Parallel()

	if got := model.HighestSeverity(nil); got != nil {
		t.Fatalf("HighestSeverity(nil) = %v, want nil", got)
	}

	findings := []model.Finding{
		{Severity: model.SeverityLow},
		{Severity: model.SeverityUnknown},
		{Severity: model.SeverityCritical},
		{Severity: model.SeverityMedium},
	}
	got := model.HighestSeverity(findings)
	if got == nil || *got != model.SeverityCritical {
		t.Fatalf("HighestSeverity() = %v, want %q", got, model.SeverityCritical)
	}

	unknownOnly := model.HighestSeverity([]model.Finding{{Severity: model.SeverityUnknown}})
	if unknownOnly == nil || *unknownOnly != model.SeverityUnknown {
		t.Fatalf("HighestSeverity(unknown) = %v, want %q", unknownOnly, model.SeverityUnknown)
	}
}

func TestScanStatusTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from model.ScanStatus
		to   model.ScanStatus
		want bool
	}{
		{from: model.StatusPending, to: model.StatusRunning, want: true},
		{from: model.StatusPending, to: model.StatusFailed, want: true},
		{from: model.StatusPending, to: model.StatusCancelled, want: true},
		{from: model.StatusPending, to: model.StatusClean, want: false},
		{from: model.StatusRunning, to: model.StatusClean, want: true},
		{from: model.StatusRunning, to: model.StatusFindings, want: true},
		{from: model.StatusRunning, to: model.StatusIncomplete, want: true},
		{from: model.StatusRunning, to: model.StatusFailed, want: true},
		{from: model.StatusRunning, to: model.StatusCancelled, want: true},
		{from: model.StatusRunning, to: model.StatusPending, want: false},
		{from: model.StatusClean, to: model.StatusRunning, want: false},
		{from: model.StatusFindings, to: model.StatusClean, want: false},
		{from: model.ScanStatus("bogus"), to: model.StatusRunning, want: false},
	}

	for _, tc := range tests {
		if got := tc.from.CanTransitionTo(tc.to); got != tc.want {
			t.Fatalf("%q.CanTransitionTo(%q) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestScanStatusFinality(t *testing.T) {
	t.Parallel()

	final := []model.ScanStatus{
		model.StatusClean,
		model.StatusFindings,
		model.StatusIncomplete,
		model.StatusFailed,
		model.StatusCancelled,
	}
	for _, status := range final {
		if !status.IsFinal() {
			t.Fatalf("%q.IsFinal() = false, want true", status)
		}
	}
	if model.StatusPending.IsFinal() || model.StatusRunning.IsFinal() {
		t.Fatal("pending and running must not be final")
	}
}
