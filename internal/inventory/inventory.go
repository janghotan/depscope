// Package inventory 以只读方式枚举本机已安装的 npm 与 Python 包，并复用共享安全引擎匹配漏洞。
// 本包只读取已安装包的元数据或调用包管理器文档化的只读列表命令；
// 它绝不执行已安装包、项目脚本、生命周期脚本或安装命令。
package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"depscope/internal/engine"
	"depscope/internal/model"
)

const (
	checkNPMDetection    = "npm:global-detection"
	checkPythonDetection = "python:global-detection"
	checkVulnerabilities = "vulnerabilities"
	checkRiskEvaluation  = "risk-evaluation"

	inventoryNPMSourceID    = "inventory:npm"
	inventoryPythonSourceID = "inventory:pypi"

	managerNPM = "npm"
	managerPip = "pip"

	defaultMaxPackages = 10000
	defaultMaxFileSize = 4 << 20
)

// Package 表示包管理器报告的已安装包名称与版本。
type Package struct {
	Name    string
	Version string
}

// NPMRootFinder 返回 npm 全局 node_modules 目录，供只读元数据枚举使用。
type NPMRootFinder interface {
	NPMGlobalRoot(ctx context.Context) (string, error)
}

// PythonLister 返回当前默认 Python 环境的已安装包列表。
type PythonLister interface {
	PipPackages(ctx context.Context) ([]Package, error)
}

// Limits 限制枚举的包数量与单个元数据文件大小；非正数字段会替换为安全默认值。
type Limits struct {
	MaxPackages int
	MaxFileSize int64
}

// DefaultLimits 返回全局清单枚举的保守默认限制。
func DefaultLimits() Limits {
	return Limits{MaxPackages: defaultMaxPackages, MaxFileSize: defaultMaxFileSize}
}

func normalizeLimits(limits Limits) Limits {
	defaults := DefaultLimits()
	if limits.MaxPackages <= 0 {
		limits.MaxPackages = defaults.MaxPackages
	}
	if limits.MaxFileSize <= 0 {
		limits.MaxFileSize = defaults.MaxFileSize
	}
	return limits
}

// InstalledPackage 表示一个已确认安装的确切包版本及其元数据来源。
type InstalledPackage struct {
	Ecosystem  model.Ecosystem
	Package    model.PackageIdentity
	Version    string
	Manager    string
	SourcePath string
}

// Stage 表示全局清单扫描的进度阶段。
type Stage string

const (
	// StageList 表示枚举已安装包。
	StageList Stage = "list"
	// StageScan 表示查询漏洞并评估风险。
	StageScan Stage = "scan"
	// StageAssemble 表示组装最终结果。
	StageAssemble Stage = "assemble"
)

// Progress 表示一次进度通知。
type Progress struct {
	Stage     Stage
	Message   string
	Completed int
	Total     int
}

// Options 配置全局清单扫描所需的引擎、限制、时钟与包管理器发现方式。
type Options struct {
	Limits   Limits
	Scanner  *engine.Scanner
	Clock    func() time.Time
	Progress func(Progress)
	NPM      NPMRootFinder
	Python   PythonLister
}

// Result 复用共享的发现、检查、来源与限制模型，并列出已安装包。
type Result struct {
	ScannedAt   time.Time
	Status      model.ScanStatus
	Packages    []InstalledPackage
	Findings    []model.Finding
	Checks      []model.CheckResult
	Sources     []model.SourceReference
	Limitations []model.Limitation
	Summary     model.Summary
}

type managerOutcome struct {
	packages        []InstalledPackage
	limitations     []model.Limitation
	source          *model.SourceReference
	issues          bool
	detectionFailed bool
	reason          string
}

type inventoryStats struct {
	ecosystem   model.Ecosystem
	resolved    int
	failed      int
	advisories  int
	riskMissing int
	sourceIDs   []string
	seen        map[string]struct{}
}

