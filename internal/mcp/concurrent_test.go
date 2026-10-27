// Package mcp 的高并发缓存与数据竞争稳定性测试。
package mcp

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
)

func TestScanCacheConcurrentAccess(t *testing.T) {
	cache := NewScanCache(20)

	var wg sync.WaitGroup
	workers := 10
	iterations := 50

	for w := 0; w < workers; w++ {
		workerID := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				// 1. 并发写入 Package 扫描
				pkgName := fmt.Sprintf("pkg-w%d-%d", workerID, i)
				v := "1.0.0"
				cache.PutPackage(model.ScanReport{
					ScannedAt: time.Now(),
					Status:    model.StatusFindings,
					Target: model.ScanTarget{
						Package:         model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: pkgName},
						ResolvedVersion: &v,
					},
					Findings: []model.Finding{
						{
							ID:       fmt.Sprintf("F-PKG-%d-%d", workerID, i),
							Kind:     model.FindingConfirmed,
							Severity: model.SeverityHigh,
							Package:  model.PackageIdentity{Ecosystem: model.EcosystemNPM, Name: pkgName},
							Version:  &v,
						},
					},
				})

				// 2. 并发写入 Project 扫描
				projRoot := fmt.Sprintf("/project/w%d-%d", workerID, i)
				cache.PutProject(project.Result{
					Root:      projRoot,
					ScannedAt: time.Now(),
					Status:    model.StatusClean,
					Findings:  []model.Finding{},
				})

				// 3. 并发读取与查询
				_, _ = cache.Latest()
				_, _, _ = cache.FindFindingAcrossScans("", fmt.Sprintf("F-PKG-%d-%d", workerID, i))
			}
		}()
	}

	wg.Wait()

	// 验证容量保持在有界上限以内
	if len(cache.order) > 20 || len(cache.scans) > 20 {
		t.Fatalf("cache exceeded capacity: order=%d, scans=%d", len(cache.order), len(cache.scans))
	}
}
