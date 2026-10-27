// Package mcp 实现向 MCP 客户端暴露的四大核心安全扫描与详情检索工具。
package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolHandlerRegistry 聚合服务依赖并提供工具处理函数。
type toolHandlerRegistry struct {
	service   app.Service
	validator *PathValidator
	cache     *ScanCache
}

// newToolHandlerRegistry 创建工具处理器注册表。
func newToolHandlerRegistry(service app.Service, validator *PathValidator, cache *ScanCache) *toolHandlerRegistry {
	return &toolHandlerRegistry{
		service:   service,
		validator: validator,
		cache:     cache,
	}
}

// scanProject 处理 scan_project 工具调用。
func (r *toolHandlerRegistry) scanProject(ctx context.Context, _ *mcp.CallToolRequest, input ScanProjectInput) (*mcp.CallToolResult, ScanProjectOutput, error) {
	if strings.TrimSpace(input.Path) == "" {
		return nil, ScanProjectOutput{}, fmt.Errorf("path parameter must not be empty")
	}

	validPath, err := r.validator.ValidatePath(input.Path)
	if err != nil {
		return nil, ScanProjectOutput{}, fmt.Errorf("validate project path: %w", err)
	}

	result, err := r.service.ScanProject(ctx, app.ProjectRequest{Path: validPath}, nil)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ScanProjectOutput{}, ctx.Err()
		}
		return nil, ScanProjectOutput{}, fmt.Errorf("scan project: %w", err)
	}

	cached := r.cache.PutProject(result)
	output := buildScanProjectOutput(cached, result)
	return nil, output, nil
}

// scanPackage 处理 scan_package 工具调用。
func (r *toolHandlerRegistry) scanPackage(ctx context.Context, _ *mcp.CallToolRequest, input ScanPackageInput) (*mcp.CallToolResult, ScanPackageOutput, error) {
	pkgStr := strings.TrimSpace(input.Package)
	if pkgStr == "" {
		return nil, ScanPackageOutput{}, fmt.Errorf("package parameter must not be empty")
	}

	defaultEcosystem := model.EcosystemNPM
	if input.Ecosystem != "" {
		eco := model.Ecosystem(strings.ToLower(strings.TrimSpace(input.Ecosystem)))
		if !eco.IsValid() {
			return nil, ScanPackageOutput{}, fmt.Errorf("unsupported ecosystem %q; must be 'npm' or 'pypi'", input.Ecosystem)
		}
		defaultEcosystem = eco
	}

	req, err := app.ParsePackageSpec(pkgStr, defaultEcosystem)
	if err != nil {
		return nil, ScanPackageOutput{}, fmt.Errorf("parse package specifier: %w", err)
	}

	if input.Version != "" {
		version := strings.TrimSpace(input.Version)
		parsedReq, err := app.NewPackageRequest(req.Ecosystem, req.Name, version)
		if err != nil {
			return nil, ScanPackageOutput{}, fmt.Errorf("invalid package version: %w", err)
		}
		req = parsedReq
	}

	report, err := r.service.ScanPackage(ctx, req, nil)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ScanPackageOutput{}, ctx.Err()
		}
		return nil, ScanPackageOutput{}, fmt.Errorf("scan package: %w", err)
	}

	cached := r.cache.PutPackage(report)
	output := buildScanPackageOutput(cached, report)
	return nil, output, nil
}

// getFindingDetails 处理 get_finding_details 工具调用。
func (r *toolHandlerRegistry) getFindingDetails(_ context.Context, _ *mcp.CallToolRequest, input GetFindingDetailsInput) (*mcp.CallToolResult, GetFindingDetailsOutput, error) {
	findingID := strings.TrimSpace(input.FindingID)
	if findingID == "" {
		return nil, GetFindingDetailsOutput{}, fmt.Errorf("finding_id parameter must not be empty")
	}

	scanID := strings.TrimSpace(input.ScanID)
	scan, finding, found := r.cache.FindFindingAcrossScans(scanID, findingID)
	if !found {
		if scanID != "" {
			return nil, GetFindingDetailsOutput{}, fmt.Errorf("finding %q was not found in scan %q", findingID, scanID)
		}
		return nil, GetFindingDetailsOutput{}, fmt.Errorf("finding %q was not found in recent scans", findingID)
	}

	output := buildFindingDetailsOutput(scan, *finding)
	return nil, output, nil
}

