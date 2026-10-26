package engine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/model"
)

func TestConfirmedAdvisoryRiskScore(t *testing.T) {
	version := "1.0.0"
	finding := model.Finding{
		ID:       "ADVISORY-1",
		Kind:     model.FindingConfirmed,
		Title:    "Known vulnerability",
		Severity: model.SeverityHigh,
		Package:  model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "example"},
		Version:  &version,
		Evidence: []model.Evidence{{
			Kind:      model.EvidenceAdvisory,
			Summary:   "OSV advisory",
			SourceID:  "osv",
			Reference: "https://osv.dev/vulnerability/OSV-1",
		}},
	}
	risk, err := engine.RulesRiskEvaluator{}.Evaluate(context.Background(), engine.RiskInput{Finding: finding})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if risk.Score != 80 {
		t.Fatalf("Score = %v, want 80", risk.Score)
	}
	if !strings.Contains(risk.Rationale, "not a probability") {
		t.Fatalf("Rationale = %q", risk.Rationale)
	}
	if !strings.Contains(risk.Rationale, "confirmed") {
		t.Fatalf("Rationale = %q", risk.Rationale)
	}
}

func TestHeuristicRiskScoreIsLowerThanConfirmed(t *testing.T) {
	version := "1.0.0"
	base := model.Finding{
		Kind:     model.FindingHeuristic,
		Title:    "Lifecycle script",
		Severity: model.SeverityLow,
		Package:  model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "example"},
		Version:  &version,
		Evidence: []model.Evidence{{
			Kind:     model.EvidenceRegistry,
			Summary:  "install script declared",
			SourceID: "npm-registry",
		}},
	}
	risk, err := engine.RulesRiskEvaluator{}.Evaluate(context.Background(), engine.RiskInput{Finding: base})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if risk.Score != 15 {
		t.Fatalf("heuristic Score = %v, want 15", risk.Score)
	}
	if !strings.Contains(risk.Rationale, "not proof") {
		t.Fatalf("Rationale = %q", risk.Rationale)
	}
}

func TestUnknownConfirmedSeverity(t *testing.T) {
	version := "1.0.0"
	finding := model.Finding{
		Kind:     model.FindingConfirmed,
		Title:    "Unrated advisory",
		Severity: model.SeverityUnknown,
		Package:  model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "example"},
		Version:  &version,
		Evidence: []model.Evidence{{Kind: model.EvidenceAdvisory, Summary: "advisory", SourceID: "osv", Reference: "https://osv.dev/vulnerability/OSV-1"}},
	}
	risk, err := engine.RulesRiskEvaluator{}.Evaluate(context.Background(), engine.RiskInput{Finding: finding})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if risk.Score != 45 {
		t.Fatalf("Score = %v, want 45", risk.Score)
	}
}

func TestRiskEvaluationHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := engine.RulesRiskEvaluator{}.Evaluate(ctx, engine.RiskInput{})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}
