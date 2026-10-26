// Package project 在显式项目根目录内发现并解析受支持的依赖清单与锁文件。
// 本包只读取文件，绝不执行项目代码、构建脚本或安装命令。
package project

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/model"
)

// Format 标识本阶段支持的清单与锁文件组合。
type Format string

const (
	// FormatNPMPackageLock 表示 package.json 与 lockfileVersion 2 或 3 的 package-lock.json。
	FormatNPMPackageLock Format = "npm-package-lock-v2-v3"
	// FormatPythonRequirements 表示 requirements.txt 中的直接依赖；本阶段没有 Python 锁文件。
	FormatPythonRequirements Format = "python-requirements-txt"
)

const (
	rootNodeID = "project"

	checkDetection            = "project:detection"
	checkNPMManifest          = "npm:manifest"
	checkNPMLockfile          = "npm:lockfile"
	checkNPMResolution        = "npm:dependency-resolution"
	checkPythonManifest       = "python:manifest"
	checkPythonResolution     = "python:dependency-resolution"
	projectNPMPSourceID       = "project:npm"
	projectPythonSourceID     = "project:pypi"
	defaultMaxFiles           = 64
	defaultMaxFileSize        = 4 << 20
	defaultMaxNodes           = 5000
	defaultMaxDepth           = 32
	defaultMaxIncludes        = 16
	maxDependencyPathEvidence = 5
	manifestPackageJSON       = "package.json"
	lockfilePackageLockJSON   = "package-lock.json"
	manifestRequirementsTxt   = "requirements.txt"
)

// Limits 限制读取的文件数量、单文件大小、依赖图节点与深度。
// 非正数字段会替换为安全默认值。
type Limits struct {
	MaxFiles    int
	MaxFileSize int64
	MaxNodes    int
	MaxDepth    int
	MaxIncludes int
}

// DefaultLimits 返回项目扫描的保守默认限制。
func DefaultLimits() Limits {
	return Limits{
		MaxFiles:    defaultMaxFiles,
		MaxFileSize: defaultMaxFileSize,
		MaxNodes:    defaultMaxNodes,
		MaxDepth:    defaultMaxDepth,
		MaxIncludes: defaultMaxIncludes,
	}
}

func normalizeLimits(limits Limits) Limits {
	defaults := DefaultLimits()
	if limits.MaxFiles <= 0 {
		limits.MaxFiles = defaults.MaxFiles
	}
	if limits.MaxFileSize <= 0 {
		limits.MaxFileSize = defaults.MaxFileSize
	}
	if limits.MaxNodes <= 0 {
		limits.MaxNodes = defaults.MaxNodes
	}
	if limits.MaxDepth <= 0 {
		limits.MaxDepth = defaults.MaxDepth
	}
	if limits.MaxIncludes <= 0 {
		limits.MaxIncludes = defaults.MaxIncludes
	}
	return limits
}

// Candidate 描述一个可解析的清单及其锁文件；LockfilePath 为空表示锁文件缺失。
type Candidate struct {
	Ecosystem    model.Ecosystem
	Format       Format
	ManifestPath string
	LockfilePath string
}

// Detection 汇总项目根目录中的受支持清单、已知不支持的文件名与覆盖限制。
type Detection struct {
	Candidates  []Candidate
	Unsupported []string
	Limitations []model.Limitation
}

var unsupportedDependencyFiles = map[string]struct{}{
	"yarn.lock":           {},
	"pnpm-lock.yaml":      {},
	"npm-shrinkwrap.json": {},
	"bun.lockb":           {},
	"bun.lock":            {},
	"poetry.lock":         {},
	"Pipfile":             {},
	"Pipfile.lock":        {},
	"pyproject.toml":      {},
	"setup.py":            {},
	"setup.cfg":           {},
	"uv.lock":             {},
	"pdm.lock":            {},
}

