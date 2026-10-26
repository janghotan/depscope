package model

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	npmSegmentPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9._~-]*$`)
	pypiSeparatorPattern = regexp.MustCompile(`[-_.]+`)
	pypiNamePattern      = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
)

// PackageIdentity 是跨生态系统使用的规范化包标识。
type PackageIdentity struct {
	Ecosystem Ecosystem `json:"ecosystem"`
	Name      string    `json:"name"`
}

// NewPackageIdentity 创建并规范化包标识。
func NewPackageIdentity(ecosystem Ecosystem, name string) (PackageIdentity, error) {
	canonical, err := canonicalName(ecosystem, name)
	if err != nil {
		return PackageIdentity{}, err
	}
	return PackageIdentity{Ecosystem: ecosystem, Name: canonical}, nil
}

// Validate 检查包标识是否有效且已规范化。
func (p PackageIdentity) Validate() error {
	if !p.Ecosystem.IsValid() {
		return fmt.Errorf("unsupported ecosystem %q", p.Ecosystem)
	}
	canonical, err := canonicalName(p.Ecosystem, p.Name)
	if err != nil {
		return err
	}
	if p.Name != canonical {
		return fmt.Errorf("package name %q is not canonical; use %q", p.Name, canonical)
	}
	return nil
}

// String 返回形如 npm:@scope/pkg 的稳定标识。
func (p PackageIdentity) String() string {
	return string(p.Ecosystem) + ":" + p.Name
}

func canonicalName(ecosystem Ecosystem, name string) (string, error) {
	if !ecosystem.IsValid() {
		return "", fmt.Errorf("unsupported ecosystem %q", ecosystem)
	}
	if name == "" {
		return "", fmt.Errorf("package name must not be empty")
	}
	if strings.TrimSpace(name) != name {
		return "", fmt.Errorf("package name must not contain leading or trailing whitespace")
	}

	switch ecosystem {
	case EcosystemNPM:
		return canonicalNPMName(name)
	case EcosystemPyPI:
		return canonicalPyPIName(name)
	default:
		return "", fmt.Errorf("unsupported ecosystem %q", ecosystem)
	}
}

func canonicalNPMName(name string) (string, error) {
	canonical := strings.ToLower(name)
	if strings.HasPrefix(canonical, "@") {
		parts := strings.Split(canonical, "/")
		if len(parts) != 2 {
			return "", fmt.Errorf("scoped npm package %q must contain exactly one slash", name)
		}
		scope := strings.TrimPrefix(parts[0], "@")
		if scope == "" || parts[1] == "" {
			return "", fmt.Errorf("scoped npm package %q must include a scope and a name", name)
		}
		if !npmSegmentPattern.MatchString(scope) || !npmSegmentPattern.MatchString(parts[1]) {
			return "", fmt.Errorf("npm package %q contains invalid characters", name)
		}
		return canonical, nil
	}

	if strings.Contains(canonical, "/") {
		return "", fmt.Errorf("unscoped npm package %q must not contain a slash", name)
	}
	if !npmSegmentPattern.MatchString(canonical) {
		return "", fmt.Errorf("npm package %q contains invalid characters", name)
	}
	return canonical, nil
}

func canonicalPyPIName(name string) (string, error) {
	canonical := pypiSeparatorPattern.ReplaceAllString(strings.ToLower(name), "-")
	if !pypiNamePattern.MatchString(canonical) {
		return "", fmt.Errorf("pypi package %q contains invalid characters", name)
	}
	return canonical, nil
}
