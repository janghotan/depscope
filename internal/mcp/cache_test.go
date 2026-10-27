// Package mcp 的扫描会话缓存测试。
package mcp

import (
	"fmt"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
)

func TestScanCacheBoundedCapacity(t *testing.T) {
	cache := NewScanCache(3)

	for i := 1; i <= 5; i++ {
		pkgName := fmt.Sprintf("test-pkg-%d", i)
		cache.PutPackage(model.ScanReport{
			ScannedAt: time.Now(),
			Status:    model.StatusClean,
			Target: model.ScanTarget{
				Package: model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: pkgName},
			},
		})
	}

	if len(cache.order) != 3 {
		t.Fatalf("cache order len = %d, want 3", len(cache.order))
	}
	if len(cache.scans) != 3 {
		t.Fatalf("cache scans len = %d, want 3", len(cache.scans))
	}

	latest, ok := cache.Latest()
	if !ok || latest.Target != "npm:test-pkg-5" {
		t.Fatalf("latest = %+v, want npm:test-pkg-5", latest)
	}
}

func TestScanCacheFindFinding(t *testing.T) {
	cache := NewScanCache(10)

	version := "1.0.0"
	scan1 := cache.PutProject(project.Result{
		Root:      "/test/project1",
		ScannedAt: time.Now(),
		Status:    model.StatusFindings,
		Findings: []model.Finding{
			{
				ID:       "FINDING-1",
				Kind:     model.FindingConfirmed,
				Title:    "Prototype Pollution",
				Severity: model.SeverityCritical,
				Package:  model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: "lodash"},
				Version:  &version,
			},
		},
		FindingContexts: map[string]project.DependencyContext{
			"FINDING-1": {
				Relationship: model.DependencyDirect,
				Scopes:       []project.DependencyScope{project.DependencyScopeProduction},
				Resolution:   project.DependencyResolutionComplete,
			},
		},
	})

	scan2 := cache.PutPackage(model.ScanReport{
		ScannedAt: time.Now(),
		Status:    model.StatusFindings,
		Target: model.ScanTarget{
			Package:         model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "requests"},
			ResolvedVersion: &version,
		},
		Findings: []model.Finding{
			{
				ID:       "FINDING-2",
				Kind:     model.FindingConfirmed,
				Title:    "CVE-2023-XXXXX",
				Severity: model.SeverityHigh,
				Package:  model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: "requests"},
				Version:  &version,
			},
		},
	})

	// 1. 在特定 scanID 中查找
	foundScan, finding, ok := cache.FindFindingAcrossScans(scan1.ID, "FINDING-1")
	if !ok || foundScan.ID != scan1.ID || finding.ID != "FINDING-1" {
		t.Fatalf("find FINDING-1 in scan1 failed: ok=%v, scan=%+v", ok, foundScan)
	}

	// 2. 跨扫描查找（不指定 scanID）
	foundScan2, finding2, ok2 := cache.FindFindingAcrossScans("", "FINDING-2")
	if !ok2 || foundScan2.ID != scan2.ID || finding2.ID != "FINDING-2" {
		t.Fatalf("find FINDING-2 across scans failed: ok=%v, scan=%+v", ok2, foundScan2)
	}

	// 3. 查找不存在的 finding
	_, _, ok3 := cache.FindFindingAcrossScans("", "NONEXISTENT")
	if ok3 {
		t.Fatal("expected false for nonexistent finding")
	}
}
