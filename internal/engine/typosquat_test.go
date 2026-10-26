package engine_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/model"
)

const referenceDataset = `{
  "datasetVersion": "test-1",
  "updatedAt": "2026-10-25T00:00:00Z",
  "provenance": "Curated test seed, not a download ranking.",
  "updateInstructions": "Review names against the official registry before updating.",
  "ecosystems": {
    "npm": ["react", "express", "lodash"],
    "pypi": ["requests", "numpy"]
  }
}`

func newChecker(t *testing.T) *engine.TyposquatChecker {
	t.Helper()
	checker, err := engine.NewTyposquatChecker([]byte(referenceDataset))
	if err != nil {
		t.Fatalf("NewTyposquatChecker: %v", err)
	}
	return checker
}

func TestTyposquatDetectsTransposition(t *testing.T) {
	checker := newChecker(t)
	result, err := checker.Check(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "raect"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.Matched || result.Reference != "react" || result.Distance != 1 {
		t.Fatalf("result = %+v", result)
	}
	if result.DatasetVersion != "test-1" {
		t.Fatalf("DatasetVersion = %q", result.DatasetVersion)
	}
	wantTime := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	if !result.DatasetUpdatedAt.Equal(wantTime) {
		t.Fatalf("DatasetUpdatedAt = %v", result.DatasetUpdatedAt)
	}
}

func TestTyposquatExactNameIsNotFlagged(t *testing.T) {
	checker := newChecker(t)
	result, err := checker.Check(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "react"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if result.Matched {
		t.Fatalf("result = %+v, want no match", result)
	}
}

func TestTyposquatDistantNameIsNotFlagged(t *testing.T) {
	checker := newChecker(t)
	result, err := checker.Check(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "completely-different"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if result.Matched {
		t.Fatalf("result = %+v, want no match", result)
	}
}

func TestTyposquatPyPINormalization(t *testing.T) {
	checker := newChecker(t)
	result, err := checker.Check(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "requestz"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.Matched || result.Reference != "requests" {
		t.Fatalf("result = %+v", result)
	}
}

func TestTyposquatSkipsScopedNPMPackages(t *testing.T) {
	checker := newChecker(t)
	result, err := checker.Check(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "@scope/react"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if result.Matched {
		t.Fatalf("result = %+v, want no match", result)
	}
}

func TestTyposquatReferenceUnavailable(t *testing.T) {
	checker, err := engine.NewTyposquatChecker([]byte(`{
      "datasetVersion": "test-1",
      "updatedAt": "2026-10-25T00:00:00Z",
      "provenance": "test",
      "updateInstructions": "test",
      "ecosystems": {"npm": [], "pypi": []}
    }`))
	if err != nil {
		t.Fatalf("NewTyposquatChecker: %v", err)
	}
	_, err = checker.Check(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "raect"})
	if !errors.Is(err, engine.ErrReferenceUnavailable) {
		t.Fatalf("error = %v, want ErrReferenceUnavailable", err)
	}
}

func TestTyposquatDatasetRequiresProvenance(t *testing.T) {
	_, err := engine.NewTyposquatChecker([]byte(`{
      "datasetVersion": "test-1",
      "updatedAt": "2026-10-25T00:00:00Z",
      "updateInstructions": "test",
      "ecosystems": {"npm": ["react"]}
    }`))
	if err == nil {
		t.Fatal("expected an error for missing provenance")
	}
}

func TestTyposquatUsesClosestDeterministicMatch(t *testing.T) {
	checker, err := engine.NewTyposquatChecker([]byte(`{
      "datasetVersion": "test-1",
      "updatedAt": "2026-10-25T00:00:00Z",
      "provenance": "test",
      "updateInstructions": "test",
      "ecosystems": {"npm": ["react", "reac"], "pypi": []}
    }`))
	if err != nil {
		t.Fatalf("NewTyposquatChecker: %v", err)
	}
	result, err := checker.Check(context.Background(), model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "reactt"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.Matched || result.Reference != "react" || result.Distance != 1 {
		t.Fatalf("result = %+v", result)
	}
}
