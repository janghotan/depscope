package engine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"depscope/internal/model"
)

const (
	checkRegistryMetadata   = "registry-metadata"
	checkDependencyMetadata = "dependency-metadata"
	checkDependencyGraph    = "dependency-graph"
	checkLifecycleScripts   = "lifecycle-scripts"
	checkTyposquat          = "typosquat"
	checkVulnerabilities    = "vulnerabilities"
	checkRiskEvaluation     = "risk-evaluation"

	rootNodeID         = "root"
	typosquatSourceID  = "typosquat-reference"
	typosquatDatasetID = "embedded-reference"
)

// ScannerOptions 配置扫描器所需的提供者、策略、数据模式与时钟。
// DataMode 为零值时按 live 处理；demo 模式必须提供非空的 DemoNotice。
type ScannerOptions struct {
	Config          Config
	Registries      map[model.Ecosystem]RegistryMetadataProvider
	Vulnerabilities VulnerabilityProvider
	Risk            RiskEvaluator
	Typosquat       *TyposquatChecker
	Clock           func() time.Time
	DataMode        model.DataMode
	DemoNotice      string
}

// Scanner 协调注册表、漏洞、拼写相似度与风险评估，并组装完整扫描报告。
type Scanner struct {
	config          Config
	registries      map[model.Ecosystem]RegistryMetadataProvider
	vulnerabilities VulnerabilityProvider
	risk            RiskEvaluator
	typosquat       *TyposquatChecker
	clock           func() time.Time
	limiter         *Limiter
	dataMode        model.DataMode
	demoNotice      string
}

// NewScanner 校验依赖并创建扫描器；未指定的风险评估与拼写相似度使用确定性默认实现。
func NewScanner(opts ScannerOptions) (*Scanner, error) {
	if err := opts.Config.Validate(); err != nil {
		return nil, fmt.Errorf("scanner config: %w", err)
	}
	dataMode := opts.DataMode
	if dataMode == "" {
		dataMode = model.DataModeLive
	}
	if !dataMode.IsValid() {
		return nil, fmt.Errorf("scanner data mode %q is not supported", dataMode)
	}
	demoNotice := strings.TrimSpace(opts.DemoNotice)
	switch dataMode {
	case model.DataModeLive:
		if demoNotice != "" {
			return nil, fmt.Errorf("live scanner must not define a demo notice")
		}
	case model.DataModeDemo:
		if demoNotice == "" {
			return nil, fmt.Errorf("demo scanner requires a non-empty demo notice")
		}
	}
	if len(opts.Registries) == 0 {
		return nil, fmt.Errorf("scanner requires at least one registry provider")
	}
	registries := make(map[model.Ecosystem]RegistryMetadataProvider, len(opts.Registries))
	for ecosystem, provider := range opts.Registries {
		if provider == nil {
			return nil, fmt.Errorf("registry provider for ecosystem %q is nil", ecosystem)
		}
		if provider.Ecosystem() != ecosystem {
			return nil, fmt.Errorf("registry provider for ecosystem %q reports ecosystem %q", ecosystem, provider.Ecosystem())
		}
		registries[ecosystem] = provider
	}
	if opts.Vulnerabilities == nil {
		return nil, fmt.Errorf("scanner requires a vulnerability provider")
	}
	if opts.Risk == nil {
		opts.Risk = RulesRiskEvaluator{}
	}
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Typosquat == nil {
		checker, err := DefaultTyposquatChecker()
		if err != nil {
			return nil, err
		}
		opts.Typosquat = checker
	}
	limiter, err := NewLimiter(opts.Config.MaxConcurrency)
	if err != nil {
		return nil, fmt.Errorf("scanner limiter: %w", err)
	}
	return &Scanner{
		config:          opts.Config,
		registries:      registries,
		vulnerabilities: opts.Vulnerabilities,
		risk:            opts.Risk,
		typosquat:       opts.Typosquat,
		clock:           opts.Clock,
		limiter:         limiter,
		dataMode:        dataMode,
		demoNotice:      demoNotice,
	}, nil
}

