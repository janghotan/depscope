// Package osv 实现 OSV.dev 漏洞查询提供者。
package osv

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"depscope/internal/engine"
	"depscope/internal/fetch"
	"depscope/internal/model"
)

const (
	defaultBaseURL = "https://api.osv.dev"
	sourceID       = "osv"
)

// Options 配置 OSV 提供者。
type Options struct {
	BaseURL string
	Client  *fetch.Client
	Now     func() time.Time
}

// Provider 通过 OSV.dev 查询已解析版本的漏洞公告。
type Provider struct {
	baseURL string
	client  *fetch.Client
	now     func() time.Time
}

// NewProvider 创建 OSV 提供者。
func NewProvider(opts Options) *Provider {
	if opts.BaseURL == "" {
		opts.BaseURL = defaultBaseURL
	}
	if opts.Client == nil {
		opts.Client = fetch.New(fetch.Config{})
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Provider{
		baseURL: strings.TrimRight(opts.BaseURL, "/"),
		client:  opts.Client,
		now:     opts.Now,
	}
}

// Query 查询指定包版本适用的漏洞公告。
func (p *Provider) Query(ctx context.Context, pkg model.PackageIdentity, version string) (engine.VulnerabilityResult, error) {
	if err := pkg.Validate(); err != nil {
		return engine.VulnerabilityResult{}, err
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return engine.VulnerabilityResult{}, fmt.Errorf("version must not be empty")
	}

	endpoint := p.baseURL + "/v1/query"
	request := queryRequest{
		Version: version,
		Package: packageReference{
			Ecosystem: osvEcosystem(pkg.Ecosystem),
			Name:      pkg.Name,
		},
	}
	var document queryDocument
	if err := p.client.PostJSON(ctx, "osv", endpoint, request, &document); err != nil {
		return engine.VulnerabilityResult{}, err
	}

	source := model.SourceReference{
		ID:          sourceID,
		Provider:    model.SourceProviderOSV,
		Identifier:  pkg.String() + "@" + version,
		URL:         &endpoint,
		RetrievedAt: p.now().UTC(),
	}

	advisories := make([]engine.Advisory, 0, len(document.Vulns))
	limitations := make([]model.Limitation, 0)
	for _, vulnerability := range document.Vulns {
		advisory, matched, advisoryLimitations := normalize(vulnerability, pkg)
		limitations = append(limitations, advisoryLimitations...)
		if !matched {
			continue
		}
		advisories = append(advisories, advisory)
	}
	sort.Slice(advisories, func(i, j int) bool { return advisories[i].ID < advisories[j].ID })

	return engine.VulnerabilityResult{
		Advisories:  advisories,
		Source:      source,
		Limitations: limitations,
	}, nil
}

type queryRequest struct {
	Version string           `json:"version"`
	Package packageReference `json:"package"`
}

type packageReference struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
}

type queryDocument struct {
	Vulns []vulnerability `json:"vulns"`
}

type vulnerability struct {
	ID               string           `json:"id"`
	Summary          string           `json:"summary"`
	Details          string           `json:"details"`
	Aliases          []string         `json:"aliases"`
	References       []reference      `json:"references"`
	Affected         []affected       `json:"affected"`
	Severity         []severity       `json:"severity"`
	DatabaseSpecific databaseSpecific `json:"database_specific"`
}

type reference struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type affected struct {
	Package           packageReference `json:"package"`
	Ranges            []versionRange   `json:"ranges"`
	Versions          []string         `json:"versions"`
	DatabaseSpecific  databaseSpecific `json:"database_specific"`
	EcosystemSpecific databaseSpecific `json:"ecosystem_specific"`
}

type versionRange struct {
	Type   string              `json:"type"`
	Events []map[string]string `json:"events"`
}

type severity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

type databaseSpecific struct {
	Severity string `json:"severity"`
}

func normalize(vulnerability vulnerability, pkg model.PackageIdentity) (engine.Advisory, bool, []model.Limitation) {
	limitations := make([]model.Limitation, 0)
	matching := make([]affected, 0, len(vulnerability.Affected))
	for _, entry := range vulnerability.Affected {
		if matchesPackage(pkg, entry.Package) {
			matching = append(matching, entry)
		}
	}
	if len(vulnerability.Affected) > 0 && len(matching) == 0 {
		limitations = append(limitations, model.Limitation{
			Code:    "OSV_AFFECTED_PACKAGE_MISMATCH",
			Message: fmt.Sprintf("Ignored OSV record %s because its affected package entries did not match the requested package.", vulnerability.ID),
		})
		return engine.Advisory{}, false, limitations
	}

	references := make([]string, 0, len(vulnerability.References))
	seenReferences := make(map[string]struct{}, len(vulnerability.References))
	for _, item := range vulnerability.References {
		value := strings.TrimSpace(item.URL)
		if value == "" {
			continue
		}
		if _, exists := seenReferences[value]; exists {
			continue
		}
		seenReferences[value] = struct{}{}
		references = append(references, value)
	}

	severityValue := severityOf(vulnerability, matching)
	ranges := make([]string, 0)
	fixedVersions := make([]string, 0)
	seenFixes := make(map[string]struct{})
	for _, entry := range matching {
		for _, versionRange := range entry.Ranges {
			ranges = append(ranges, formatRange(versionRange))
			for _, event := range versionRange.Events {
				if fixed := strings.TrimSpace(event["fixed"]); fixed != "" {
					if _, exists := seenFixes[fixed]; exists {
						continue
					}
					seenFixes[fixed] = struct{}{}
					fixedVersions = append(fixedVersions, fixed)
				}
			}
		}
		if len(entry.Ranges) == 0 && len(entry.Versions) > 0 {
			ranges = append(ranges, formatVersions(entry.Versions))
		}
	}
	sort.Strings(fixedVersions)

	summary := strings.TrimSpace(vulnerability.Summary)
	if summary == "" {
		summary = "OSV advisory " + vulnerability.ID
	}
	details := buildDetails(vulnerability, severityValue, ranges, references)

	return engine.Advisory{
		ID:             vulnerability.ID,
		Aliases:        append([]string(nil), vulnerability.Aliases...),
		Summary:        summary,
		Details:        details,
		Severity:       severityValue,
		References:     references,
		FixedVersions:  fixedVersions,
		AffectedRanges: ranges,
	}, true, limitations
}

