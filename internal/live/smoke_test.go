//go:build live

package live_test

import (
	"context"
	"os"
	"testing"
	"time"

	"depscope/internal/contract"
	"depscope/internal/engine"
	"depscope/internal/live"
	"depscope/internal/model"
)

// TestLiveSmokeNPM 访问真实 npm 与 OSV；仅在显式设置环境变量时运行。
func TestLiveSmokeNPM(t *testing.T) {
	if os.Getenv("DEPSCOPE_LIVE_TESTS") != "1" {
		t.Skip("set DEPSCOPE_LIVE_TESTS=1 to run live network smoke tests")
	}
	scanner, err := live.NewScanner(live.Options{})
	if err != nil {
		t.Fatalf("NewScanner: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pkg, err := model.NewPackageIdentity(model.EcosystemNPM, "is-number")
	if err != nil {
		t.Fatalf("NewPackageIdentity: %v", err)
	}
	report, err := scanner.Scan(ctx, engine.ScanRequest{
		Target:   model.ScanTarget{Package: pkg},
		DataMode: model.DataModeLive,
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.Target.ResolvedVersion == nil {
		t.Fatalf("ResolvedVersion = nil, report = %+v", report)
	}
	if report.Status != model.StatusClean && report.Status != model.StatusFindings && report.Status != model.StatusIncomplete {
		t.Fatalf("Status = %q, want a completed live result", report.Status)
	}
	if len(report.Sources) == 0 {
		t.Fatal("live report contains no source references")
	}
	if err := contract.ValidateReport(report); err != nil {
		t.Fatalf("report is not valid: %v", err)
	}
}