// Scan 执行一次与扫描器数据模式匹配的扫描。
// 提供者故障会转换为可见的检查结果与限制，只有取消或无效请求才返回错误。
func (s *Scanner) Scan(ctx context.Context, request ScanRequest) (model.ScanReport, error) {
	if err := request.Validate(); err != nil {
		return model.ScanReport{}, fmt.Errorf("invalid scan request: %w", err)
	}
	if request.DataMode != s.dataMode {
		return model.ScanReport{}, fmt.Errorf("scanner data mode %q cannot serve request data mode %q", s.dataMode, request.DataMode)
	}
	registry, exists := s.registries[request.Target.Package.Ecosystem]
	if !exists {
		return model.ScanReport{}, fmt.Errorf("no registry provider for ecosystem %q", request.Target.Package.Ecosystem)
	}

	scanCtx, cancel := s.config.ScanContext(ctx)
	defer cancel()
	report := s.newReport(request)
	state := &scanState{
		scanner:       s,
		ctx:           scanCtx,
		request:       request,
		registry:      registry,
		report:        &report,
		sourceIDs:     make(map[string]struct{}),
		limitationIDs: make(map[string]struct{}),
	}
	if err := state.run(); err != nil {
		if scanErr := scanCtx.Err(); scanErr != nil {
			report.Status = model.StatusCancelled
			return report, scanErr
		}
		return report, err
	}
	if !state.terminal {
		state.finalize()
	}
	return report, nil
}

func (s *Scanner) newReport(request ScanRequest) model.ScanReport {
	var demoNotice *string
	if s.dataMode == model.DataModeDemo {
		notice := s.demoNotice
		demoNotice = &notice
	}
	return model.ScanReport{
		SchemaVersion: model.SchemaVersion,
		DataMode:      s.dataMode,
		DemoNotice:    demoNotice,
		ScannedAt:     s.clock().UTC(),
		Status:        model.StatusRunning,
		Target:        request.Target,
		Summary:       model.Summary{},
		Findings:      []model.Finding{},
		Checks:        []model.CheckResult{},
		Dependencies: model.DependencyGraph{
			RootNodeID: rootNodeID,
			Nodes: []model.DependencyNode{{
				ID:             rootNodeID,
				Package:        request.Target.Package,
				Version:        request.Target.ResolvedVersion,
				Relationship:   model.DependencyRoot,
				DependencyPath: []string{rootNodeID},
				Dependencies:   []string{},
			}},
		},
		Sources:     []model.SourceReference{},
		Limitations: []model.Limitation{},
	}
}

type scanState struct {
	scanner       *Scanner
	ctx           context.Context
	request       ScanRequest
	registry      RegistryMetadataProvider
	report        *model.ScanReport
	sourceIDs     map[string]struct{}
	limitationIDs map[string]struct{}
	riskFailed    bool
	terminal      bool
}

func (st *scanState) run() error {
	pkg := st.request.Target.Package

	metadata, err := st.packageMetadata(pkg)
	if err != nil {
		if ctxErr := st.contextError(); ctxErr != nil {
			return ctxErr
		}
		st.terminal = true
		st.report.Status = model.StatusFailed
		st.addCheck(model.CheckResult{
			ID:        checkRegistryMetadata,
			Name:      "Registry metadata",
			Status:    model.CheckFailed,
			Message:   fmt.Sprintf("Registry metadata could not be read for %s: %v", pkg.String(), err),
			SourceIDs: []string{},
		})
		st.addLimitation("REGISTRY_METADATA_UNAVAILABLE", "The package registry metadata could not be read, so the scan result is not meaningful.")
		return nil
	}
	st.addSource(metadata.Source)
	st.addLimitations(metadata.Limitations)

	resolved, limitationCode, resolutionMessage, ok := resolveVersion(metadata, st.request.Target.ResolvedVersion)
	if !ok {
		st.terminal = true
		st.report.Status = model.StatusFailed
		st.addCheck(model.CheckResult{
			ID:        checkRegistryMetadata,
			Name:      "Registry metadata",
			Status:    model.CheckFailed,
			Message:   resolutionMessage,
			SourceIDs: st.sourceIDList(metadata.Source.ID),
		})
		st.addLimitation(limitationCode, resolutionMessage)
		return nil
	}
	st.report.Target.ResolvedVersion = &resolved
	st.report.Dependencies.Nodes[0].Version = &resolved

	version, versionKnown, versionSourceID := st.versionMetadata(pkg, resolved)
	if err := st.contextError(); err != nil {
		return err
	}

	registrySourceIDs := st.sourceIDList(metadata.Source.ID)
	if versionKnown && version.Source.ID != "" && version.Source.ID != metadata.Source.ID {
		registrySourceIDs = appendUnique(registrySourceIDs, version.Source.ID)
	}
	st.addCheck(model.CheckResult{
		ID:        checkRegistryMetadata,
		Name:      "Registry metadata",
		Status:    model.CheckSucceeded,
		Message:   registryCheckMessage(pkg, resolved, metadata, version, versionKnown),
		SourceIDs: registrySourceIDs,
	})

	if pkg.Ecosystem == model.EcosystemNPM {
		st.inspectLifecycleScripts(pkg, resolved, version, versionKnown, versionSourceID)
		if err := st.contextError(); err != nil {
			return err
		}
	}

	st.inspectTyposquat(pkg)
	if err := st.contextError(); err != nil {
		return err
	}

	st.queryVulnerabilities(pkg, resolved)
	if err := st.contextError(); err != nil {
		return err
	}
	return nil
}