// Detect 只检查项目根目录的固定文件名，不递归、不跟随符号链接。
// 根目录本身不可读时返回错误；其余覆盖问题通过 Limitations 表达。
func Detect(root string, limits Limits) (Detection, error) {
	_ = normalizeLimits(limits)
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Detection{}, fmt.Errorf("resolve project root: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return Detection{}, fmt.Errorf("inspect project root: %w", err)
	}
	if !info.IsDir() {
		return Detection{}, fmt.Errorf("project root %q is not a directory", root)
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return Detection{}, fmt.Errorf("read project root: %w", err)
	}

	byName := make(map[string]os.DirEntry, len(entries))
	unsupported := []string{}
	for _, entry := range entries {
		byName[entry.Name()] = entry
		if _, known := unsupportedDependencyFiles[entry.Name()]; known {
			unsupported = append(unsupported, entry.Name())
		}
	}
	sort.Strings(unsupported)

	detection := Detection{Candidates: []Candidate{}, Unsupported: unsupported}
	add := limitationCollector(&detection.Limitations)

	if len(unsupported) > 0 {
		add("UNSUPPORTED_FORMAT", fmt.Sprintf("Found dependency file(s) that this version does not support and did not read: %s.", strings.Join(unsupported, ", ")))
	}

	packageEntry, hasPackage := byName[manifestPackageJSON]
	lockEntry, hasLock := byName[lockfilePackageLockJSON]

	if hasPackage && isSymlink(packageEntry) {
		add("SYMLINKED_FILE_UNSUPPORTED", "package.json is a symbolic link and was not followed.")
		hasPackage = false
	}
	if hasPackage && !isRegular(packageEntry) {
		add("MANIFEST_NOT_REGULAR", "package.json is not a regular file and was not read.")
		hasPackage = false
	}

	if hasPackage {
		candidate := Candidate{
			Ecosystem:    model.EcosystemNPM,
			Format:       FormatNPMPackageLock,
			ManifestPath: manifestPackageJSON,
		}
		if hasLock {
			switch {
			case isSymlink(lockEntry):
				add("SYMLINKED_FILE_UNSUPPORTED", "package-lock.json is a symbolic link and was not followed.")
			case !isRegular(lockEntry):
				add("LOCKFILE_NOT_REGULAR", "package-lock.json is not a regular file and was not read.")
			default:
				candidate.LockfilePath = lockfilePackageLockJSON
			}
		}
		if candidate.LockfilePath == "" {
			add("LOCKFILE_MISSING", "package-lock.json is missing; dependency versions cannot be resolved from the manifest alone and latest versions are never substituted.")
		}
		detection.Candidates = append(detection.Candidates, candidate)
	} else if hasLock {
		if isSymlink(lockEntry) {
			add("SYMLINKED_FILE_UNSUPPORTED", "package-lock.json is a symbolic link and was not followed.")
		} else {
			add("MANIFEST_MISSING", "package-lock.json exists without package.json, so declared direct dependencies cannot be identified.")
		}
	}

	if requirementsEntry, ok := byName[manifestRequirementsTxt]; ok {
		switch {
		case isSymlink(requirementsEntry):
			add("SYMLINKED_FILE_UNSUPPORTED", "requirements.txt is a symbolic link and was not followed.")
		case !isRegular(requirementsEntry):
			add("MANIFEST_NOT_REGULAR", "requirements.txt is not a regular file and was not read.")
		default:
			detection.Candidates = append(detection.Candidates, Candidate{
				Ecosystem:    model.EcosystemPyPI,
				Format:       FormatPythonRequirements,
				ManifestPath: manifestRequirementsTxt,
			})
		}
	}

	sort.SliceStable(detection.Candidates, func(i, j int) bool {
		if detection.Candidates[i].Ecosystem != detection.Candidates[j].Ecosystem {
			return detection.Candidates[i].Ecosystem < detection.Candidates[j].Ecosystem
		}
		return detection.Candidates[i].Format < detection.Candidates[j].Format
	})

	if len(detection.Candidates) == 0 {
		add("NO_SUPPORTED_MANIFEST", "No supported package.json or requirements.txt was found in the project root.")
	}
	return detection, nil
}

func isSymlink(entry os.DirEntry) bool {
	return entry.Type()&os.ModeSymlink != 0
}

func isRegular(entry os.DirEntry) bool {
	return entry.Type().IsRegular()
}

// Stage 表示项目扫描的进度阶段。
type Stage string

const (
	// StageDetect 表示发现清单与锁文件。
	StageDetect Stage = "detect"
	// StageParse 表示解析清单与锁文件。
	StageParse Stage = "parse"
	// StageResolve 表示汇总已解析版本。
	StageResolve Stage = "resolve"
	// StageScan 表示查询漏洞并评估风险。
	StageScan Stage = "scan"
	// StageAssemble 表示组装最终结果。
	StageAssemble Stage = "assemble"
)

// Progress 表示一次进度通知；Completed 与 Total 在适用时表示文件或包的数量。
type Progress struct {
	Stage     Stage
	Message   string
	Completed int
	Total     int
}

// Options 配置项目扫描所需的引擎、限制、时钟与进度回调。
type Options struct {
	Limits   Limits
	Scanner  *engine.Scanner
	Clock    func() time.Time
	Progress func(Progress)
}

// Node 在共享依赖节点模型上附加清单声明的版本约束。
type Node struct {
	model.DependencyNode
	Constraint string
}

// Graph 表示以项目为根节点的依赖图；根节点不对应虚构的包标识。
type Graph struct {
	RootID string
	Nodes  []Node
}

// FormatInfo 描述一个已发现格式及其解析覆盖情况。
type FormatInfo struct {
	Ecosystem          model.Ecosystem
	Format             Format
	ManifestPath       string
	LockfilePath       string
	ResolvedPackages   int
	UnresolvedPackages int
}

