package engine

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"depscope/internal/model"
)

// ResolvedPackage 表示一个已知确切的包版本。
type ResolvedPackage struct {
	Package model.PackageIdentity
	Version string
}

// PackageScanResult 表示批量扫描中单个已解析包的漏洞与风险评估结果。
// Err 只记录该包的提供者失败，不影响其他包的结果。
type PackageScanResult struct {
	Package     model.PackageIdentity
	Version     string
	Findings    []model.Finding
	Checks      []model.CheckResult
	Sources     []model.SourceReference
	Limitations []model.Limitation
	Err         error
}

// ScanResolvedBatch 对一组已解析版本执行漏洞查询与风险评估。
// 结果按生态系统、包名和版本确定性排序并去重；并发受配置上限约束。
func (s *Scanner) ScanResolvedBatch(ctx context.Context, packages []ResolvedPackage) ([]PackageScanResult, error) {
	items, err := normalizeResolvedPackages(packages)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []PackageScanResult{}, nil
	}

	scanCtx, cancel := s.config.ScanContext(ctx)
	defer cancel()
	results := make([]PackageScanResult, len(items))
	workers := s.config.MaxConcurrency
	if workers > len(items) {
		workers = len(items)
	}

	jobs := make(chan int)
	var wait sync.WaitGroup
	wait.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer wait.Done()
			for index := range jobs {
				results[index] = s.scanResolved(scanCtx, items[index])
			}
		}()
	}

dispatch:
	for index := range items {
		select {
		case jobs <- index:
		case <-scanCtx.Done():
			break dispatch
		}
	}
	close(jobs)
	wait.Wait()

	if err := scanCtx.Err(); err != nil {
		return results, err
	}
	return results, nil
}

func normalizeResolvedPackages(packages []ResolvedPackage) ([]ResolvedPackage, error) {
	seen := make(map[string]struct{}, len(packages))
	result := make([]ResolvedPackage, 0, len(packages))
	for _, item := range packages {
		if err := item.Package.Validate(); err != nil {
			return nil, fmt.Errorf("invalid resolved package: %w", err)
		}
		version := strings.TrimSpace(item.Version)
		if version == "" {
			return nil, fmt.Errorf("resolved package %s requires a version", item.Package)
		}
		key := item.Package.String() + "@" + version
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, ResolvedPackage{Package: item.Package, Version: version})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Package.String() != result[j].Package.String() {
			return result[i].Package.String() < result[j].Package.String()
		}
		return result[i].Version < result[j].Version
	})
	return result, nil
}