func (st *scanState) packageMetadata(pkg model.PackageIdentity) (RegistryMetadata, error) {
	var metadata RegistryMetadata
	err := st.callProvider(st.ctx, func(ctx context.Context) error {
		var callErr error
		metadata, callErr = st.registry.PackageMetadata(ctx, pkg)
		return callErr
	})
	return metadata, err
}

func (st *scanState) versionMetadata(pkg model.PackageIdentity, version string) (VersionMetadata, bool, string) {
	provider, ok := st.registry.(VersionMetadataProvider)
	if !ok {
		st.inspectBasicDependencies(pkg, version, st.registry)
		if pkg.Ecosystem == model.EcosystemNPM {
			st.addCheck(model.CheckResult{
				ID:        checkLifecycleScripts,
				Name:      "npm lifecycle scripts",
				Status:    model.CheckUnavailable,
				Message:   "The registry provider cannot expose version-level metadata, so npm lifecycle scripts were not inspected.",
				SourceIDs: []string{},
			})
		}
		return VersionMetadata{}, false, ""
	}

	var metadata VersionMetadata
	err := st.callProvider(st.ctx, func(ctx context.Context) error {
		var callErr error
		metadata, callErr = provider.VersionMetadata(ctx, pkg, version)
		return callErr
	})
	if err != nil {
		if ctxErr := st.contextError(); ctxErr != nil {
			return VersionMetadata{}, false, ""
		}
		message := fmt.Sprintf("Version metadata for %s@%s could not be read: %v", pkg.String(), version, err)
		st.addCheck(model.CheckResult{
			ID:        checkDependencyMetadata,
			Name:      "Dependency metadata",
			Status:    model.CheckUnavailable,
			Message:   message,
			SourceIDs: []string{},
		})
		st.addCheck(model.CheckResult{
			ID:        checkDependencyGraph,
			Name:      "Dependency graph",
			Status:    model.CheckUnavailable,
			Message:   "The dependency graph is incomplete because version metadata was unavailable.",
			SourceIDs: []string{},
		})
		st.addLimitation("VERSION_METADATA_UNAVAILABLE", message)
		return VersionMetadata{}, false, ""
	}
	if metadata.Version != "" && metadata.Version != version {
		message := fmt.Sprintf("The registry returned version %q while %q was requested.", metadata.Version, version)
		st.addCheck(model.CheckResult{
			ID:        checkDependencyMetadata,
			Name:      "Dependency metadata",
			Status:    model.CheckUnavailable,
			Message:   message,
			SourceIDs: []string{},
		})
		st.addCheck(model.CheckResult{
			ID:        checkDependencyGraph,
			Name:      "Dependency graph",
			Status:    model.CheckUnavailable,
			Message:   "The dependency graph is incomplete because the version metadata did not match the resolved version.",
			SourceIDs: []string{},
		})
		st.addLimitation("VERSION_METADATA_MISMATCH", message)
		return VersionMetadata{}, false, ""
	}

	st.addSource(metadata.Source)
	st.addLimitations(metadata.Limitations)
	sourceID := metadata.Source.ID
	if sourceID == "" && len(st.report.Sources) > 0 {
		sourceID = st.report.Sources[0].ID
	}
	st.inspectVersionDependencies(pkg, version, metadata, sourceID)
	return metadata, true, sourceID
}

func (st *scanState) inspectBasicDependencies(pkg model.PackageIdentity, version string, provider RegistryMetadataProvider) {
	var dependencies []DependencyRequirement
	err := st.callProvider(st.ctx, func(ctx context.Context) error {
		var callErr error
		dependencies, callErr = provider.Dependencies(ctx, pkg, version)
		return callErr
	})
	if err != nil {
		if st.contextError() != nil {
			return
		}
		message := fmt.Sprintf("Declared dependencies for %s@%s could not be read: %v", pkg.String(), version, err)
		st.addCheck(model.CheckResult{
			ID:        checkDependencyMetadata,
			Name:      "Dependency metadata",
			Status:    model.CheckUnavailable,
			Message:   message,
			SourceIDs: []string{},
		})
		st.addCheck(model.CheckResult{
			ID:        checkDependencyGraph,
			Name:      "Dependency graph",
			Status:    model.CheckUnavailable,
			Message:   "The dependency graph is incomplete because declared dependencies were unavailable.",
			SourceIDs: []string{},
		})
		st.addLimitation("DEPENDENCY_METADATA_UNAVAILABLE", message)
		return
	}
	accepted, invalid := st.buildDirectDependencyNodes(dependencies)
	st.addDependencyChecks(pkg, version, accepted, invalid, invalid > 0, "")
}

