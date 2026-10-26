package app

import (
	"fmt"
	"strings"

	"github.com/janghotan/depscope/internal/model"
)

// ParsePackageSpec 解析交互表单与命令行使用的包标识。
// 支持 npm:pkg@1.2.3、pypi:requests==2.31.0 以及不带前缀的 name@version。
func ParsePackageSpec(input string, defaultEcosystem model.Ecosystem) (PackageRequest, error) {
	spec := strings.TrimSpace(input)
	if spec == "" {
		return PackageRequest{}, fmt.Errorf("package must not be empty")
	}
	if !defaultEcosystem.IsValid() {
		defaultEcosystem = model.EcosystemNPM
	}

	ecosystem := defaultEcosystem
	lower := strings.ToLower(spec)
	switch {
	case strings.HasPrefix(lower, "npm:"):
		ecosystem = model.EcosystemNPM
		spec = spec[len("npm:"):]
	case strings.HasPrefix(lower, "pypi:"):
		ecosystem = model.EcosystemPyPI
		spec = spec[len("pypi:"):]
	}
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return PackageRequest{}, fmt.Errorf("package must include a name")
	}

	name, version, err := splitPackageVersion(ecosystem, spec)
	if err != nil {
		return PackageRequest{}, err
	}
	return NewPackageRequest(ecosystem, name, version)
}

func splitPackageVersion(ecosystem model.Ecosystem, spec string) (string, string, error) {
	if ecosystem == model.EcosystemPyPI {
		if index := strings.Index(spec, "==="); index >= 0 {
			return splitAt(spec, index, len("==="))
		}
		if index := strings.Index(spec, "=="); index >= 0 {
			return splitAt(spec, index, len("=="))
		}
	}
	if index := versionSeparator(ecosystem, spec); index >= 0 {
		return splitAt(spec, index, 1)
	}
	return spec, "", nil
}

func versionSeparator(ecosystem model.Ecosystem, spec string) int {
	if ecosystem == model.EcosystemNPM && strings.HasPrefix(spec, "@") {
		if index := strings.Index(spec[1:], "@"); index >= 0 {
			return index + 1
		}
		return -1
	}
	return strings.LastIndex(spec, "@")
}

func splitAt(spec string, index, separatorLength int) (string, string, error) {
	name := strings.TrimSpace(spec[:index])
	version := strings.TrimSpace(spec[index+separatorLength:])
	if name == "" {
		return "", "", fmt.Errorf("package name must not be empty")
	}
	if version == "" {
		return "", "", fmt.Errorf("version must not be empty after a separator")
	}
	return name, version, nil
}