// Result 复用共享的发现、检查、来源与限制模型，并附加项目依赖图。
type Result struct {
	Root         string
	ScannedAt    time.Time
	Status       model.ScanStatus
	Formats      []FormatInfo
	Dependencies Graph
	Findings     []model.Finding
	Checks       []model.CheckResult
	Sources      []model.SourceReference
	Limitations  []model.Limitation
	Summary      model.Summary
}

// Scan 在项目根目录内检测受支持格式，解析已安装版本并调用共享引擎查询漏洞。
// 解析与提供者失败会转换为检查、限制或最终状态；只有取消与调用错误才返回 error。
func Scan(ctx context.Context, root string, opts Options) (Result, error) {
	if opts.Scanner == nil {
		return Result{}, errors.New("project scan requires an engine scanner")
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	limits := normalizeLimits(opts.Limits)
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Result{}, fmt.Errorf("resolve project root: %w", err)
	}
	result := Result{
		Root:      absolute,
		ScannedAt: clock().UTC(),
		Status:    model.StatusRunning,
		Formats:   []FormatInfo{},
		Dependencies: Graph{
			RootID: rootNodeID,
			Nodes:  []Node{},
		},
		Findings:    []model.Finding{},
		Checks:      []model.CheckResult{},
		Sources:     []model.SourceReference{},
		Limitations: []model.Limitation{},
	}
	if err := ctx.Err(); err != nil {
		result.Status = model.StatusCancelled
		return result, err
	}

	detection, err := Detect(absolute, limits)
	if err != nil {
		result.Status = model.StatusFailed
		return result, err
	}
	notify(opts.Progress, Progress{
		Stage:   StageDetect,
		Message: fmt.Sprintf("Detected %d supported project format(s).", len(detection.Candidates)),
		Total:   len(detection.Candidates),
	})
	for _, limitation := range detection.Limitations {
		addResultLimitation(&result, limitation.Code, limitation.Message)
	}

	detectionStatus := model.CheckSucceeded
	detectionMessage := fmt.Sprintf("Detected %d supported project format(s) in the project root.", len(detection.Candidates))
	if len(detection.Candidates) == 0 {
		detectionStatus = model.CheckUnavailable
		detectionMessage = "No supported project manifest was found; no dependency coverage is available."
	} else if len(detection.Unsupported) > 0 {
		detectionStatus = model.CheckUnavailable
		detectionMessage = fmt.Sprintf("Detected %d supported project format(s), but unsupported dependency file(s) were not read: %s.", len(detection.Candidates), strings.Join(detection.Unsupported, ", "))
	}
	addResultCheck(&result, model.CheckResult{
		ID:        checkDetection,
		Name:      "Project format detection",
		Status:    detectionStatus,
		Message:   detectionMessage,
		SourceIDs: []string{},
	})

	budget := &fileBudget{limits: limits}
	parsed := make([]parsedProject, 0, len(detection.Candidates))
	fatal := false
	for index, candidate := range detection.Candidates {
		if err := ctx.Err(); err != nil {
			result.Status = model.StatusCancelled
			return result, err
		}
		var project parsedProject
		switch {
		case candidate.Ecosystem == model.EcosystemNPM && candidate.Format == FormatNPMPackageLock:
			project, err = parseNPMProject(ctx, absolute, candidate, budget, clock)
		case candidate.Ecosystem == model.EcosystemPyPI && candidate.Format == FormatPythonRequirements:
			project, err = parsePythonProject(ctx, absolute, candidate, budget, clock)
		default:
			err = fmt.Errorf("unsupported project format %q for ecosystem %q", candidate.Format, candidate.Ecosystem)
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				result.Status = model.StatusCancelled
				return result, ctxErr
			}
			result.Status = model.StatusFailed
			return result, err
		}
		parsed = append(parsed, project)
		fatal = fatal || project.fatal
		resolvedCount, unresolvedCount := countProjectNodes(project.nodes)
		result.Formats = append(result.Formats, FormatInfo{
			Ecosystem:          candidate.Ecosystem,
			Format:             candidate.Format,
			ManifestPath:       candidate.ManifestPath,
			LockfilePath:       candidate.LockfilePath,
			ResolvedPackages:   resolvedCount,
			UnresolvedPackages: unresolvedCount,
		})
		if project.source.ID != "" {
			addResultSource(&result, project.source)
		}
		for _, check := range project.checks {
			addResultCheck(&result, check)
		}
		for _, limitation := range project.limitations {
			addResultLimitation(&result, limitation.Code, limitation.Message)
		}
		result.Dependencies.Nodes = append(result.Dependencies.Nodes, project.nodes...)
		notify(opts.Progress, Progress{
			Stage:     StageParse,
			Message:   fmt.Sprintf("Parsed %s.", candidate.ManifestPath),
			Completed: index + 1,
			Total:     len(detection.Candidates),
		})
	}

	packages := resolvedPackages(parsed)
	notify(opts.Progress, Progress{
		Stage:   StageResolve,
		Message: fmt.Sprintf("Resolved %d exact package version(s) for vulnerability matching.", len(packages)),
		Total:   len(packages),
	})
	if err := ctx.Err(); err != nil {
		result.Status = model.StatusCancelled
		return result, err
	}

	if len(packages) > 0 {
		notify(opts.Progress, Progress{
			Stage:   StageScan,
			Message: fmt.Sprintf("Querying OSV for %d resolved package version(s).", len(packages)),
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
		mergeBatchResults(&result, parsed, batch)
		notify(opts.Progress, Progress{
			Stage:     StageScan,
			Message:   "Vulnerability and risk checks completed.",
			Completed: len(packages),
			Total:     len(packages),
		})
	} else {
		mergeBatchResults(&result, parsed, nil)
	}

	finalizeResult(&result, fatal)
	notify(opts.Progress, Progress{
		Stage:   StageAssemble,
		Message: fmt.Sprintf("Assembled project result with status %s.", result.Status),
	})
	return result, nil
}

func notify(progress func(Progress), event Progress) {
	if progress != nil {
		progress(event)
	}
}

func resolvedPackages(parsed []parsedProject) []engine.ResolvedPackage {
	seen := make(map[string]struct{})
	packages := []engine.ResolvedPackage{}
	for _, project := range parsed {
		for _, node := range project.nodes {
			if node.Version == nil {
				continue
			}
			key := node.Package.String() + "@" + *node.Version
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			packages = append(packages, engine.ResolvedPackage{Package: node.Package, Version: *node.Version})
		}
	}
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Package.String() != packages[j].Package.String() {
			return packages[i].Package.String() < packages[j].Package.String()
		}
		return packages[i].Version < packages[j].Version
	})
	return packages
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