func (st *scanState) inspectVersionDependencies(pkg model.PackageIdentity, version string, metadata VersionMetadata, sourceID string) {
	accepted, invalid := st.buildDirectDependencyNodes(metadata.Dependencies)
	incomplete := hasDependencyDataLimitation(metadata.Limitations)
	st.addDependencyChecks(pkg, version, accepted, invalid, incomplete, sourceID)
}

func (st *scanState) addDependencyChecks(pkg model.PackageIdentity, version string, accepted, invalid int, incomplete bool, sourceID string) {
	sourceIDs := st.sourceIDList(sourceID)
	status := model.CheckSucceeded
	message := fmt.Sprintf("Read %d declared direct dependencies for %s@%s. Dependencies are inspected but not resolved in this phase.", accepted, pkg.String(), version)
	if invalid > 0 || incomplete {
		status = model.CheckUnavailable
		message = fmt.Sprintf("Some declared dependencies for %s@%s could not be interpreted, so the dependency view is incomplete.", pkg.String(), version)
	} else if accepted == 0 {
		message = fmt.Sprintf("No direct dependencies are declared for %s@%s.", pkg.String(), version)
	}
	st.addCheck(model.CheckResult{
		ID:        checkDependencyMetadata,
		Name:      "Dependency metadata",
		Status:    status,
		Message:   message,
		SourceIDs: sourceIDs,
	})

	graphStatus := model.CheckSucceeded
	graphMessage := "The root package is the complete scanned graph; no declared direct dependencies were found."
	if invalid > 0 || incomplete {
		graphStatus = model.CheckUnavailable
		graphMessage = "The dependency graph is incomplete because some declared dependency entries could not be interpreted."
	} else if accepted > 0 {
		graphStatus = model.CheckSkipped
		graphMessage = fmt.Sprintf("%d declared direct dependencies were found, but recursive resolution is not implemented in this phase; the graph is intentionally incomplete.", accepted)
		st.addLimitation("DEPENDENCY_RESOLUTION_UNAVAILABLE", "Declared direct dependencies were not resolved in this phase; transitive vulnerabilities are not covered by this report.")
	}
	st.addCheck(model.CheckResult{
		ID:        checkDependencyGraph,
		Name:      "Dependency graph",
		Status:    graphStatus,
		Message:   graphMessage,
		SourceIDs: sourceIDs,
	})
}

func (st *scanState) buildDirectDependencyNodes(dependencies []DependencyRequirement) (int, int) {
	if len(dependencies) == 0 {
		return 0, 0
	}
	sorted := append([]DependencyRequirement(nil), dependencies...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Package.String() != sorted[j].Package.String() {
			return sorted[i].Package.String() < sorted[j].Package.String()
		}
		return sorted[i].Constraint < sorted[j].Constraint
	})

	seen := make(map[string]struct{}, len(sorted))
	nodeIDs := make([]string, 0, len(sorted))
	accepted := 0
	invalid := 0
	for _, dependency := range sorted {
		if err := dependency.Package.Validate(); err != nil {
			st.addLimitation("INVALID_DEPENDENCY_NAME", fmt.Sprintf("Ignored an invalid declared dependency %q: %v", dependency.Package.Name, err))
			invalid++
			continue
		}
		key := dependency.Package.String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		nodeID := "direct:" + key
		st.report.Dependencies.Nodes = append(st.report.Dependencies.Nodes, model.DependencyNode{
			ID:             nodeID,
			Package:        dependency.Package,
			Version:        nil,
			Relationship:   model.DependencyDirect,
			DependencyPath: []string{rootNodeID, nodeID},
			Dependencies:   []string{},
		})
		nodeIDs = append(nodeIDs, nodeID)
		accepted++
	}
	sort.Strings(nodeIDs)
	st.report.Dependencies.Nodes[0].Dependencies = append(st.report.Dependencies.Nodes[0].Dependencies, nodeIDs...)
	return accepted, invalid
}