// Scan 枚举已安装的 npm 与 Python 包，并对确切版本执行漏洞匹配。
// 平台或包管理器检测失败会转换为检查、限制或最终状态；只有取消与调用错误才返回 error。
func Scan(ctx context.Context, opts Options) (Result, error) {
	if opts.Scanner == nil {
		return Result{}, errors.New("inventory scan requires an engine scanner")
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	limits := normalizeLimits(opts.Limits)
	result := Result{
		ScannedAt:   clock().UTC(),
		Status:      model.StatusRunning,
		Packages:    []InstalledPackage{},
		Findings:    []model.Finding{},
		Checks:      []model.CheckResult{},
		Sources:     []model.SourceReference{},
		Limitations: []model.Limitation{},
	}
	if err := ctx.Err(); err != nil {
		result.Status = model.StatusCancelled
		return result, err
	}

	npmConfigured := opts.NPM != nil
	pythonConfigured := opts.Python != nil
	remaining := limits.MaxPackages
	var npmOutcome, pythonOutcome managerOutcome

	if !npmConfigured {
		addResultCheck(&result, model.CheckResult{
			ID:        checkNPMDetection,
			Name:      "npm global package detection",
			Status:    model.CheckUnavailable,
			Message:   "npm global inventory is not configured; no npm packages were inspected.",
			SourceIDs: []string{},
		})
		addResultLimitation(&result, "NPM_NOT_CONFIGURED", "npm global inventory was not available, so no npm packages were inspected.")
	} else {
		npmOutcome = collectNPM(ctx, opts.NPM, limits, remaining, result.ScannedAt)
		if err := ctx.Err(); err != nil {
			result.Status = model.StatusCancelled
			return result, err
		}
		applyNPMOutcome(&result, npmOutcome)
		remaining -= len(npmOutcome.packages)
		if remaining < 0 {
			remaining = 0
		}
	}

	if !pythonConfigured {
		addResultCheck(&result, model.CheckResult{
			ID:        checkPythonDetection,
			Name:      "Python global package detection",
			Status:    model.CheckUnavailable,
			Message:   "Python global inventory is not configured; no Python packages were inspected.",
			SourceIDs: []string{},
		})
		addResultLimitation(&result, "PYTHON_NOT_CONFIGURED", "Python global inventory was not available, so no Python packages were inspected.")
	} else {
		pythonOutcome = collectPython(ctx, opts.Python, limits, remaining, result.ScannedAt)
		if err := ctx.Err(); err != nil {
			result.Status = model.StatusCancelled
			return result, err
		}
		applyPythonOutcome(&result, pythonOutcome)
	}

	npmListed := npmConfigured && !npmOutcome.detectionFailed
	pythonListed := pythonConfigured && !pythonOutcome.detectionFailed
	if npmListed && len(npmOutcome.packages) == 0 {
		addResultLimitation(&result, "NO_INSTALLED_PACKAGES", "npm global inventory reported no installed packages; no npm vulnerability coverage is available.")
	}
	if pythonListed && len(pythonOutcome.packages) == 0 {
		addResultLimitation(&result, "NO_INSTALLED_PACKAGES", "pip reported no installed packages; no Python vulnerability coverage is available.")
	}

	installed := make([]InstalledPackage, 0, len(npmOutcome.packages)+len(pythonOutcome.packages))
	installed = append(installed, npmOutcome.packages...)
	installed = append(installed, pythonOutcome.packages...)
	installed = dedupeInstalled(installed)
	sortInstalled(installed)
	result.Packages = installed

	notify(opts.Progress, Progress{
		Stage:   StageList,
		Message: fmt.Sprintf("Listed %d installed package(s) from local package metadata.", len(installed)),
		Total:   len(installed),
	})

	packages := resolvedPackages(installed)
	active := activeEcosystems(npmConfigured, npmOutcome, pythonConfigured, pythonOutcome)
	if len(packages) > 0 {
		notify(opts.Progress, Progress{
			Stage:   StageScan,
			Message: fmt.Sprintf("Querying OSV for %d installed package version(s).", len(packages)),
			Total:   len(packages),
		})
		batch, batchErr := opts.Scanner.ScanResolvedBatch(ctx, packages)
		if batchErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				result.Status = model.StatusCancelled
				return result, ctxErr
			}
			result.Status = model.StatusFailed
			return result, fmt.Errorf("batch vulnerability scan: %w", batchErr)
		}
		mergeBatchResults(&result, installed, batch, active)
		notify(opts.Progress, Progress{
			Stage:     StageScan,
			Message:   "Vulnerability and risk checks completed.",
			Completed: len(packages),
			Total:     len(packages),
		})
	} else {
		mergeBatchResults(&result, installed, nil, active)
	}

	fatal := npmConfigured && pythonConfigured && npmOutcome.detectionFailed && pythonOutcome.detectionFailed
	finalizeResult(&result, fatal)
	notify(opts.Progress, Progress{
		Stage:   StageAssemble,
		Message: fmt.Sprintf("Assembled global inventory result with status %s.", result.Status),
	})
	return result, nil
}

func activeEcosystems(npmConfigured bool, npm managerOutcome, pythonConfigured bool, python managerOutcome) []model.Ecosystem {
	active := []model.Ecosystem{}
	if npmConfigured && !npm.detectionFailed {
		active = append(active, model.EcosystemNPM)
	}
	if pythonConfigured && !python.detectionFailed {
		active = append(active, model.EcosystemPyPI)
	}
	sort.Slice(active, func(i, j int) bool { return active[i] < active[j] })
	return active
}