func (s *Scanner) scanResolved(ctx context.Context, item ResolvedPackage) PackageScanResult {
	result := PackageScanResult{
		Package:     item.Package,
		Version:     item.Version,
		Findings:    []model.Finding{},
		Checks:      []model.CheckResult{},
		Sources:     []model.SourceReference{},
		Limitations: []model.Limitation{},
	}

	var vulnerability VulnerabilityResult
	err := s.callProvider(ctx, func(callCtx context.Context) error {
		var queryErr error
		vulnerability, queryErr = s.vulnerabilities.Query(callCtx, item.Package, item.Version)
		return queryErr
	})
	if err != nil {
		result.Err = err
		result.Checks = append(result.Checks, model.CheckResult{
			ID:        checkVulnerabilities,
			Name:      "OSV vulnerability query",
			Status:    model.CheckUnavailable,
			Message:   fmt.Sprintf("OSV vulnerability data for %s@%s could not be read: %v", item.Package, item.Version, err),
			SourceIDs: []string{},
		})
		if ctx.Err() == nil {
			result.Limitations = append(result.Limitations, model.Limitation{
				Code:    "VULNERABILITY_DATA_UNAVAILABLE",
				Message: fmt.Sprintf("OSV vulnerability data for %s@%s could not be read.", item.Package, item.Version),
			})
		}
		return result
	}

	if vulnerability.Source.ID != "" {
		result.Sources = append(result.Sources, vulnerability.Source)
	}
	result.Limitations = append(result.Limitations, vulnerability.Limitations...)

	status := model.CheckSucceeded
	message := fmt.Sprintf("OSV returned no known advisories affecting %s@%s.", item.Package, item.Version)
	if len(vulnerability.Advisories) > 0 {
		message = fmt.Sprintf("OSV returned %d advisory(ies) affecting %s@%s.", len(vulnerability.Advisories), item.Package, item.Version)
	}
	if len(vulnerability.Limitations) > 0 {
		status = model.CheckUnavailable
		message = fmt.Sprintf("OSV returned %d advisory(ies) for %s@%s, but part of the response could not be interpreted; coverage is incomplete.", len(vulnerability.Advisories), item.Package, item.Version)
	}

	seen := make(map[string]struct{}, len(vulnerability.Advisories))
	riskFailed := false
	for _, advisory := range vulnerability.Advisories {
		if _, exists := seen[advisory.ID]; exists {
			continue
		}
		seen[advisory.ID] = struct{}{}
		finding := vulnerabilityFinding(item.Package, item.Version, advisory, vulnerability.Source.ID, s.advisoryFallbackReference(advisory.ID))
		assessment, riskErr := s.risk.Evaluate(ctx, RiskInput{Finding: finding})
		if riskErr != nil {
			if ctx.Err() != nil {
				break
			}
			riskFailed = true
			finding.Risk = nil
		} else if !validRiskAssessment(assessment) {
			riskFailed = true
			finding.Risk = nil
		} else {
			finding.Risk = &assessment
		}
		result.Findings = append(result.Findings, finding)
	}
	if riskFailed {
		message := fmt.Sprintf("Risk prioritization could not be computed for %s@%s. Findings are retained without risk scores.", item.Package, item.Version)
		result.Checks = append(result.Checks, model.CheckResult{
			ID:        checkRiskEvaluation,
			Name:      "Risk evaluation",
			Status:    model.CheckFailed,
			Message:   message,
			SourceIDs: []string{},
		})
		result.Limitations = append(result.Limitations, model.Limitation{
			Code:    "RISK_EVALUATION_UNAVAILABLE",
			Message: message,
		})
	}

	result.Checks = append(result.Checks, model.CheckResult{
		ID:        checkVulnerabilities,
		Name:      "OSV vulnerability query",
		Status:    status,
		Message:   message,
		SourceIDs: batchSourceIDs(vulnerability.Source.ID),
	})
	sortBatchResult(&result)
	return result
}

func validRiskAssessment(assessment model.RiskAssessment) bool {
	if math.IsNaN(assessment.Score) || math.IsInf(assessment.Score, 0) || assessment.Score < 0 || assessment.Score > 100 {
		return false
	}
	return strings.TrimSpace(assessment.Rationale) != ""
}

func batchSourceIDs(id string) []string {
	if strings.TrimSpace(id) == "" {
		return []string{}
	}
	return []string{id}
}

func sortBatchResult(result *PackageScanResult) {
	sort.SliceStable(result.Findings, func(i, j int) bool {
		left, right := result.Findings[i], result.Findings[j]
		if left.Kind != right.Kind {
			return left.Kind == model.FindingConfirmed
		}
		if left.Severity.Rank() != right.Severity.Rank() {
			return left.Severity.Rank() > right.Severity.Rank()
		}
		return left.ID < right.ID
	})
	sort.SliceStable(result.Checks, func(i, j int) bool {
		return result.Checks[i].ID < result.Checks[j].ID
	})
	sort.SliceStable(result.Sources, func(i, j int) bool {
		return result.Sources[i].ID < result.Sources[j].ID
	})
	sort.SliceStable(result.Limitations, func(i, j int) bool {
		if result.Limitations[i].Code != result.Limitations[j].Code {
			return result.Limitations[i].Code < result.Limitations[j].Code
		}
		return result.Limitations[i].Message < result.Limitations[j].Message
	})
}
