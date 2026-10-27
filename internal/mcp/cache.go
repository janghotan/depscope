// Package mcp 提供会话内扫描结果缓存，供后续工具查询发现详情或汇总数据。
package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
)

// ScanType 区分项目扫描与单包扫描。
type ScanType string

const (
	// ScanTypeProject 表示本地项目目录扫描。
	ScanTypeProject ScanType = "project"
	// ScanTypePackage 表示单包安全评估扫描。
	ScanTypePackage ScanType = "package"
)

// CachedScan 统一记录一次扫描的原始结果与上下文，支持按发现 ID 追溯。
type CachedScan struct {
	ID              string
	Type            ScanType
	Target          string
	ScannedAt       time.Time
	Status          model.ScanStatus
	Summary         model.Summary
	SeverityCounts  map[string]int
	Findings        []model.Finding
	FindingContexts map[string]project.DependencyContext
	Sources         []model.SourceReference
	Limitations     []model.Limitation
	ProjectResult   *project.Result
	PackageReport   *model.ScanReport
}

// ScanCache 维护有界且并发安全的扫描历史。
type ScanCache struct {
	mu      sync.RWMutex
	maxSize int
	order   []string
	scans   map[string]*CachedScan
}

// NewScanCache 创建指定容量上限的扫描缓存。若容量非正，则默认使用 50。
func NewScanCache(maxSize int) *ScanCache {
	if maxSize <= 0 {
		maxSize = 50
	}
	return &ScanCache{
		maxSize: maxSize,
		order:   make([]string, 0, maxSize),
		scans:   make(map[string]*CachedScan, maxSize),
	}
}

// generateScanID 生成带类型前缀的稳定随机标识符。
func generateScanID(prefix string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(bytes[:]))
}

// PutProject 缓存一次项目扫描结果。
func (c *ScanCache) PutProject(result project.Result) *CachedScan {
	c.mu.Lock()
	defer c.mu.Unlock()

	scanID := generateScanID("proj")
	counts := calculateSeverityCounts(result.Findings)

	entry := &CachedScan{
		ID:              scanID,
		Type:            ScanTypeProject,
		Target:          result.Root,
		ScannedAt:       result.ScannedAt,
		Status:          result.Status,
		Summary:         result.Summary,
		SeverityCounts:  counts,
		Findings:        result.Findings,
		FindingContexts: result.FindingContexts,
		Sources:         result.Sources,
		Limitations:     result.Limitations,
		ProjectResult:   &result,
	}

	c.addEntryLocked(entry)
	return entry
}

// PutPackage 缓存一次单包扫描结果。
func (c *ScanCache) PutPackage(report model.ScanReport) *CachedScan {
	c.mu.Lock()
	defer c.mu.Unlock()

	scanID := generateScanID("pkg")
	counts := calculateSeverityCounts(report.Findings)

	targetStr := report.Target.Package.String()
	if report.Target.ResolvedVersion != nil && *report.Target.ResolvedVersion != "" {
		targetStr += "@" + *report.Target.ResolvedVersion
	}

	entry := &CachedScan{
		ID:              scanID,
		Type:            ScanTypePackage,
		Target:          targetStr,
		ScannedAt:       report.ScannedAt,
		Status:          report.Status,
		Summary:         report.Summary,
		SeverityCounts:  counts,
		Findings:        report.Findings,
		FindingContexts: make(map[string]project.DependencyContext),
		Sources:         report.Sources,
		Limitations:     report.Limitations,
		PackageReport:   &report,
	}

	c.addEntryLocked(entry)
	return entry
}

func (c *ScanCache) addEntryLocked(entry *CachedScan) {
	if len(c.order) >= c.maxSize {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.scans, oldest)
	}
	c.order = append(c.order, entry.ID)
	c.scans[entry.ID] = entry
}

// Get 根据扫描 ID 获取缓存条目。
func (c *ScanCache) Get(scanID string) (*CachedScan, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.scans[scanID]
	return entry, ok
}

// Latest 获取最近一次扫描条目。
func (c *ScanCache) Latest() (*CachedScan, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.order) == 0 {
		return nil, false
	}
	latestID := c.order[len(c.order)-1]
	return c.scans[latestID], true
}

// FindFindingAcrossScans 在缓存的所有扫描中查找指定 ID 的发现（优先按指定 scanID，否则从最新到最旧）。
func (c *ScanCache) FindFindingAcrossScans(scanID, findingID string) (*CachedScan, *model.Finding, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if scanID != "" {
		scan, ok := c.scans[scanID]
		if !ok {
			return nil, nil, false
		}
		for i := range scan.Findings {
			if scan.Findings[i].ID == findingID {
				return scan, &scan.Findings[i], true
			}
		}
		return nil, nil, false
	}

	// 从最新向最旧查找
	for i := len(c.order) - 1; i >= 0; i-- {
		scan := c.scans[c.order[i]]
		for j := range scan.Findings {
			if scan.Findings[j].ID == findingID {
				return scan, &scan.Findings[j], true
			}
		}
	}

	return nil, nil, false
}

// calculateSeverityCounts 计算各严重程度等级的发现数量。
func calculateSeverityCounts(findings []model.Finding) map[string]int {
	counts := map[string]int{
		string(model.SeverityCritical): 0,
		string(model.SeverityHigh):     0,
		string(model.SeverityMedium):   0,
		string(model.SeverityLow):      0,
		string(model.SeverityUnknown):  0,
	}
	for _, f := range findings {
		counts[string(f.Severity)]++
	}
	return counts
}