func applyNPMOutcome(result *Result, outcome managerOutcome) {
	if outcome.detectionFailed {
		message := fmt.Sprintf("npm global package detection failed: %s", outcome.reason)
		addResultCheck(result, model.CheckResult{
			ID:        checkNPMDetection,
			Name:      "npm global package detection",
			Status:    model.CheckUnavailable,
			Message:   message,
			SourceIDs: []string{},
		})
		addResultLimitation(result, "NPM_GLOBAL_DETECTION_FAILED", message)
		return
	}

	status := model.CheckSucceeded
	message := fmt.Sprintf("Read package metadata for %d top-level npm global package(s).", len(outcome.packages))
	sourceIDs := []string{}
	if outcome.source != nil {
		if addResultSource(result, *outcome.source) {
			sourceIDs = append(sourceIDs, outcome.source.ID)
			addResultLimitation(result, "NPM_GLOBAL_TOP_LEVEL_ONLY", "Only top-level packages in the npm global root were inventoried; nested node_modules trees were not scanned.")
		}
	}
	if outcome.issues {
		status = model.CheckUnavailable
		message = fmt.Sprintf("Read package metadata for %d top-level npm global package(s), but some installed packages could not be inventoried; coverage is incomplete.", len(outcome.packages))
	}
	for _, limitation := range outcome.limitations {
		addResultLimitation(result, limitation.Code, limitation.Message)
	}
	addResultCheck(result, model.CheckResult{
		ID:        checkNPMDetection,
		Name:      "npm global package detection",
		Status:    status,
		Message:   message,
		SourceIDs: sourceIDs,
	})
}

func applyPythonOutcome(result *Result, outcome managerOutcome) {
	if outcome.detectionFailed {
		message := fmt.Sprintf("Python global package detection failed: %s", outcome.reason)
		addResultCheck(result, model.CheckResult{
			ID:        checkPythonDetection,
			Name:      "Python global package detection",
			Status:    model.CheckUnavailable,
			Message:   message,
			SourceIDs: []string{},
		})
		addResultLimitation(result, "PYTHON_GLOBAL_DETECTION_FAILED", message)
		return
	}

	status := model.CheckSucceeded
	message := fmt.Sprintf("pip reported %d installed package(s).", len(outcome.packages))
	sourceIDs := []string{}
	if outcome.source != nil {
		if addResultSource(result, *outcome.source) {
			sourceIDs = append(sourceIDs, outcome.source.ID)
			addResultLimitation(result, "PYTHON_SINGLE_ENVIRONMENT", "Python inventory reflects only the default pip environment; virtual environments and other interpreters were not inspected.")
		}
	}
	if outcome.issues {
		status = model.CheckUnavailable
		message = fmt.Sprintf("pip reported %d installed package(s), but some metadata could not be inventoried; coverage is incomplete.", len(outcome.packages))
	}
	for _, limitation := range outcome.limitations {
		addResultLimitation(result, limitation.Code, limitation.Message)
	}
	addResultCheck(result, model.CheckResult{
		ID:        checkPythonDetection,
		Name:      "Python global package detection",
		Status:    status,
		Message:   message,
		SourceIDs: sourceIDs,
	})
}

func collectNPM(ctx context.Context, finder NPMRootFinder, limits Limits, remaining int, now time.Time) managerOutcome {
	root, err := finder.NPMGlobalRoot(ctx)
	if err != nil {
		return managerOutcome{detectionFailed: true, reason: err.Error()}
	}
	packages, limitations, issues, err := readNPMGlobalPackages(ctx, root, limits, remaining)
	if err != nil {
		return managerOutcome{detectionFailed: true, reason: err.Error()}
	}
	source := model.SourceReference{
		ID:          inventoryNPMSourceID,
		Provider:    model.SourceProviderNPM,
		Identifier:  "npm global package metadata",
		RetrievedAt: now,
	}
	return managerOutcome{
		packages:    packages,
		limitations: limitations,
		source:      &source,
		issues:      issues,
	}
}

