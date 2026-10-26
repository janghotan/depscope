// Package pypi 实现 PyPI JSON API 元数据提供者。
package pypi

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"depscope/internal/engine"
	"depscope/internal/fetch"
	"depscope/internal/model"
)

const (
	defaultBaseURL = "https://pypi.org"
	sourceID       = "pypi-json"
)

var repositoryURLKeys = []string{
	"repository", "source", "source code", "code", "homepage",
}

// Options 配置 PyPI 提供者。
type Options struct {
	BaseURL string
	Client  *fetch.Client
	Now     func() time.Time
}

// Provider 通过 PyPI JSON API 读取项目与发行版元数据。
type Provider struct {
	baseURL string
	client  *fetch.Client
}

// NewProvider 创建 PyPI 提供者。
func NewProvider(opts Options) *Provider {
	if opts.BaseURL == "" {
		opts.BaseURL = defaultBaseURL
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Client == nil {
		opts.Client = fetch.New(fetch.Config{Now: opts.Now})
	}
	return &Provider{
		baseURL: strings.TrimRight(opts.BaseURL, "/"),
		client:  opts.Client,
	}
}

// Ecosystem 返回 PyPI。
func (p *Provider) Ecosystem() model.Ecosystem {
	return model.EcosystemPyPI
}

// PackageMetadata 读取项目级元数据与全部发行版。
func (p *Provider) PackageMetadata(ctx context.Context, pkg model.PackageIdentity) (engine.RegistryMetadata, error) {
	document, source, err := p.fetchProject(ctx, pkg)
	if err != nil {
		return engine.RegistryMetadata{}, err
	}
	if document.Info.Name != "" {
		canonical, canonicalErr := model.NewPackageIdentity(model.EcosystemPyPI, document.Info.Name)
		if canonicalErr != nil || canonical != pkg {
			return engine.RegistryMetadata{}, &engine.ProviderError{
				Provider: "pypi",
				Kind:     engine.ProviderErrorInvalidResponse,
				Err:      fmt.Errorf("project API returned %q for requested package %q", document.Info.Name, pkg.Name),
			}
		}
	}

	versions := make([]string, 0, len(document.Releases))
	for version := range document.Releases {
		versions = append(versions, version)
	}
	sort.Strings(versions)

	limitations := make([]model.Limitation, 0)
	if len(versions) == 0 {
		limitations = append(limitations, model.Limitation{
			Code:    "EMPTY_VERSION_LIST",
			Message: "The PyPI API returned no releases for this project.",
		})
	}

	var latest *string
	if value := strings.TrimSpace(document.Info.Version); value != "" {
		latest = &value
	}

	var latestPublishedAt *time.Time
	if latest != nil {
		files := document.URLs
		if len(files) == 0 {
			files = document.Releases[*latest]
		}
		published, invalid := earliestUpload(files)
		latestPublishedAt = published
		if invalid {
			limitations = append(limitations, model.Limitation{
				Code:    "INVALID_PUBLISH_TIME",
				Message: "The PyPI API returned an unparseable upload time for the latest release.",
			})
		}
		if document.Info.Yanked {
			limitations = append(limitations, model.Limitation{
				Code:    "YANKED_RELEASE",
				Message: "The latest PyPI release is marked as yanked by its publisher.",
			})
		}
	}

	return engine.RegistryMetadata{
		Package:           pkg,
		Versions:          versions,
		LatestVersion:     latest,
		LatestPublishedAt: latestPublishedAt,
		RepositoryURL:     repositoryURL(document.Info),
		Source:            source,
		Limitations:       limitations,
	}, nil
}

// Dependencies 返回指定发行版声明的直接依赖需求。
func (p *Provider) Dependencies(ctx context.Context, pkg model.PackageIdentity, version string) ([]engine.DependencyRequirement, error) {
	metadata, err := p.VersionMetadata(ctx, pkg, version)
	if err != nil {
		return nil, err
	}
	return metadata.Dependencies, nil
}

// VersionMetadata 读取指定发行版的上传时间、仓库信息与依赖声明。
func (p *Provider) VersionMetadata(ctx context.Context, pkg model.PackageIdentity, version string) (engine.VersionMetadata, error) {
	document, source, err := p.fetchVersion(ctx, pkg, version)
	if err != nil {
		return engine.VersionMetadata{}, err
	}
	if document.Info.Version != "" && document.Info.Version != version {
		return engine.VersionMetadata{}, &engine.ProviderError{
			Provider: "pypi",
			Kind:     engine.ProviderErrorInvalidResponse,
			Err:      fmt.Errorf("project API returned version %q for requested version %q", document.Info.Version, version),
		}
	}

	limitations := make([]model.Limitation, 0)
	published, invalid := earliestUpload(document.URLs)
	if invalid {
		limitations = append(limitations, model.Limitation{
			Code:    "INVALID_PUBLISH_TIME",
			Message: "The PyPI API returned an unparseable upload time for the selected release.",
		})
	}
	if document.Info.Yanked {
		limitations = append(limitations, model.Limitation{
			Code:    "YANKED_RELEASE",
			Message: "The selected PyPI release is marked as yanked by its publisher.",
		})
	}

	dependencies, dependencyLimitations := parseRequiresDist(document.Info.RequiresDist)
	limitations = append(limitations, dependencyLimitations...)

	return engine.VersionMetadata{
		Package:       pkg,
		Version:       version,
		PublishedAt:   published,
		RepositoryURL: repositoryURL(document.Info),
		Dependencies:  dependencies,
		Source:        source,
		Limitations:   limitations,
	}, nil
}

func (p *Provider) fetchProject(ctx context.Context, pkg model.PackageIdentity) (projectDocument, model.SourceReference, error) {
	if pkg.Ecosystem != model.EcosystemPyPI {
		return projectDocument{}, model.SourceReference{}, fmt.Errorf("pypi provider does not support ecosystem %q", pkg.Ecosystem)
	}
	if err := pkg.Validate(); err != nil {
		return projectDocument{}, model.SourceReference{}, err
	}
	endpoint := p.baseURL + "/pypi/" + url.PathEscape(pkg.Name) + "/json"
	var document projectDocument
	retrievedAt, err := p.client.GetJSONWithRetrievedAt(ctx, "pypi", endpoint, &document)
	if err != nil {
		return projectDocument{}, model.SourceReference{}, err
	}
	return document, p.source(pkg, endpoint, retrievedAt), nil
}

func (p *Provider) fetchVersion(ctx context.Context, pkg model.PackageIdentity, version string) (projectDocument, model.SourceReference, error) {
	if pkg.Ecosystem != model.EcosystemPyPI {
		return projectDocument{}, model.SourceReference{}, fmt.Errorf("pypi provider does not support ecosystem %q", pkg.Ecosystem)
	}
	if err := pkg.Validate(); err != nil {
		return projectDocument{}, model.SourceReference{}, err
	}
	if strings.TrimSpace(version) == "" {
		return projectDocument{}, model.SourceReference{}, fmt.Errorf("version must not be empty")
	}
	endpoint := p.baseURL + "/pypi/" + url.PathEscape(pkg.Name) + "/" + url.PathEscape(version) + "/json"
	var document projectDocument
	retrievedAt, err := p.client.GetJSONWithRetrievedAt(ctx, "pypi", endpoint, &document)
	if err != nil {
		return projectDocument{}, model.SourceReference{}, err
	}
	return document, p.source(pkg, endpoint, retrievedAt), nil
}

func (p *Provider) source(pkg model.PackageIdentity, endpoint string, retrievedAt time.Time) model.SourceReference {
	return model.SourceReference{
		ID:          sourceID,
		Provider:    model.SourceProviderPyPI,
		Identifier:  pkg.Name,
		URL:         &endpoint,
		RetrievedAt: retrievedAt,
	}
}

type projectDocument struct {
	Info     projectInfo              `json:"info"`
	Releases map[string][]releaseFile `json:"releases"`
	URLs     []releaseFile            `json:"urls"`
}

type projectInfo struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	HomePage     string            `json:"home_page"`
	ProjectURLs  map[string]string `json:"project_urls"`
	RequiresDist []string          `json:"requires_dist"`
	Yanked       bool              `json:"yanked"`
}