func (st *scanState) inspectLifecycleScripts(pkg model.PackageIdentity, version string, metadata VersionMetadata, versionKnown bool, sourceID string) {
	if !versionKnown {
		st.addCheck(model.CheckResult{
			ID:        checkLifecycleScripts,
			Name:      "npm lifecycle scripts",
			Status:    model.CheckUnavailable,
			Message:   "npm lifecycle scripts were not inspected because version metadata was unavailable.",
			SourceIDs: []string{},
		})
		return
	}

	for _, script := range metadata.LifecycleScripts {
		command := strings.TrimSpace(script.Command)
		if command == "" {
			continue
		}
		finding := model.Finding{
			ID:          "LIFECYCLE-" + strings.ToUpper(script.Name),
			Kind:        model.FindingHeuristic,
			Title:       fmt.Sprintf("npm %s lifecycle script declared", script.Name),
			Description: fmt.Sprintf("%s@%s declares an npm %s lifecycle script. The command is shown for inspection and was not executed. A lifecycle script is a signal that warrants review; it is not proof of malicious behavior.", pkg.String(), version, script.Name),
			Severity:    model.SeverityLow,
			Package:     pkg,
			Version:     st.report.Target.ResolvedVersion,
			Evidence: []model.Evidence{{
				Kind:     model.EvidenceHeuristic,
				Summary:  fmt.Sprintf("Declared %s script command: %s", script.Name, truncate(command, 500)),
				SourceID: sourceID,
			}},
		}
		if err := st.evaluateRisk(&finding); err != nil {
			return
		}
		st.addFinding(finding)
	}

	message := fmt.Sprintf("Inspected %d npm lifecycle script(s) for %s@%s. Commands are never executed; lifecycle scripts are signals for review, not proof of malicious behavior.", len(metadata.LifecycleScripts), pkg.String(), version)
	if len(metadata.LifecycleScripts) == 0 {
		message = fmt.Sprintf("No preinstall, install, or postinstall lifecycle scripts are declared for %s@%s.", pkg.String(), version)
	}
	st.addCheck(model.CheckResult{
		ID:        checkLifecycleScripts,
		Name:      "npm lifecycle scripts",
		Status:    model.CheckSucceeded,
		Message:   message,
		SourceIDs: st.sourceIDList(sourceID),
	})
}

func (st *scanState) inspectTyposquat(pkg model.PackageIdentity) {
	result, err := st.scanner.typosquat.Check(st.ctx, pkg)
	if err != nil {
		if ctxErr := st.contextError(); ctxErr != nil {
			return
		}
		status := model.CheckFailed
		message := fmt.Sprintf("Name similarity check failed: %v", err)
		if errors.Is(err, ErrReferenceUnavailable) {
			status = model.CheckUnavailable
			message = "No versioned typosquat reference data is available for this ecosystem, so name similarity could not be checked."
			st.addLimitation("TYPOSQUAT_REFERENCE_UNAVAILABLE", message)
		}
		st.addCheck(model.CheckResult{
			ID:        checkTyposquat,
			Name:      "Typosquat reference comparison",
			Status:    status,
			Message:   message,
			SourceIDs: []string{},
		})
		return
	}

	sourceID := st.addTyposquatSource(pkg, result)
	if result.Matched {
		description := fmt.Sprintf(
			"The package name %q is within edit distance %d of the reference name %q after canonical normalization. The reference list is a curated, versioned seed and is not a live popularity ranking. Name similarity is a heuristic signal for review, not proof of malicious behavior.",
			pkg.Name, result.Distance, result.Reference,
		)
		finding := model.Finding{
			ID:          "TYPOSQUAT-" + pkg.String(),
			Kind:        model.FindingHeuristic,
			Title:       fmt.Sprintf("Possible typosquat of %q", result.Reference),
			Description: description,
			Severity:    model.SeverityMedium,
			Package:     pkg,
			Version:     st.report.Target.ResolvedVersion,
			Evidence: []model.Evidence{{
				Kind:     model.EvidenceHeuristic,
				Summary:  fmt.Sprintf("Closest reference %q from dataset %s (updated %s) at Damerau-Levenshtein distance %d.", result.Reference, result.DatasetVersion, result.DatasetUpdatedAt.Format(time.RFC3339), result.Distance),
				SourceID: sourceID,
			}},
		}
		if err := st.evaluateRisk(&finding); err != nil {
			return
		}
		st.addFinding(finding)
	}

	message := fmt.Sprintf("No close match found against embedded reference dataset %s; the dataset is a versioned seed, not a live top-N list.", result.DatasetVersion)
	if result.Matched {
		message = fmt.Sprintf("Name is within edit distance %d of reference %q in dataset %s; this is a heuristic signal, not proof of malicious intent.", result.Distance, result.Reference, result.DatasetVersion)
	}
	st.addCheck(model.CheckResult{
		ID:        checkTyposquat,
		Name:      "Typosquat reference comparison",
		Status:    model.CheckSucceeded,
		Message:   message,
		SourceIDs: st.sourceIDList(sourceID),
	})
}