// getScanSummary 处理 get_scan_summary 工具调用。
func (r *toolHandlerRegistry) getScanSummary(ctx context.Context, _ *mcp.CallToolRequest, input GetScanSummaryInput) (*mcp.CallToolResult, GetScanSummaryOutput, error) {
	scanID := strings.TrimSpace(input.ScanID)
	pathStr := strings.TrimSpace(input.Path)
	pkgStr := strings.TrimSpace(input.Package)

	var scan *CachedScan
	if scanID != "" {
		cached, ok := r.cache.Get(scanID)
		if !ok {
			return nil, GetScanSummaryOutput{}, fmt.Errorf("scan %q was not found in cache", scanID)
		}
		scan = cached
	} else if pathStr != "" {
		validPath, err := r.validator.ValidatePath(pathStr)
		if err != nil {
			return nil, GetScanSummaryOutput{}, fmt.Errorf("validate project path: %w", err)
		}
		result, err := r.service.ScanProject(ctx, app.ProjectRequest{Path: validPath}, nil)
		if err != nil {
			if ctx.Err() != nil {
				return nil, GetScanSummaryOutput{}, ctx.Err()
			}
			return nil, GetScanSummaryOutput{}, fmt.Errorf("scan project: %w", err)
		}
		scan = r.cache.PutProject(result)
	} else if pkgStr != "" {
		defaultEcosystem := model.EcosystemNPM
		if input.Ecosystem != "" {
			eco := model.Ecosystem(strings.ToLower(strings.TrimSpace(input.Ecosystem)))
			if !eco.IsValid() {
				return nil, GetScanSummaryOutput{}, fmt.Errorf("unsupported ecosystem %q", input.Ecosystem)
			}
			defaultEcosystem = eco
		}
		req, err := app.ParsePackageSpec(pkgStr, defaultEcosystem)
		if err != nil {
			return nil, GetScanSummaryOutput{}, fmt.Errorf("parse package: %w", err)
		}
		if input.Version != "" {
			version := strings.TrimSpace(input.Version)
			parsedReq, err := app.NewPackageRequest(req.Ecosystem, req.Name, version)
			if err != nil {
				return nil, GetScanSummaryOutput{}, fmt.Errorf("invalid package version: %w", err)
			}
			req = parsedReq
		}
		report, err := r.service.ScanPackage(ctx, req, nil)
		if err != nil {
			if ctx.Err() != nil {
				return nil, GetScanSummaryOutput{}, ctx.Err()
			}
			return nil, GetScanSummaryOutput{}, fmt.Errorf("scan package: %w", err)
		}
		scan = r.cache.PutPackage(report)
	} else {
		latest, ok := r.cache.Latest()
		if !ok {
			return nil, GetScanSummaryOutput{}, fmt.Errorf("no recent scan available; please provide scan_id, path, or package")
		}
		scan = latest
	}

	output := buildScanSummaryOutput(scan)
	return nil, output, nil
}

// formatScanStatusMessage 生成关于扫描结果安全含义的清晰提示信息。
// 绝不将无发现等同于绝对安全保证。
func formatScanStatusMessage(status model.ScanStatus, findingCount int) string {
	switch status {
	case model.StatusClean:
		return "No known security advisories or risk warnings were identified for the resolved packages by consulted sources. Note that a clean scan does not guarantee the package or dependencies are entirely free of security defects."
	case model.StatusFindings:
		return fmt.Sprintf("Security findings or warnings were identified in the scanned dependencies (total findings: %d).", findingCount)
	case model.StatusIncomplete:
		return "Scan completed with incomplete coverage. Some dependencies could not be resolved or supported project manifests were missing."
	case model.StatusFailed:
		return "Scan failed due to an error encountered during manifest parsing or security engine queries."
	case model.StatusCancelled:
		return "Scan was cancelled."
	default:
		return fmt.Sprintf("Scan completed with status %s.", status)
	}
}

