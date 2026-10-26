// Package npm 实现 npm Registry 元数据提供者。
package npm

import (
	"context"
	"encoding/json"
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
	defaultBaseURL = "https://registry.npmjs.org"
	sourceID       = "npm-registry"
)

// Options 配置 npm 提供者。
type Options struct {
	BaseURL string
	Client  *fetch.Client
	Now     func() time.Time
}

// Provider 通过官方 npm Registry API 读取包元数据。
type Provider struct {
	baseURL string
	client  *fetch.Client
	now     func() time.Time
}

// NewProvider 创建 npm 提供者。
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

// Ecosystem 返回 npm。
func (p *Provider) Ecosystem() model.Ecosystem {
	return model.EcosystemNPM
}

// PackageMetadata 读取包级元数据、版本列表与最新版本。
func (p *Provider) PackageMetadata(ctx context.Context, pkg model.PackageIdentity) (engine.RegistryMetadata, error) {
	document, source, limitations, err := p.fetch(ctx, pkg)
	if err != nil {
		return engine.RegistryMetadata{}, err
	}

	versions := make([]string, 0, len(document.Versions))
	for version := range document.Versions {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	if len(versions) == 0 {
		limitations = append(limitations, model.Limitation{
			Code:    "EMPTY_VERSION_LIST",
			Message: "The npm registry returned no versions for this package.",
		})
	}

	var latest *string
	if value := strings.TrimSpace(document.DistTags["latest"]); value != "" {
		latest = &value
	}

	var latestPublishedAt *time.Time
	if latest != nil {
		published, ok := parseTime(document.Time[*latest])
		if !ok && strings.TrimSpace(document.Time[*latest]) != "" {
			limitations = append(limitations, model.Limitation{
				Code:    "INVALID_PUBLISH_TIME",
				Message: "The npm registry returned an unparseable publication time for the latest version.",
			})
		}
		latestPublishedAt = published
	}

	return engine.RegistryMetadata{
		Package:           pkg,
		Versions:          versions,
		LatestVersion:     latest,
		LatestPublishedAt: latestPublishedAt,
		RepositoryURL:     normalizeRepositoryURL(document.Repository.URL),
		Source:            source,
		Limitations:       limitations,
	}, nil
}

// Dependencies 返回指定版本声明的直接依赖需求。
func (p *Provider) Dependencies(ctx context.Context, pkg model.PackageIdentity, version string) ([]engine.DependencyRequirement, error) {
	metadata, err := p.VersionMetadata(ctx, pkg, version)
	if err != nil {
		return nil, err
	}
	return metadata.Dependencies, nil
}

// VersionMetadata 读取指定版本的依赖、脚本、发布日期与仓库信息。
func (p *Provider) VersionMetadata(ctx context.Context, pkg model.PackageIdentity, version string) (engine.VersionMetadata, error) {
	document, source, _, err := p.fetch(ctx, pkg)
	if err != nil {
		return engine.VersionMetadata{}, err
	}
	detail, exists := document.Versions[version]
	if !exists {
		return engine.VersionMetadata{}, &engine.ProviderError{
			Provider: "npm",
			Kind:     engine.ProviderErrorNotFound,
			Err:      fmt.Errorf("version %q not found for %s", version, pkg.Name),
		}
	}
	if detail.Version != "" && detail.Version != version {
		return engine.VersionMetadata{}, &engine.ProviderError{
			Provider: "npm",
			Kind:     engine.ProviderErrorInvalidResponse,
			Err:      fmt.Errorf("registry returned version %q for requested version %q", detail.Version, version),
		}
	}

	limitations := make([]model.Limitation, 0)
	published, ok := parseTime(document.Time[version])
	if !ok && strings.TrimSpace(document.Time[version]) != "" {
		limitations = append(limitations, model.Limitation{
			Code:    "INVALID_PUBLISH_TIME",
			Message: "The npm registry returned an unparseable publication time for the selected version.",
		})
	}

	repositoryURL := normalizeRepositoryURL(detail.Repository.URL)
	if repositoryURL == nil {
		repositoryURL = normalizeRepositoryURL(document.Repository.URL)
	}

	scripts := make([]engine.LifecycleScript, 0, 3)
	for _, name := range []string{"preinstall", "install", "postinstall"} {
		command := strings.TrimSpace(detail.Scripts[name])
		if command == "" {
			continue
		}
		scripts = append(scripts, engine.LifecycleScript{Name: name, Command: command})
	}

	dependencies, dependencyLimitations := parseDependencies(detail.Dependencies, detail.OptionalDependencies)
	limitations = append(limitations, dependencyLimitations...)

	return engine.VersionMetadata{
		Package:          pkg,
		Version:          version,
		PublishedAt:      published,
		RepositoryURL:    repositoryURL,
		Deprecated:       optionalString(detail.Deprecated),
		LifecycleScripts: scripts,
		Dependencies:     dependencies,
		Source:           source,
		Limitations:      limitations,
	}, nil
}

func (p *Provider) fetch(ctx context.Context, pkg model.PackageIdentity) (registryDocument, model.SourceReference, []model.Limitation, error) {
	if pkg.Ecosystem != model.EcosystemNPM {
		return registryDocument{}, model.SourceReference{}, nil, fmt.Errorf("npm provider does not support ecosystem %q", pkg.Ecosystem)
	}
	if err := pkg.Validate(); err != nil {
		return registryDocument{}, model.SourceReference{}, nil, err
	}

	endpoint := p.baseURL + "/" + url.PathEscape(pkg.Name)
	var document registryDocument
	if err := p.client.GetJSON(ctx, "npm", endpoint, &document); err != nil {
		return registryDocument{}, model.SourceReference{}, nil, err
	}
	if document.Name != "" && !strings.EqualFold(document.Name, pkg.Name) {
		return registryDocument{}, model.SourceReference{}, nil, &engine.ProviderError{
			Provider: "npm",
			Kind:     engine.ProviderErrorInvalidResponse,
			Err:      fmt.Errorf("registry returned package %q for requested package %q", document.Name, pkg.Name),
		}
	}

	limitations := make([]model.Limitation, 0)
	source := model.SourceReference{
		ID:          sourceID,
		Provider:    model.SourceProviderNPM,
		Identifier:  pkg.Name,
		URL:         &endpoint,
		RetrievedAt: p.now().UTC(),
	}
	return document, source, limitations, nil
}

type registryDocument struct {
	Name       string                     `json:"name"`
	DistTags   map[string]string          `json:"dist-tags"`
	Versions   map[string]versionDocument `json:"versions"`
	Time       map[string]string          `json:"time"`
	Repository repository                 `json:"repository"`
}

type versionDocument struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Dependencies         map[string]string `json:"dependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	Scripts              map[string]string `json:"scripts"`
	Deprecated           string            `json:"deprecated"`
	Repository           repository        `json:"repository"`
}

type repository struct {
	URL string `json:"url"`
}

// UnmarshalJSON 兼容 npm 元数据中字符串与对象两种 repository 形式。
func (r *repository) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		r.URL = raw
		return nil
	}
	var object struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	r.URL = object.URL
	return nil
}

func parseDependencies(required, optional map[string]string) ([]engine.DependencyRequirement, []model.Limitation) {
	type declared struct {
		constraint string
		optional   bool
	}
	merged := make(map[string]declared, len(required)+len(optional))
	limitations := make([]model.Limitation, 0)

	addAll := func(values map[string]string, isOptional bool) {
		names := make([]string, 0, len(values))
		for name := range values {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			pkg, err := model.NewPackageIdentity(model.EcosystemNPM, name)
			if err != nil {
				limitations = append(limitations, model.Limitation{
					Code:    "INVALID_DEPENDENCY_NAME",
					Message: fmt.Sprintf("Ignored an invalid npm dependency name %q.", name),
				})
				continue
			}
			current, exists := merged[pkg.Name]
			if !exists {
				merged[pkg.Name] = declared{constraint: values[name], optional: isOptional}
				continue
			}
			current.optional = current.optional || isOptional
			if current.constraint == "" {
				current.constraint = values[name]
			}
			merged[pkg.Name] = current
		}
	}
	addAll(required, false)
	addAll(optional, true)

	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]engine.DependencyRequirement, 0, len(names))
	for _, name := range names {
		pkg, _ := model.NewPackageIdentity(model.EcosystemNPM, name)
		result = append(result, engine.DependencyRequirement{
			Package:    pkg,
			Constraint: merged[name].constraint,
			Optional:   merged[name].optional,
		})
	}
	return result, limitations
}

func normalizeRepositoryURL(raw string) *string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	switch {
	case strings.HasPrefix(value, "github:"):
		value = "https://github.com/" + strings.TrimPrefix(value, "github:")
	case strings.HasPrefix(value, "git+"):
		value = strings.TrimPrefix(value, "git+")
	case strings.HasPrefix(value, "git://"):
		value = "https://" + strings.TrimPrefix(value, "git://")
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return nil
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, ".git")
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	result := parsed.String()
	return &result
}

func parseTime(value string) (*time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, false
	}
	parsed = parsed.UTC()
	return &parsed, true
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