func (st *scanState) addTyposquatSource(pkg model.PackageIdentity, result TyposquatResult) string {
	provider := model.SourceProviderNPM
	if st.scanner.dataMode == model.DataModeDemo {
		provider = model.SourceProviderDemo
	} else if pkg.Ecosystem == model.EcosystemPyPI {
		provider = model.SourceProviderPyPI
	}
	source := model.SourceReference{
		ID:          typosquatSourceID,
		Provider:    provider,
		Identifier:  fmt.Sprintf("%s:%s:%s", typosquatDatasetID, pkg.Ecosystem, result.DatasetVersion),
		RetrievedAt: st.scanner.clock().UTC(),
	}
	st.addSource(source)
	return source.ID
}

func (st *scanState) queryVulnerabilities(pkg model.PackageIdentity, version string) {
	var result VulnerabilityResult
	err := st.callProvider(st.ctx, func(ctx context.Context) error {
		var callErr error
		result, callErr = st.scanner.vulnerabilities.Query(ctx, pkg, version)
		return callErr
	})
	if err != nil {
		if ctxErr := st.contextError(); ctxErr != nil {
			return
		}
		message := fmt.Sprintf("%s for %s@%s could not be read: %v", st.scanner.vulnerabilitySourceName(), pkg.String(), version, err)
		st.addCheck(model.CheckResult{
			ID:        checkVulnerabilities,
			Name:      st.scanner.vulnerabilityCheckName(),
			Status:    model.CheckUnavailable,
			Message:   message,
			SourceIDs: []string{},
		})
		st.addLimitation("VULNERABILITY_DATA_UNAVAILABLE", message)
		return
	}

	st.addSource(result.Source)
	st.addLimitations(result.Limitations)
	sourceIDs := st.sourceIDList(result.Source.ID)

	status := model.CheckSucceeded
	message := fmt.Sprintf("%s returned no known advisories affecting %s@%s.", st.scanner.vulnerabilitySourceName(), pkg.String(), version)
	if len(result.Advisories) > 0 {
		message = fmt.Sprintf("%s returned %d advisory(ies) affecting %s@%s.", st.scanner.vulnerabilitySourceName(), len(result.Advisories), pkg.String(), version)
	}
	if len(result.Limitations) > 0 {
		status = model.CheckUnavailable
		message = fmt.Sprintf("%s returned %d advisory(ies) for %s@%s, but part of the response could not be interpreted; coverage is incomplete.", st.scanner.vulnerabilitySourceName(), len(result.Advisories), pkg.String(), version)
	}

	seen := make(map[string]struct{}, len(result.Advisories))
	for _, advisory := range result.Advisories {
		if _, exists := seen[advisory.ID]; exists {
			continue
		}
		seen[advisory.ID] = struct{}{}
		finding := vulnerabilityFinding(pkg, version, advisory, result.Source.ID, st.scanner.advisoryFallbackReference(advisory.ID))
		if err := st.evaluateRisk(&finding); err != nil {
			return
		}
		st.addFinding(finding)
	}

	st.addCheck(model.CheckResult{
		ID:        checkVulnerabilities,
		Name:      st.scanner.vulnerabilityCheckName(),
		Status:    status,
		Message:   message,
		SourceIDs: sourceIDs,
	})
}

func (s *Scanner) vulnerabilitySourceName() string {
	if s.dataMode == model.DataModeDemo {
		return "Demo advisory data"
	}
	return "OSV"
}

func (s *Scanner) vulnerabilityCheckName() string {
	if s.dataMode == model.DataModeDemo {
		return "Demo advisory data"
	}
	return "OSV vulnerability query"
}

// advisoryFallbackReference 为缺少引用链接的公告生成稳定引用。
// 演示数据只使用公告标识，避免把合成数据伪装成真实外部链接。
func (s *Scanner) advisoryFallbackReference(advisoryID string) string {
	if s.dataMode == model.DataModeDemo {
		return advisoryID
	}
	return "https://osv.dev/vulnerability/" + url.PathEscape(advisoryID)
}

func vulnerabilityFinding(pkg model.PackageIdentity, version string, advisory Advisory, sourceID, fallbackReference string) model.Finding {
	severity := advisory.Severity
	if !severity.IsValid() {
		severity = model.SeverityUnknown
	}
	summary := strings.TrimSpace(advisory.Summary)
	if summary == "" {
		summary = "OSV advisory " + advisory.ID
	}
	description := strings.TrimSpace(advisory.Details)
	if description == "" {
		description = summary + " See the referenced advisory for details."
	}

	reference := ""
	if len(advisory.References) > 0 {
		reference = advisory.References[0]
	} else {
		reference = fallbackReference
	}
	evidence := []model.Evidence{{
		Kind:      model.EvidenceAdvisory,
		Summary:   summary,
		SourceID:  sourceID,
		Reference: reference,
	}}
	if len(advisory.AffectedRanges) > 0 {
		evidence = append(evidence, model.Evidence{
			Kind:      model.EvidenceAdvisory,
			Summary:   "Affected ranges: " + truncate(strings.Join(advisory.AffectedRanges, "; "), 1000),
			SourceID:  sourceID,
			Reference: reference,
		})
	}

	var remediation *string
	if len(advisory.FixedVersions) > 0 {
		value := fmt.Sprintf("Upgrade %s to %s or later; verify compatibility before upgrading.", pkg.String(), strings.Join(advisory.FixedVersions, ", "))
		remediation = &value
	} else {
		value := "No fixed version was listed in the advisory; review the referenced advisory for mitigation guidance."
		remediation = &value
	}

	versionCopy := version
	return model.Finding{
		ID:          "VULN-" + advisory.ID,
		Kind:        model.FindingConfirmed,
		Title:       summary,
		Description: description,
		Severity:    severity,
		Package:     pkg,
		Version:     &versionCopy,
		Evidence:    evidence,
		Remediation: remediation,
	}
}