func buildScanProjectOutput(cached *CachedScan, result project.Result) ScanProjectOutput {
	formats := make([]FormatInfoOutput, 0, len(result.Formats))
	for _, f := range result.Formats {
		formats = append(formats, FormatInfoOutput{
			Ecosystem:          string(f.Ecosystem),
			Format:             string(f.Format),
			ManifestPath:       f.ManifestPath,
			LockfilePath:       f.LockfilePath,
			ResolvedPackages:   f.ResolvedPackages,
			UnresolvedPackages: f.UnresolvedPackages,
			Resolution:         string(f.Resolution),
		})
	}

	findings := make([]FindingBriefOutput, 0, len(result.Findings))
	for _, f := range result.Findings {
		brief := FindingBriefOutput{
			ID:       f.ID,
			Kind:     string(f.Kind),
			Title:    f.Title,
			Severity: string(f.Severity),
			Package: PackageIdentityOutput{
				Ecosystem: string(f.Package.Ecosystem),
				Name:      f.Package.Name,
			},
			Version: f.Version,
		}
		if ctx, ok := result.FindingContexts[f.ID]; ok {
			brief.Relationship = string(ctx.Relationship)
			scopes := make([]string, 0, len(ctx.Scopes))
			for _, s := range ctx.Scopes {
				scopes = append(scopes, string(s))
			}
			brief.Scopes = scopes
			brief.Resolution = string(ctx.Resolution)
		}
		if f.Risk != nil {
			score := f.Risk.Score
			brief.RiskScore = &score
			brief.RiskRationale = f.Risk.Rationale
		}
		findings = append(findings, brief)
	}

	checks := make([]CheckResultOutput, 0, len(result.Checks))
	for _, c := range result.Checks {
		checks = append(checks, CheckResultOutput{
			ID:        c.ID,
			Name:      c.Name,
			Status:    string(c.Status),
			Message:   c.Message,
			SourceIDs: c.SourceIDs,
		})
	}

	limitations := make([]LimitationOutput, 0, len(result.Limitations))
	for _, l := range result.Limitations {
		limitations = append(limitations, LimitationOutput{
			Code:    l.Code,
			Message: l.Message,
		})
	}

	sources := make([]SourceOutput, 0, len(result.Sources))
	for _, s := range result.Sources {
		sources = append(sources, SourceOutput{
			ID:          s.ID,
			Provider:    string(s.Provider),
			Identifier:  s.Identifier,
			URL:         s.URL,
			RetrievedAt: s.RetrievedAt.Format(time.RFC3339),
		})
	}

	var highestSeverity *string
	if result.Summary.HighestSeverity != nil {
		sev := string(*result.Summary.HighestSeverity)
		highestSeverity = &sev
	}

	return ScanProjectOutput{
		ScanID:    cached.ID,
		Root:      result.Root,
		ScannedAt: result.ScannedAt.Format(time.RFC3339),
		Status:    string(result.Status),
		Message:   formatScanStatusMessage(result.Status, len(result.Findings)),
		Summary: ScanSummaryOutput{
			ConfirmedFindings: result.Summary.ConfirmedFindings,
			HeuristicWarnings: result.Summary.HeuristicWarnings,
			HighestSeverity:   highestSeverity,
			SeverityCounts:    cached.SeverityCounts,
		},
		Formats:     formats,
		Findings:    findings,
		Checks:      checks,
		Limitations: limitations,
		Sources:     sources,
	}
}