// readNPMGlobalPackages 只枚举 npm 全局根目录的顶层包，不递归 node_modules。
func readNPMGlobalPackages(ctx context.Context, root string, limits Limits, remaining int) ([]InstalledPackage, []model.Limitation, bool, error) {
	limitations := []model.Limitation{}
	add := limitationCollector(&limitations)
	issues := false

	info, err := os.Stat(root)
	if err != nil {
		return nil, limitations, false, fmt.Errorf("inspect npm global root: %w", err)
	}
	if !info.IsDir() {
		return nil, limitations, false, fmt.Errorf("npm global root %q is not a directory", root)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, limitations, false, fmt.Errorf("read npm global root: %w", err)
	}

	type candidate struct {
		rel string
		dir string
	}
	candidates := []candidate{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, limitations, issues, err
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if isSymlink(entry) {
			add("SYMLINKED_PACKAGE_SKIPPED", fmt.Sprintf("npm global package %q is a symbolic link and was not followed.", name))
			issues = true
			continue
		}
		if !entry.IsDir() {
			continue
		}

		dir := filepath.Join(root, name)
		if strings.HasPrefix(name, "@") {
			scopeEntries, err := os.ReadDir(dir)
			if err != nil {
				add("PACKAGE_METADATA_UNREADABLE", fmt.Sprintf("npm scope %q could not be read: %v", name, err))
				issues = true
				continue
			}
			for _, scopeEntry := range scopeEntries {
				sub := scopeEntry.Name()
				if strings.HasPrefix(sub, ".") {
					continue
				}
				rel := name + "/" + sub
				if isSymlink(scopeEntry) {
					add("SYMLINKED_PACKAGE_SKIPPED", fmt.Sprintf("npm global package %q is a symbolic link and was not followed.", rel))
					issues = true
					continue
				}
				if !scopeEntry.IsDir() {
					continue
				}
				candidates = append(candidates, candidate{rel: rel, dir: filepath.Join(dir, sub)})
			}
			continue
		}
		candidates = append(candidates, candidate{rel: name, dir: dir})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].rel < candidates[j].rel })

	packages := []InstalledPackage{}
	seen := make(map[string]InstalledPackage)
	for _, item := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, limitations, issues, err
		}
		if len(packages) >= remaining {
			add("PACKAGE_LIMIT_EXCEEDED", fmt.Sprintf("Inventory records at most %d packages; remaining npm global packages were not read.", limits.MaxPackages))
			issues = true
			break
		}

		metadataPath := filepath.Join(item.dir, "package.json")
		metadataInfo, err := os.Lstat(metadataPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				add("PACKAGE_METADATA_MISSING", fmt.Sprintf("npm global package %q has no package.json metadata.", item.rel))
			} else {
				add("PACKAGE_METADATA_UNREADABLE", fmt.Sprintf("npm global package %q metadata could not be inspected: %v", item.rel, err))
			}
			issues = true
			continue
		}
		if metadataInfo.Mode()&os.ModeSymlink != 0 {
			add("SYMLINKED_PACKAGE_SKIPPED", fmt.Sprintf("npm global package %q package.json is a symbolic link and was not followed.", item.rel))
			issues = true
			continue
		}
		if !metadataInfo.Mode().IsRegular() {
			add("PACKAGE_METADATA_UNREADABLE", fmt.Sprintf("npm global package %q package.json is not a regular file.", item.rel))
			issues = true
			continue
		}
		if metadataInfo.Size() > limits.MaxFileSize {
			add("PACKAGE_METADATA_TOO_LARGE", fmt.Sprintf("npm global package %q metadata is %d bytes, exceeding the %d byte limit.", item.rel, metadataInfo.Size(), limits.MaxFileSize))
			issues = true
			continue
		}
		data, err := readLimitedFile(metadataPath, limits.MaxFileSize)
		if err != nil {
			if errors.Is(err, errMetadataTooLarge) {
				add("PACKAGE_METADATA_TOO_LARGE", fmt.Sprintf("npm global package %q metadata exceeds the %d byte limit.", item.rel, limits.MaxFileSize))
			} else {
				add("PACKAGE_METADATA_UNREADABLE", fmt.Sprintf("npm global package %q metadata could not be read: %v", item.rel, err))
			}
			issues = true
			continue
		}

		metadata, err := decodeNPMInstallMetadata(data)
		if err != nil {
			add("MALFORMED_PACKAGE_METADATA", fmt.Sprintf("npm global package %q metadata is not valid: %v", item.rel, err))
			issues = true
			continue
		}
		name := strings.TrimSpace(metadata.Name)
		if name == "" {
			add("INVALID_PACKAGE_METADATA", fmt.Sprintf("npm global package %q has an empty name.", item.rel))
			issues = true
			continue
		}
		identity, err := model.NewPackageIdentity(model.EcosystemNPM, name)
		if err != nil {
			add("INVALID_PACKAGE_METADATA", fmt.Sprintf("npm global package %q has an invalid name %q: %v", item.rel, metadata.Name, err))
			issues = true
			continue
		}
		version, ok := normalizeInstalledVersion(metadata.Version)
		if !ok {
			if strings.TrimSpace(metadata.Version) == "" {
				add("UNRESOLVED_VERSION", fmt.Sprintf("npm global package %s has no version and was not matched.", identity))
			} else {
				add("INVALID_PACKAGE_METADATA", fmt.Sprintf("npm global package %s has an invalid version %q.", identity, metadata.Version))
			}
			issues = true
			continue
		}

		if existing, exists := seen[identity.String()]; exists {
			if existing.Version != version {
				add("CONFLICTING_PACKAGE_METADATA", fmt.Sprintf("npm global metadata reports conflicting versions for %s (%s and %s); the later copy was ignored.", identity, existing.Version, version))
				issues = true
			}
			continue
		}
		installedItem := InstalledPackage{
			Ecosystem:  model.EcosystemNPM,
			Package:    identity,
			Version:    version,
			Manager:    managerNPM,
			SourcePath: item.dir,
		}
		seen[identity.String()] = installedItem
		packages = append(packages, installedItem)
	}
	return packages, limitations, issues, nil
}