func matchesPackage(requested model.PackageIdentity, candidate packageReference) bool {
	if !strings.EqualFold(candidate.Ecosystem, osvEcosystem(requested.Ecosystem)) {
		return false
	}
	canonical, err := model.NewPackageIdentity(requested.Ecosystem, candidate.Name)
	if err != nil {
		return false
	}
	return canonical == requested
}

func severityOf(vulnerability vulnerability, matching []affected) model.Severity {
	if severity := normalizeSeverityLabel(vulnerability.DatabaseSpecific.Severity); severity != model.SeverityUnknown {
		return severity
	}
	for _, entry := range matching {
		if severity := normalizeSeverityLabel(entry.DatabaseSpecific.Severity); severity != model.SeverityUnknown {
			return severity
		}
		if severity := normalizeSeverityLabel(entry.EcosystemSpecific.Severity); severity != model.SeverityUnknown {
			return severity
		}
	}
	best := model.SeverityUnknown
	for _, item := range vulnerability.Severity {
		score, err := strconv.ParseFloat(strings.TrimSpace(item.Score), 64)
		if err != nil {
			continue
		}
		candidate := severityFromScore(score)
		if candidate.Rank() > best.Rank() {
			best = candidate
		}
	}
	return best
}

func normalizeSeverityLabel(value string) model.Severity {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical":
		return model.SeverityCritical
	case "high", "important":
		return model.SeverityHigh
	case "moderate", "medium":
		return model.SeverityMedium
	case "low":
		return model.SeverityLow
	default:
		return model.SeverityUnknown
	}
}

func severityFromScore(score float64) model.Severity {
	switch {
	case score >= 9:
		return model.SeverityCritical
	case score >= 7:
		return model.SeverityHigh
	case score >= 4:
		return model.SeverityMedium
	case score > 0:
		return model.SeverityLow
	default:
		return model.SeverityUnknown
	}
}

func formatRange(versionRange versionRange) string {
	parts := make([]string, 0, len(versionRange.Events)+1)
	if versionRange.Type != "" {
		parts = append(parts, versionRange.Type)
	}
	for _, event := range versionRange.Events {
		for _, key := range []string{"introduced", "fixed", "last_affected", "limit"} {
			if value, exists := event[key]; exists && strings.TrimSpace(value) != "" {
				parts = append(parts, key+" "+value)
			}
		}
	}
	if len(parts) == 0 {
		return "affected range unavailable"
	}
	return strings.Join(parts, ", ")
}

func formatVersions(versions []string) string {
	const limit = 20
	values := versions
	truncated := false
	if len(values) > limit {
		values = values[:limit]
		truncated = true
	}
	result := "explicit versions: " + strings.Join(values, ", ")
	if truncated {
		result += " (truncated)"
	}
	return result
}

func buildDetails(vulnerability vulnerability, severity model.Severity, ranges, references []string) string {
	parts := make([]string, 0, 6)
	if details := strings.TrimSpace(vulnerability.Details); details != "" {
		parts = append(parts, details)
	} else if summary := strings.TrimSpace(vulnerability.Summary); summary != "" {
		parts = append(parts, summary)
	}
	if len(vulnerability.Aliases) > 0 {
		parts = append(parts, "Aliases: "+strings.Join(vulnerability.Aliases, ", "))
	}
	if len(ranges) > 0 {
		parts = append(parts, "Affected ranges: "+strings.Join(ranges, "; "))
	}
	parts = append(parts, "OSV severity: "+string(severity))
	if len(references) > 0 {
		items := references
		if len(items) > 10 {
			items = items[:10]
		}
		parts = append(parts, "References: "+strings.Join(items, " "))
	}
	result := strings.Join(parts, "\n\n")
	if len(result) > 4000 {
		result = result[:4000] + "\n\n[truncated]"
	}
	return result
}

func osvEcosystem(ecosystem model.Ecosystem) string {
	switch ecosystem {
	case model.EcosystemNPM:
		return "npm"
	case model.EcosystemPyPI:
		return "PyPI"
	default:
		return string(ecosystem)
	}
}