func buildScanPackageOutput(cached *CachedScan, report model.ScanReport) ScanPackageOutput {
	findings := make([]FindingBriefOutput, 0, len(report.Findings))
	for _, f := range report.Findings {
		brief := FindingBriefOutput{
			ID:       f.ID,
			Kind:     string(f.Kind),
			Title:    f.Title,
			Severity: string(f.Severity),
			Package: PackageIdentityOutput{
				Ecosystem: string(f.Package.Ecosystem),
				Name:      f.Package.Name,
			},
			Version:      f.Version,
			Relationship: string(model.DependencyDirect),
		}
		if f.Risk != nil {
			score := f.Risk.Score
			brief.RiskScore = &score
			brief.RiskRationale = f.Risk.Rationale
		}
		findings = append(findings, brief)
	}

	checks := make([]CheckResultOutput, 0, len(report.Checks))
	for _, c := range report.Checks {
		checks = append(checks, CheckResultOutput{
			ID:        c.ID,
			Name:      c.Name,
			Status:    string(c.Status),
			Message:   c.Message,
			SourceIDs: c.SourceIDs,
		})
	}

	limitations := make([]LimitationOutput, 0, len(report.Limitations))
	for _, l := range report.Limitations {
		limitations = append(limitations, LimitationOutput{
			Code:    l.Code,
			Message: l.Message,
		})
	}

	sources := make([]SourceOutput, 0, len(report.Sources))
	for _, s := range report.Sources {
		sources = append(sources, SourceOutput{
			ID:          s.ID,
			Provider:    string(s.Provider),
			Identifier:  s.Identifier,
			URL:         s.URL,
			RetrievedAt: s.RetrievedAt.Format(time.RFC3339),
		})
	}

	var highestSeverity *string
	if report.Summary.HighestSeverity != nil {
		sev := string(*report.Summary.HighestSeverity)
		highestSeverity = &sev
	}

	return ScanPackageOutput{
		ScanID: cached.ID,
		Target: PackageTargetOutput{
			Package: PackageIdentityOutput{
				Ecosystem: string(report.Target.Package.Ecosystem),
				Name:      report.Target.Package.Name,
			},
			ResolvedVersion: report.Target.ResolvedVersion,
		},
		ScannedAt: report.ScannedAt.Format(time.RFC3339),
		Status:    string(report.Status),
		DataMode:  string(report.DataMode),
		Message:   formatScanStatusMessage(report.Status, len(report.Findings)),
		Summary: ScanSummaryOutput{
			ConfirmedFindings: report.Summary.ConfirmedFindings,
			HeuristicWarnings: report.Summary.HeuristicWarnings,
			HighestSeverity:   highestSeverity,
			SeverityCounts:    cached.SeverityCounts,
		},
		Findings:    findings,
		Checks:      checks,
		Limitations: limitations,
		Sources:     sources,
	}
}

func buildFindingDetailsOutput(scan *CachedScan, finding model.Finding) GetFindingDetailsOutput {
	evidenceList := make([]FindingEvidenceOutput, 0, len(finding.Evidence))
	sourceIDsMap := make(map[string]struct{})
	representativePaths := make([][]string, 0)

	for _, ev := range finding.Evidence {
		evidenceList = append(evidenceList, FindingEvidenceOutput{
			Kind:      string(ev.Kind),
			Summary:   ev.Summary,
			SourceID:  ev.SourceID,
			Reference: ev.Reference,
		})
		if ev.SourceID != "" {
			sourceIDsMap[ev.SourceID] = struct{}{}
		}
		if ev.Kind == model.EvidenceDependencyPath && ev.Summary != "" && !strings.HasPrefix(ev.Summary, "Additional dependency paths") {
			parts := strings.Split(ev.Summary, " -> ")
			if len(parts) > 0 {
				representativePaths = append(representativePaths, parts)
			}
		}
	}

	matchingSources := make([]SourceOutput, 0)
	for _, s := range scan.Sources {
		if _, matches := sourceIDsMap[s.ID]; matches {
			matchingSources = append(matchingSources, SourceOutput{
				ID:          s.ID,
				Provider:    string(s.Provider),
				Identifier:  s.Identifier,
				URL:         s.URL,
				RetrievedAt: s.RetrievedAt.Format(time.RFC3339),
			})
		}
	}

	var depContext *DependencyContextOutput
	if scan.Type == ScanTypeProject {
		if ctx, ok := scan.FindingContexts[finding.ID]; ok {
			scopes := make([]string, 0, len(ctx.Scopes))
			for _, sc := range ctx.Scopes {
				scopes = append(scopes, string(sc))
			}
			evidenceItems := make([]DependencyEvidenceItemOutput, 0, len(ctx.Evidence))
			for _, item := range ctx.Evidence {
				evidenceItems = append(evidenceItems, DependencyEvidenceItemOutput{
					Path:  item.Path,
					Field: item.Field,
				})
			}
			depContext = &DependencyContextOutput{
				Relationship:        string(ctx.Relationship),
				Scopes:              scopes,
				Resolution:          string(ctx.Resolution),
				Evidence:            evidenceItems,
				RepresentativePaths: representativePaths,
			}
		}
	} else if scan.Type == ScanTypePackage {
		depContext = &DependencyContextOutput{
			Relationship: string(model.DependencyDirect),
			Scopes:       []string{string(project.DependencyScopeUnknown)},
			Resolution:   string(project.DependencyResolutionComplete),
			Evidence:     []DependencyEvidenceItemOutput{},
			RepresentativePaths: [][]string{
				{scan.Target},
			},
		}
	}

	var riskOutput *RiskAssessmentOutput
	if finding.Risk != nil {
		riskOutput = &RiskAssessmentOutput{
			Score:     finding.Risk.Score,
			Rationale: finding.Risk.Rationale,
		}
	}

	message := "Finding details, evidence records, and dependency context preserved from underlying scans."
	if depContext != nil && depContext.Resolution == string(project.DependencyResolutionPartial) {
		message += " Note: Dependency resolution for this format was partial; evidence reflects available lockfile/manifest declarations."
	}

	return GetFindingDetailsOutput{
		FindingID:   finding.ID,
		ScanID:      scan.ID,
		Kind:        string(finding.Kind),
		Title:       finding.Title,
		Description: finding.Description,
		Severity:    string(finding.Severity),
		Package: PackageIdentityOutput{
			Ecosystem: string(finding.Package.Ecosystem),
			Name:      finding.Package.Name,
		},
		Version:           finding.Version,
		Risk:              riskOutput,
		Remediation:       finding.Remediation,
		Evidence:          evidenceList,
		Sources:           matchingSources,
		DependencyContext: depContext,
		Message:           message,
	}
}