func collectPython(ctx context.Context, lister PythonLister, limits Limits, remaining int, now time.Time) managerOutcome {
	listed, err := lister.PipPackages(ctx)
	if err != nil {
		return managerOutcome{detectionFailed: true, reason: err.Error()}
	}

	limitations := []model.Limitation{}
	add := limitationCollector(&limitations)
	issues := false
	type pythonGroup struct {
		identity model.PackageIdentity
		versions map[string]struct{}
	}
	groups := make(map[string]*pythonGroup)
	order := []string{}

	for index, pkg := range listed {
		if index%128 == 0 {
			if err := ctx.Err(); err != nil {
				return managerOutcome{detectionFailed: true, reason: err.Error()}
			}
		}
		name := strings.TrimSpace(pkg.Name)
		if name == "" {
			add("INVALID_PACKAGE_METADATA", fmt.Sprintf("pip reported a package with an empty name at position %d.", index+1))
			issues = true
			continue
		}
		identity, err := model.NewPackageIdentity(model.EcosystemPyPI, name)
		if err != nil {
			add("INVALID_PACKAGE_METADATA", fmt.Sprintf("pip reported an invalid package name %q: %v", pkg.Name, err))
			issues = true
			continue
		}
		version, ok := normalizeInstalledVersion(pkg.Version)
		if !ok {
			if strings.TrimSpace(pkg.Version) == "" {
				add("UNRESOLVED_VERSION", fmt.Sprintf("pip reported no version for %s; it was not matched.", identity))
			} else {
				add("INVALID_PACKAGE_METADATA", fmt.Sprintf("pip reported an invalid version for %s.", identity))
			}
			issues = true
			continue
		}
		key := identity.String()
		group := groups[key]
		if group == nil {
			group = &pythonGroup{identity: identity, versions: make(map[string]struct{})}
			groups[key] = group
			order = append(order, key)
		}
		group.versions[version] = struct{}{}
	}
	sort.Strings(order)

	packages := []InstalledPackage{}
	for _, key := range order {
		if err := ctx.Err(); err != nil {
			return managerOutcome{detectionFailed: true, reason: err.Error()}
		}
		if len(packages) >= remaining {
			add("PACKAGE_LIMIT_EXCEEDED", fmt.Sprintf("Inventory records at most %d packages; remaining Python packages were not read.", limits.MaxPackages))
			issues = true
			break
		}
		group := groups[key]
		if len(group.versions) > 1 {
			versions := make([]string, 0, len(group.versions))
			for version := range group.versions {
				versions = append(versions, version)
			}
			sort.Strings(versions)
			add("CONFLICTING_PACKAGE_METADATA", fmt.Sprintf("pip reported conflicting versions for %s (%s); the package was skipped.", key, strings.Join(versions, ", ")))
			issues = true
			continue
		}
		version := ""
		for value := range group.versions {
			version = value
		}
		packages = append(packages, InstalledPackage{
			Ecosystem: model.EcosystemPyPI,
			Package:   group.identity,
			Version:   version,
			Manager:   managerPip,
		})
	}

	source := model.SourceReference{
		ID:          inventoryPythonSourceID,
		Provider:    model.SourceProviderPyPI,
		Identifier:  "pip list --format=json",
		RetrievedAt: now,
	}
	return managerOutcome{
		packages:    packages,
		limitations: limitations,
		source:      &source,
		issues:      issues,
	}
}

type npmInstallMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// errMetadataTooLarge 表示元数据文件在读取过程中超出了允许的大小。
var errMetadataTooLarge = errors.New("metadata exceeds size limit")