type releaseFile struct {
	UploadTimeISO8601 string `json:"upload_time_iso_8601"`
	UploadTime        string `json:"upload_time"`
}

func earliestUpload(files []releaseFile) (*time.Time, bool) {
	var earliest *time.Time
	invalid := false
	for _, file := range files {
		value := strings.TrimSpace(file.UploadTimeISO8601)
		if value == "" {
			value = strings.TrimSpace(file.UploadTime)
		}
		if value == "" {
			continue
		}
		parsed, ok := parseTime(value)
		if !ok {
			invalid = true
			continue
		}
		if earliest == nil || parsed.Before(*earliest) {
			copy := parsed
			earliest = &copy
		}
	}
	return earliest, invalid
}

func repositoryURL(info projectInfo) *string {
	for _, key := range repositoryURLKeys {
		for candidate, value := range info.ProjectURLs {
			if strings.EqualFold(candidate, key) {
				if normalized := normalizeURL(value); normalized != nil {
					return normalized
				}
			}
		}
	}
	return normalizeURL(info.HomePage)
}

func normalizeURL(raw string) *string {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return nil
	}
	result := parsed.String()
	return &result
}

func parseRequiresDist(values []string) ([]engine.DependencyRequirement, []model.Limitation) {
	type declared struct {
		constraint string
		optional   bool
	}
	merged := make(map[string]declared, len(values))
	limitations := make([]model.Limitation, 0)

	for _, raw := range values {
		requirement := strings.TrimSpace(raw)
		if requirement == "" {
			continue
		}
		marker := ""
		if index := strings.Index(requirement, ";"); index >= 0 {
			marker = requirement[index+1:]
			requirement = strings.TrimSpace(requirement[:index])
		}
		optional := strings.Contains(strings.ToLower(marker), "extra")

		constraint := ""
		if index := strings.Index(requirement, "@"); index >= 0 {
			constraint = strings.TrimSpace(requirement[index:])
			requirement = strings.TrimSpace(requirement[:index])
		}
		if index := strings.Index(requirement, "["); index >= 0 {
			end := strings.Index(requirement[index:], "]")
			if end >= 0 {
				requirement = requirement[:index] + requirement[index+end+1:]
			} else {
				requirement = requirement[:index]
			}
		}
		name := requirement
		if constraint == "" {
			if index := strings.IndexAny(requirement, "<>=!~"); index >= 0 {
				name = requirement[:index]
				constraint = strings.TrimSpace(requirement[index:])
			}
		}
		name = strings.TrimSpace(strings.Trim(name, "()"))
		constraint = strings.TrimSpace(strings.Trim(constraint, "()"))
		if name == "" {
			continue
		}
		pkg, err := model.NewPackageIdentity(model.EcosystemPyPI, name)
		if err != nil {
			limitations = append(limitations, model.Limitation{
				Code:    "INVALID_DEPENDENCY_NAME",
				Message: fmt.Sprintf("Ignored an invalid PyPI dependency name %q.", name),
			})
			continue
		}
		current, exists := merged[pkg.Name]
		if !exists {
			merged[pkg.Name] = declared{constraint: constraint, optional: optional}
			continue
		}
		current.optional = current.optional || optional
		if current.constraint == "" {
			current.constraint = constraint
		}
		merged[pkg.Name] = current
	}

	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]engine.DependencyRequirement, 0, len(names))
	for _, name := range names {
		pkg, _ := model.NewPackageIdentity(model.EcosystemPyPI, name)
		result = append(result, engine.DependencyRequirement{
			Package:    pkg,
			Constraint: merged[name].constraint,
			Optional:   merged[name].optional,
		})
	}
	return result, limitations
}

func parseTime(value string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}