func buildScanSummaryOutput(scan *CachedScan) GetScanSummaryOutput {
	totalFindings := len(scan.Findings)
	var highestSev *string
	if scan.Summary.HighestSeverity != nil {
		sev := string(*scan.Summary.HighestSeverity)
		highestSev = &sev
	}

	dependenciesCount := 0
	resolvedPackagesCount := 0
	unresolvedPackagesCount := 0
	resolution := string(project.DependencyResolutionUnknown)

	if scan.Type == ScanTypeProject && scan.ProjectResult != nil {
		dependenciesCount = len(scan.ProjectResult.Dependencies.Nodes)
		hasPartial := false
		hasComplete := false
		for _, f := range scan.ProjectResult.Formats {
			resolvedPackagesCount += f.ResolvedPackages
			unresolvedPackagesCount += f.UnresolvedPackages
			if f.Resolution == project.DependencyResolutionPartial {
				hasPartial = true
			} else if f.Resolution == project.DependencyResolutionComplete {
				hasComplete = true
			}
		}
		if hasPartial {
			resolution = string(project.DependencyResolutionPartial)
		} else if hasComplete {
			resolution = string(project.DependencyResolutionComplete)
		} else if len(scan.ProjectResult.Formats) == 0 {
			resolution = string(project.DependencyResolutionUnknown)
		}
	} else if scan.Type == ScanTypePackage && scan.PackageReport != nil {
		dependenciesCount = len(scan.PackageReport.Dependencies.Nodes)
		if scan.PackageReport.Target.ResolvedVersion != nil {
			resolvedPackagesCount = 1
			resolution = string(project.DependencyResolutionComplete)
		} else {
			unresolvedPackagesCount = 1
			resolution = string(project.DependencyResolutionPartial)
		}
	}

	hasUnresolved := unresolvedPackagesCount > 0 || resolution == string(project.DependencyResolutionPartial)

	msg := formatScanStatusMessage(scan.Status, totalFindings)

	return GetScanSummaryOutput{
		ScanID:     scan.ID,
		TargetType: string(scan.Type),
		TargetName: scan.Target,
		Status:     string(scan.Status),
		ScannedAt:  scan.ScannedAt.Format(time.RFC3339),
		Totals: SummaryTotals{
			TotalFindings:           totalFindings,
			ConfirmedFindings:       scan.Summary.ConfirmedFindings,
			HeuristicWarnings:       scan.Summary.HeuristicWarnings,
			HighestSeverity:         highestSev,
			SeverityCounts:          scan.SeverityCounts,
			DependenciesCount:       dependenciesCount,
			ResolvedPackagesCount:   resolvedPackagesCount,
			UnresolvedPackagesCount: unresolvedPackagesCount,
		},
		Resolution:                resolution,
		LimitationsCount:          len(scan.Limitations),
		HasUnresolvedDependencies: hasUnresolved,
		Message:                   msg,
	}
}