func (st *scanState) evaluateRisk(finding *model.Finding) error {
	assessment, err := st.scanner.risk.Evaluate(st.ctx, RiskInput{Finding: *finding})
	if err != nil {
		if ctxErr := st.contextError(); ctxErr != nil {
			return ctxErr
		}
		st.recordRiskFailure(err)
		return nil
	}
	if math.IsNaN(assessment.Score) || math.IsInf(assessment.Score, 0) || assessment.Score < 0 || assessment.Score > 100 || strings.TrimSpace(assessment.Rationale) == "" {
		st.recordRiskFailure(fmt.Errorf("risk evaluator returned an invalid assessment"))
		return nil
	}
	finding.Risk = &assessment
	return nil
}

func (st *scanState) recordRiskFailure(err error) {
	if st.riskFailed {
		return
	}
	st.riskFailed = true
	message := fmt.Sprintf("Risk prioritization could not be computed: %v. The finding is retained without a risk score.", err)
	st.addCheck(model.CheckResult{
		ID:        checkRiskEvaluation,
		Name:      "Risk evaluation",
		Status:    model.CheckFailed,
		Message:   message,
		SourceIDs: []string{},
	})
	st.addLimitation("RISK_EVALUATION_UNAVAILABLE", message)
}

func (st *scanState) finalize() {
	sort.SliceStable(st.report.Findings, func(i, j int) bool {
		left, right := st.report.Findings[i], st.report.Findings[j]
		if left.Kind != right.Kind {
			return left.Kind == model.FindingConfirmed
		}
		if left.Severity.Rank() != right.Severity.Rank() {
			return left.Severity.Rank() > right.Severity.Rank()
		}
		return left.ID < right.ID
	})
	sort.SliceStable(st.report.Checks, func(i, j int) bool {
		return st.report.Checks[i].ID < st.report.Checks[j].ID
	})
	for index := range st.report.Checks {
		sort.Strings(st.report.Checks[index].SourceIDs)
	}
	sort.SliceStable(st.report.Sources, func(i, j int) bool {
		return st.report.Sources[i].ID < st.report.Sources[j].ID
	})
	sort.SliceStable(st.report.Limitations, func(i, j int) bool {
		if st.report.Limitations[i].Code != st.report.Limitations[j].Code {
			return st.report.Limitations[i].Code < st.report.Limitations[j].Code
		}
		return st.report.Limitations[i].Message < st.report.Limitations[j].Message
	})

	confirmed := 0
	heuristic := 0
	for _, finding := range st.report.Findings {
		if finding.Kind == model.FindingConfirmed {
			confirmed++
		} else {
			heuristic++
		}
	}
	st.report.Summary = model.Summary{
		ConfirmedFindings: confirmed,
		HeuristicWarnings: heuristic,
		HighestSeverity:   model.HighestSeverity(st.report.Findings),
	}
	st.report.Status = model.DeriveFinalStatus(st.report.Checks, st.report.Findings)
}

func (st *scanState) callProvider(parent context.Context, invoke func(context.Context) error) error {
	return st.scanner.callProvider(parent, invoke)
}

// callProvider 在并发上限与单次提供者超时约束下执行一次外部调用。
func (s *Scanner) callProvider(parent context.Context, invoke func(context.Context) error) error {
	if err := parent.Err(); err != nil {
		return err
	}
	if err := s.limiter.Acquire(parent); err != nil {
		return err
	}
	defer s.limiter.Release()
	ctx, cancel := s.config.ProviderContext(parent)
	defer cancel()
	return invoke(ctx)
}

func (st *scanState) contextError() error {
	return st.ctx.Err()
}

func (st *scanState) addCheck(check model.CheckResult) {
	if check.SourceIDs == nil {
		check.SourceIDs = []string{}
	}
	st.report.Checks = append(st.report.Checks, check)
}