func readLimitedFile(path string, maxSize int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, errMetadataTooLarge
	}
	return data, nil
}

func decodeNPMInstallMetadata(data []byte) (npmInstallMetadata, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var metadata npmInstallMetadata
	if err := decoder.Decode(&metadata); err != nil {
		return npmInstallMetadata{}, fmt.Errorf("parse package.json: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return npmInstallMetadata{}, errors.New("package.json contains trailing data")
		}
		return npmInstallMetadata{}, fmt.Errorf("parse package.json: %w", err)
	}
	return metadata, nil
}

func isSymlink(entry os.DirEntry) bool {
	return entry.Type()&os.ModeSymlink != 0
}

func normalizeInstalledVersion(value string) (string, bool) {
	version := strings.TrimSpace(value)
	if version == "" {
		return "", false
	}
	for _, r := range version {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", false
		}
	}
	return version, true
}

func dedupeInstalled(packages []InstalledPackage) []InstalledPackage {
	seen := make(map[string]struct{}, len(packages))
	result := make([]InstalledPackage, 0, len(packages))
	for _, item := range packages {
		key := strings.Join([]string{item.Package.String(), item.Version, item.Manager, item.SourcePath}, "\x00")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func sortInstalled(packages []InstalledPackage) {
	sort.SliceStable(packages, func(i, j int) bool {
		left, right := packages[i], packages[j]
		if left.Ecosystem != right.Ecosystem {
			return left.Ecosystem < right.Ecosystem
		}
		if left.Package.String() != right.Package.String() {
			return left.Package.String() < right.Package.String()
		}
		if left.Version != right.Version {
			return left.Version < right.Version
		}
		if left.Manager != right.Manager {
			return left.Manager < right.Manager
		}
		return left.SourcePath < right.SourcePath
	})
}

func resolvedPackages(packages []InstalledPackage) []engine.ResolvedPackage {
	seen := make(map[string]struct{}, len(packages))
	resolved := make([]engine.ResolvedPackage, 0, len(packages))
	for _, item := range packages {
		key := item.Package.String() + "@" + item.Version
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		resolved = append(resolved, engine.ResolvedPackage{Package: item.Package, Version: item.Version})
	}
	sort.Slice(resolved, func(i, j int) bool {
		if resolved[i].Package.String() != resolved[j].Package.String() {
			return resolved[i].Package.String() < resolved[j].Package.String()
		}
		return resolved[i].Version < resolved[j].Version
	})
	return resolved
}

func mergeBatchResults(result *Result, installed []InstalledPackage, batch []engine.PackageScanResult, active []model.Ecosystem) {
	stats := make(map[model.Ecosystem]*inventoryStats, len(active))
	for _, ecosystem := range active {
		stats[ecosystem] = &inventoryStats{ecosystem: ecosystem, seen: make(map[string]struct{})}
	}
	byKey := make(map[string]InstalledPackage, len(installed))
	for _, item := range installed {
		key := item.Package.String() + "@" + item.Version
		byKey[key] = item
		itemStats := stats[item.Package.Ecosystem]
		if itemStats == nil {
			itemStats = &inventoryStats{ecosystem: item.Package.Ecosystem, seen: make(map[string]struct{})}
			stats[item.Package.Ecosystem] = itemStats
		}
		if _, exists := itemStats.seen[key]; !exists {
			itemStats.seen[key] = struct{}{}
			itemStats.resolved++
		}
	}

	for _, item := range batch {
		itemStats := stats[item.Package.Ecosystem]
		if itemStats == nil {
			itemStats = &inventoryStats{ecosystem: item.Package.Ecosystem, seen: make(map[string]struct{})}
			stats[item.Package.Ecosystem] = itemStats
		}

		remapped := make(map[string]string, len(item.Sources))
		for _, source := range item.Sources {
			if strings.TrimSpace(source.ID) == "" {
				continue
			}
			originalID := source.ID
			source.ID = fmt.Sprintf("%s:%s@%s", originalID, item.Package.String(), item.Version)
			if _, exists := remapped[originalID]; !exists {
				remapped[originalID] = source.ID
			}
			if addResultSource(result, source) {
				itemStats.sourceIDs = append(itemStats.sourceIDs, source.ID)
			}
		}

		for _, finding := range item.Findings {
			for index := range finding.Evidence {
				if newID, ok := remapped[finding.Evidence[index].SourceID]; ok {
					finding.Evidence[index].SourceID = newID
				}
			}
			finding.ID = inventoryFindingID(finding)
			key := finding.Package.String()
			if finding.Version != nil && *finding.Version != "" {
				key += "@" + *finding.Version
			}
			if installedItem, ok := byKey[key]; ok {
				sourceID := inventorySourceForManager(installedItem.Manager)
				if sourceExists(result, sourceID) {
					finding.Evidence = append(finding.Evidence, model.Evidence{
						Kind:     model.EvidenceRegistry,
						Summary:  fmt.Sprintf("Installed metadata from %s reports %s@%s.", installedItem.Manager, finding.Package.String(), versionOrUnknown(finding.Version)),
						SourceID: sourceID,
					})
				} else {
					addResultLimitation(result, "INVALID_SOURCE_REFERENCE", fmt.Sprintf("Finding %q could not be attributed to installed package metadata.", finding.ID))
				}
			} else {
				addResultLimitation(result, "FINDING_WITHOUT_INSTALLED_PACKAGE", fmt.Sprintf("Finding %q does not match any inventoried installed package.", finding.ID))
			}
			for _, evidence := range finding.Evidence {
				if !sourceExists(result, evidence.SourceID) {
					addResultLimitation(result, "INVALID_SOURCE_REFERENCE", fmt.Sprintf("Finding %q has evidence with an unknown source reference %q.", finding.ID, evidence.SourceID))
					break
				}
			}
			result.Findings = append(result.Findings, finding)
			itemStats.advisories++
			if finding.Risk == nil {
				itemStats.riskMissing++
			}
		}

		vulnerabilityCheck := findBatchCheck(item.Checks, checkVulnerabilities)
		if item.Err != nil || vulnerabilityCheck == nil || vulnerabilityCheck.Status != model.CheckSucceeded {
			itemStats.failed++
		}
		for _, limitation := range item.Limitations {
			addResultLimitation(result, limitation.Code, limitation.Message)
		}
	}

	ecosystems := make([]model.Ecosystem, 0, len(stats))
	for ecosystem := range stats {
		ecosystems = append(ecosystems, ecosystem)
	}
	sort.Slice(ecosystems, func(i, j int) bool { return ecosystems[i] < ecosystems[j] })

	for _, ecosystem := range ecosystems {
		itemStats := stats[ecosystem]
		status, message := inventoryVulnerabilityCheck(itemStats)
		addResultCheck(result, model.CheckResult{
			ID:        ecosystemCheckID(ecosystem, checkVulnerabilities),
			Name:      "OSV vulnerability query",
			Status:    status,
			Message:   message,
			SourceIDs: dedupeStrings(itemStats.sourceIDs),
		})
		if itemStats.advisories > 0 {
			riskStatus := model.CheckSucceeded
			riskMessage := fmt.Sprintf("Risk scores were computed for all %d %s finding(s).", itemStats.advisories, ecosystem)
			if itemStats.riskMissing > 0 {
				riskStatus = model.CheckFailed
				riskMessage = fmt.Sprintf("Risk scoring was unavailable for some %s findings; they are retained without risk scores.", ecosystem)
			}
			addResultCheck(result, model.CheckResult{
				ID:        ecosystemCheckID(ecosystem, checkRiskEvaluation),
				Name:      "Risk evaluation",
				Status:    riskStatus,
				Message:   riskMessage,
				SourceIDs: []string{},
			})
		}
	}
}

func inventoryVulnerabilityCheck(stats *inventoryStats) (model.CheckStatus, string) {
	if stats.resolved == 0 {
		return model.CheckUnavailable, fmt.Sprintf("No installed %s package versions were available for vulnerability matching; coverage is incomplete.", stats.ecosystem)
	}
	if stats.failed > 0 {
		return model.CheckUnavailable, fmt.Sprintf("OSV data for %d of %d installed %s package version(s) was unavailable or incomplete; coverage is incomplete.", stats.failed, stats.resolved, stats.ecosystem)
	}
	if stats.advisories == 0 {
		return model.CheckSucceeded, fmt.Sprintf("OSV returned no known advisories affecting %d installed %s package version(s).", stats.resolved, stats.ecosystem)
	}
	return model.CheckSucceeded, fmt.Sprintf("OSV returned %d advisory(ies) affecting %d installed %s package version(s).", stats.advisories, stats.resolved, stats.ecosystem)
}

func inventorySourceForManager(manager string) string {
	if manager == managerPip {
		return inventoryPythonSourceID
	}
	return inventoryNPMSourceID
}

func ecosystemCheckID(ecosystem model.Ecosystem, suffix string) string {
	prefix := string(ecosystem)
	if ecosystem == model.EcosystemPyPI {
		prefix = "python"
	}
	return prefix + ":" + suffix
}

func inventoryFindingID(finding model.Finding) string {
	parts := []string{finding.ID, finding.Package.String()}
	if finding.Version != nil && *finding.Version != "" {
		parts = append(parts, *finding.Version)
	}
	return strings.Join(parts, "@")
}

func versionOrUnknown(version *string) string {
	if version == nil || *version == "" {
		return "(version unavailable)"
	}
	return *version
}

func sourceExists(result *Result, id string) bool {
	if strings.TrimSpace(id) == "" {
		return false
	}
	for _, source := range result.Sources {
		if source.ID == id {
			return true
		}
	}
	return false
}

func findBatchCheck(checks []model.CheckResult, id string) *model.CheckResult {
	for index := range checks {
		if checks[index].ID == id {
			return &checks[index]
		}
	}
	return nil
}

func limitationCollector(limitations *[]model.Limitation) func(code, message string) {
	seen := make(map[string]struct{})
	return func(code, message string) {
		code = strings.TrimSpace(code)
		message = strings.TrimSpace(message)
		if code == "" || message == "" {
			return
		}
		key := code + "\x00" + message
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		*limitations = append(*limitations, model.Limitation{Code: code, Message: message})
	}
}

func addResultLimitation(result *Result, code, message string) {
	code = strings.TrimSpace(code)
	message = strings.TrimSpace(message)
	if code == "" || message == "" {
		return
	}
	for _, limitation := range result.Limitations {
		if limitation.Code == code && limitation.Message == message {
			return
		}
	}
	result.Limitations = append(result.Limitations, model.Limitation{Code: code, Message: message})
}

func addResultCheck(result *Result, check model.CheckResult) {
	if check.SourceIDs == nil {
		check.SourceIDs = []string{}
	}
	result.Checks = append(result.Checks, check)
}

func addResultSource(result *Result, source model.SourceReference) bool {
	if strings.TrimSpace(source.ID) == "" {
		return false
	}
	for _, existing := range result.Sources {
		if existing.ID == source.ID {
			return true
		}
	}
	if err := source.Validate(); err != nil {
		addResultLimitation(result, "INVALID_SOURCE_REFERENCE", fmt.Sprintf("Ignored an invalid source reference %q: %v", source.ID, err))
		return false
	}
	result.Sources = append(result.Sources, source)
	return true
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	result := sorted[:0]
	for _, value := range sorted {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func finalizeResult(result *Result, fatal bool) {
	sortInstalled(result.Packages)

	sort.SliceStable(result.Findings, func(i, j int) bool {
		left, right := result.Findings[i], result.Findings[j]
		if left.Kind != right.Kind {
			return left.Kind == model.FindingConfirmed
		}
		if left.Severity.Rank() != right.Severity.Rank() {
			return left.Severity.Rank() > right.Severity.Rank()
		}
		leftScore, rightScore := riskScoreValue(left.Risk), riskScoreValue(right.Risk)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if len(left.Evidence) != len(right.Evidence) {
			return len(left.Evidence) > len(right.Evidence)
		}
		return left.ID < right.ID
	})
	sort.SliceStable(result.Checks, func(i, j int) bool {
		return result.Checks[i].ID < result.Checks[j].ID
	})
	for index := range result.Checks {
		result.Checks[index].SourceIDs = dedupeStrings(result.Checks[index].SourceIDs)
		if result.Checks[index].SourceIDs == nil {
			result.Checks[index].SourceIDs = []string{}
		}
	}
	sort.SliceStable(result.Sources, func(i, j int) bool {
		return result.Sources[i].ID < result.Sources[j].ID
	})
	sort.SliceStable(result.Limitations, func(i, j int) bool {
		if result.Limitations[i].Code != result.Limitations[j].Code {
			return result.Limitations[i].Code < result.Limitations[j].Code
		}
		return result.Limitations[i].Message < result.Limitations[j].Message
	})

	confirmed := 0
	heuristic := 0
	for _, finding := range result.Findings {
		if finding.Kind == model.FindingConfirmed {
			confirmed++
		} else {
			heuristic++
		}
	}
	result.Summary = model.Summary{
		ConfirmedFindings: confirmed,
		HeuristicWarnings: heuristic,
		HighestSeverity:   model.HighestSeverity(result.Findings),
	}
	if fatal {
		result.Status = model.StatusFailed
		return
	}
	result.Status = model.DeriveFinalStatus(result.Checks, result.Findings)
}

func riskScoreValue(risk *model.RiskAssessment) float64 {
	if risk == nil {
		return -1
	}
	return risk.Score
}

func notify(progress func(Progress), event Progress) {
	if progress != nil {
		progress(event)
	}
}