type fileBudget struct {
	limits Limits
	files  int
}

var errFileLimit = errors.New("file count limit exceeded")

// readProjectFile 在项目根目录内读取普通文件，拒绝符号链接、越界路径与超限文件。
func readProjectFile(root, rel string, budget *fileBudget, add func(code, message string)) ([]byte, error) {
	full, cleaned, err := resolveInsideRoot(root, rel)
	if err != nil {
		add("PATH_OUTSIDE_ROOT", fmt.Sprintf("Refused to read %q because it is outside the project root.", rel))
		return nil, err
	}
	if budget.files >= budget.limits.MaxFiles {
		add("FILE_LIMIT_EXCEEDED", fmt.Sprintf("The project scan reads at most %d files; %q was not read.", budget.limits.MaxFiles, cleaned))
		return nil, errFileLimit
	}
	info, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		add("SYMLINKED_FILE_UNSUPPORTED", fmt.Sprintf("%s is a symbolic link and was not followed.", cleaned))
		return nil, fmt.Errorf("%s is a symbolic link", cleaned)
	}
	if !info.Mode().IsRegular() {
		add("FILE_NOT_REGULAR", fmt.Sprintf("%s is not a regular file and was not read.", cleaned))
		return nil, fmt.Errorf("%s is not a regular file", cleaned)
	}
	if info.Size() > budget.limits.MaxFileSize {
		add("FILE_TOO_LARGE", fmt.Sprintf("%s is %d bytes, exceeding the %d byte limit.", cleaned, info.Size(), budget.limits.MaxFileSize))
		return nil, fmt.Errorf("%s exceeds the file size limit", cleaned)
	}

	file, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !openedInfo.Mode().IsRegular() {
		add("FILE_NOT_REGULAR", fmt.Sprintf("%s is not a regular file and was not read.", cleaned))
		return nil, fmt.Errorf("%s is not a regular file", cleaned)
	}
	budget.files++
	data, err := io.ReadAll(io.LimitReader(file, budget.limits.MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > budget.limits.MaxFileSize {
		add("FILE_TOO_LARGE", fmt.Sprintf("%s exceeds the %d byte limit.", cleaned, budget.limits.MaxFileSize))
		return nil, fmt.Errorf("%s exceeds the file size limit", cleaned)
	}
	return data, nil
}

func resolveInsideRoot(root, rel string) (string, string, error) {
	if filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("absolute paths are not allowed: %q", rel)
	}
	cleaned := filepath.Clean(rel)
	if cleaned == "." {
		return "", "", fmt.Errorf("path must reference a file")
	}
	full := filepath.Join(root, cleaned)
	relative, err := filepath.Rel(root, full)
	if err != nil {
		return "", "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("path escapes project root: %q", rel)
	}
	return full, filepath.ToSlash(relative), nil
}