func (st *scanState) addFinding(finding model.Finding) {
	if finding.Evidence == nil {
		finding.Evidence = []model.Evidence{}
	}
	st.report.Findings = append(st.report.Findings, finding)
}

func (st *scanState) addSource(source model.SourceReference) bool {
	if strings.TrimSpace(source.ID) == "" {
		return false
	}
	if _, exists := st.sourceIDs[source.ID]; exists {
		return true
	}
	if err := source.Validate(); err != nil {
		st.addLimitation("INVALID_SOURCE_REFERENCE", fmt.Sprintf("Ignored an invalid source reference %q: %v", source.ID, err))
		return false
	}
	st.sourceIDs[source.ID] = struct{}{}
	st.report.Sources = append(st.report.Sources, source)
	return true
}

func (st *scanState) addLimitations(limitations []model.Limitation) {
	for _, limitation := range limitations {
		st.addLimitation(limitation.Code, limitation.Message)
	}
}

func (st *scanState) addLimitation(code, message string) {
	code = strings.TrimSpace(code)
	message = strings.TrimSpace(message)
	if code == "" || message == "" {
		return
	}
	key := code + "\x00" + message
	if _, exists := st.limitationIDs[key]; exists {
		return
	}
	st.limitationIDs[key] = struct{}{}
	st.report.Limitations = append(st.report.Limitations, model.Limitation{Code: code, Message: message})
}

func (st *scanState) sourceIDList(ids ...string) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := st.sourceIDs[id]; !exists {
			continue
		}
		result = appendUnique(result, id)
	}
	return result
}

func resolveVersion(metadata RegistryMetadata, requested *string) (string, string, string, bool) {
	if requested != nil {
		value := strings.TrimSpace(*requested)
		if value == "" {
			return "", "REQUESTED_VERSION_INVALID", "The requested version must not be empty.", false
		}
		if len(metadata.Versions) == 0 {
			return "", "VERSION_LIST_UNAVAILABLE", fmt.Sprintf("The registry returned no version list, so the requested version %q could not be verified.", value), false
		}
		for _, candidate := range metadata.Versions {
			if candidate == value {
				return value, "", "", true
			}
		}
		return "", "REQUESTED_VERSION_NOT_FOUND", fmt.Sprintf("The requested version %q is not present in the registry version list.", value), false
	}

	if metadata.LatestVersion == nil || strings.TrimSpace(*metadata.LatestVersion) == "" {
		return "", "LATEST_VERSION_UNAVAILABLE", "The registry did not provide a latest version, so no version could be resolved.", false
	}
	latest := strings.TrimSpace(*metadata.LatestVersion)
	if len(metadata.Versions) > 0 && !containsVersion(metadata.Versions, latest) {
		return "", "LATEST_VERSION_INCONSISTENT", fmt.Sprintf("The registry latest version %q is not present in its version list.", latest), false
	}
	return latest, "", "", true
}

func registryCheckMessage(pkg model.PackageIdentity, version string, metadata RegistryMetadata, detail VersionMetadata, detailKnown bool) string {
	parts := []string{fmt.Sprintf("Resolved %s@%s from registry metadata.", pkg.String(), version)}

	var publishedAt *time.Time
	if detailKnown && detail.PublishedAt != nil {
		publishedAt = detail.PublishedAt
	}
	if publishedAt != nil {
		parts = append(parts, "Selected version publication time: "+publishedAt.UTC().Format(time.RFC3339)+".")
	} else if metadata.LatestPublishedAt != nil {
		parts = append(parts, "Latest release publication time: "+metadata.LatestPublishedAt.UTC().Format(time.RFC3339)+".")
	} else {
		parts = append(parts, "Publication time is unavailable from the registry.")
	}

	var repository *string
	if detailKnown && detail.RepositoryURL != nil {
		repository = detail.RepositoryURL
	} else if metadata.RepositoryURL != nil {
		repository = metadata.RepositoryURL
	}
	if repository != nil {
		parts = append(parts, "Repository: "+*repository+".")
	} else {
		parts = append(parts, "Repository metadata is unavailable.")
	}
	if detailKnown && detail.Deprecated != nil {
		parts = append(parts, "The selected version is deprecated: "+truncate(*detail.Deprecated, 500))
	}
	parts = append(parts, "Maintenance metadata is context only; it is not evidence of malicious behavior.")
	return strings.Join(parts, " ")
}

func hasDependencyDataLimitation(limitations []model.Limitation) bool {
	for _, limitation := range limitations {
		switch limitation.Code {
		case "INVALID_DEPENDENCY_NAME", "INVALID_DEPENDENCY_CONSTRAINT":
			return true
		}
	}
	return false
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func containsVersion(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func truncate(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
