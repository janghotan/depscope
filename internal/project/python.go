package project

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/janghotan/depscope/internal/model"
)

type pythonRequirement struct {
	name         string
	constraint   string
	exactVersion string
	exact        bool
	other        bool
	unsupported  bool
}

type pythonSpecifier struct {
	operator string
	version  string
}

type pythonParser struct {
	root             string
	limits           Limits
	budget           *fileBudget
	addLimit         func(code, message string)
	active           map[string]struct{}
	visited          map[string]struct{}
	requirements     []pythonRequirement
	includes         int
	includeLimitHit  bool
	unsupportedCount int
	invalidCount     int
	incomplete       bool
	fatal            bool
}

type pythonAggregate struct {
	name        string
	constraints []string
	exact       map[string]struct{}
	other       bool
	unsupported bool
}

func parsePythonProject(ctx context.Context, root string, candidate Candidate, budget *fileBudget, clock func() time.Time) (parsedProject, error) {
	result := parsedProject{
		candidate:   candidate,
		nodes:       []Node{},
		checks:      []model.CheckResult{},
		limitations: []model.Limitation{},
	}
	addLimit := limitationCollector(&result.limitations)
	result.source = model.SourceReference{
		ID:          projectPythonSourceID,
		Provider:    model.SourceProviderPyPI,
		Identifier:  candidate.ManifestPath,
		RetrievedAt: clock().UTC(),
	}

	parser := &pythonParser{
		root:     root,
		limits:   budget.limits,
		budget:   budget,
		addLimit: addLimit,
		active:   map[string]struct{}{candidate.ManifestPath: {}},
		visited:  map[string]struct{}{},
	}
	if err := parser.parseFile(ctx, candidate.ManifestPath, true); err != nil {
		return result, err
	}
	if parser.fatal {
		result.fatal = true
		result.checks = append(result.checks, model.CheckResult{
			ID:        checkPythonManifest,
			Name:      "Python requirements",
			Status:    model.CheckFailed,
			Message:   "requirements.txt could not be read completely; no Python dependencies were scanned.",
			SourceIDs: []string{projectPythonSourceID},
		})
		result.checks = append(result.checks, unavailableResolutionCheck(
			model.EcosystemPyPI,
			"Dependency resolution could not run because requirements.txt was unreadable.",
			projectPythonSourceID,
		))
		return result, nil
	}

	nodes, resolved, unresolved, conflicts := buildPythonNodes(parser)
	// requirements.txt 没有生产/开发分节，也没有锁文件标记；
	// 因此依赖分类只能是 unknown，证据仅指向被读取的清单文件。
	for index := range nodes {
		nodes[index].Scopes = []DependencyScope{DependencyScopeUnknown}
		nodes[index].ScopeEvidence = []DependencyEvidence{{Path: candidate.ManifestPath}}
	}
	result.nodes = nodes
	manifestStatus := model.CheckSucceeded
	manifestMessage := fmt.Sprintf("Read requirements.txt with %d direct requirement(s): %d exact version(s) resolved and %d unresolved.", len(nodes), resolved, unresolved)
	if unresolved > 0 || conflicts > 0 || parser.unsupportedCount > 0 || parser.invalidCount > 0 || parser.incomplete {
		manifestStatus = model.CheckUnavailable
		manifestMessage = fmt.Sprintf("Read requirements.txt with %d direct requirement(s): %d exact version(s) resolved, %d unresolved, %d unsupported or invalid entry(ies).", len(nodes), resolved, unresolved, parser.unsupportedCount+parser.invalidCount)
	}
	result.checks = append(result.checks, model.CheckResult{
		ID:        checkPythonManifest,
		Name:      "Python requirements",
		Status:    manifestStatus,
		Message:   manifestMessage,
		SourceIDs: []string{projectPythonSourceID},
	})

	addLimit("PYTHON_TRANSITIVE_UNAVAILABLE", "requirements.txt lists direct requirements only; transitive Python dependencies are not resolved in this version, so project coverage is incomplete.")
	result.checks = append(result.checks, model.CheckResult{
		ID:        checkPythonResolution,
		Name:      "Python dependency resolution",
		Status:    model.CheckUnavailable,
		Message:   "Transitive Python dependency resolution is unavailable in this version; only exact direct requirements are matched against OSV.",
		SourceIDs: []string{projectPythonSourceID},
	})
	return result, nil
}

func (p *pythonParser) parseFile(ctx context.Context, rel string, fatal bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, exists := p.visited[rel]; exists {
		return nil
	}
	p.visited[rel] = struct{}{}

	data, err := readProjectFile(p.root, rel, p.budget, p.addLimit)
	if err != nil {
		if fatal {
			p.fatal = true
			p.addLimit("MANIFEST_UNREADABLE", fmt.Sprintf("%s could not be read: %v", rel, err))
		} else {
			p.addLimit("INCLUDE_UNREADABLE", fmt.Sprintf("Included requirements file %s could not be read: %v", rel, err))
		}
		p.unsupportedCount++
		return nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), int(p.limits.MaxFileSize)+1)
	var continued strings.Builder
	lineNumber := 0
	process := func(line string) error {
		lineNumber++
		if lineNumber%128 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		return p.processLine(ctx, rel, line)
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimRight(line, " \t")
		if strings.HasSuffix(trimmed, "\\") {
			continued.WriteString(strings.TrimSuffix(trimmed, "\\"))
			continued.WriteString(" ")
			continue
		}
		continued.WriteString(line)
		if err := process(continued.String()); err != nil {
			return err
		}
		continued.Reset()
	}
	if err := scanner.Err(); err != nil {
		p.addLimit("INVALID_REQUIREMENT", fmt.Sprintf("%s could not be parsed completely: %v", rel, err))
		p.unsupportedCount++
		return nil
	}
	if continued.Len() > 0 {
		if err := process(continued.String()); err != nil {
			return err
		}
	}
	return nil
}

func (p *pythonParser) processLine(ctx context.Context, rel string, raw string) error {
	line := strings.TrimSpace(stripRequirementComment(raw))
	if line == "" {
		return nil
	}
	lower := strings.ToLower(line)
	if lower == "-e" || strings.HasPrefix(lower, "-e ") || strings.HasPrefix(lower, "-e=") || strings.HasPrefix(lower, "--editable ") || strings.HasPrefix(lower, "--editable=") {
		p.addLimit("EDITABLE_UNSUPPORTED", fmt.Sprintf("Editable requirement %q in %s was not read or installed.", truncateText(line, 200), rel))
		p.unsupportedCount++
		return nil
	}
	if includePath, isInclude, valid := parseIncludeDirective(line); isInclude {
		if !valid {
			p.addLimit("INVALID_REQUIREMENT", fmt.Sprintf("Include directive %q in %s has no valid path.", truncateText(line, 200), rel))
			p.invalidCount++
			return nil
		}
		return p.include(ctx, rel, includePath)
	}
	if lower == "-c" || strings.HasPrefix(lower, "-c ") || strings.HasPrefix(lower, "-c=") || strings.HasPrefix(lower, "--constraint ") || strings.HasPrefix(lower, "--constraint=") {
		p.addLimit("CONSTRAINT_FILE_UNSUPPORTED", fmt.Sprintf("Constraint file directive %q in %s is not supported.", truncateText(line, 200), rel))
		p.unsupportedCount++
		return nil
	}
	if strings.HasPrefix(line, "-") {
		option := strings.Fields(line)[0]
		p.addLimit("UNSUPPORTED_REQUIREMENTS_OPTION", fmt.Sprintf("Requirements option %q in %s is not supported and was not acted on.", option, rel))
		p.unsupportedCount++
		return nil
	}
	if isUnsupportedRequirementSource(line) {
		p.addLimit("UNSUPPORTED_REQUIREMENT_SOURCE", fmt.Sprintf("Requirement source %q in %s was not fetched or installed.", truncateText(line, 200), rel))
		p.unsupportedCount++
		return nil
	}

	if markerIndex := strings.Index(line, ";"); markerIndex >= 0 {
		base := strings.TrimSpace(line[:markerIndex])
		p.addLimit("ENVIRONMENT_MARKER_UNSUPPORTED", fmt.Sprintf("Environment marker requirement %q in %s cannot be evaluated and was left unresolved.", truncateText(base, 200), rel))
		p.unsupportedCount++
		cleaned, option := stripInlineOptions(base)
		if option != "" {
			p.addLimit("UNSUPPORTED_REQUIREMENTS_OPTION", fmt.Sprintf("Requirements option %q in %s is not supported.", option, rel))
		}
		requirement, err := parsePythonRequirement(cleaned)
		if err != nil {
			p.addLimit("INVALID_REQUIREMENT", fmt.Sprintf("Requirement %q in %s could not be parsed: %v", truncateText(base, 200), rel, err))
			p.invalidCount++
			return nil
		}
		requirement.unsupported = true
		p.requirements = append(p.requirements, requirement)
		return nil
	}

	cleaned, option := stripInlineOptions(line)
	if option != "" {
		p.addLimit("UNSUPPORTED_REQUIREMENTS_OPTION", fmt.Sprintf("Requirements option %q in %s is not supported.", option, rel))
		p.unsupportedCount++
		return nil
	}
	requirement, err := parsePythonRequirement(cleaned)
	if err != nil {
		p.addLimit("INVALID_REQUIREMENT", fmt.Sprintf("Requirement %q in %s could not be parsed: %v", truncateText(line, 200), rel, err))
		p.invalidCount++
		return nil
	}
	p.requirements = append(p.requirements, requirement)
	return nil
}

func (p *pythonParser) include(ctx context.Context, rel string, includePath string) error {
	includePath = strings.Trim(strings.TrimSpace(includePath), `"'`)
	if includePath == "" {
		p.addLimit("INVALID_REQUIREMENT", fmt.Sprintf("Include directive in %s has an empty path.", rel))
		p.invalidCount++
		return nil
	}
	if filepath.IsAbs(includePath) {
		p.addLimit("INCLUDE_OUTSIDE_ROOT", fmt.Sprintf("Absolute include path %q in %s is outside the project root and was not read.", includePath, rel))
		p.unsupportedCount++
		return nil
	}
	target := filepath.ToSlash(filepath.Clean(path.Join(path.Dir(rel), includePath)))
	if _, _, err := resolveInsideRoot(p.root, target); err != nil {
		p.addLimit("INCLUDE_OUTSIDE_ROOT", fmt.Sprintf("Include path %q in %s escapes the project root and was not read.", includePath, rel))
		p.unsupportedCount++
		return nil
	}
	if p.includes >= p.limits.MaxIncludes {
		if !p.includeLimitHit {
			p.addLimit("INCLUDE_LIMIT_EXCEEDED", fmt.Sprintf("At most %d included requirements files are read; later includes were skipped.", p.limits.MaxIncludes))
			p.includeLimitHit = true
		}
		p.unsupportedCount++
		return nil
	}
	if _, active := p.active[target]; active {
		p.addLimit("INCLUDE_CYCLE", fmt.Sprintf("Requirements include cycle detected at %s.", target))
		p.unsupportedCount++
		return nil
	}
	if _, seen := p.visited[target]; seen {
		return nil
	}
	p.includes++
	p.active[target] = struct{}{}
	err := p.parseFile(ctx, target, false)
	delete(p.active, target)
	return err
}

func parseIncludeDirective(line string) (string, bool, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", false, false
	}
	switch fields[0] {
	case "-r", "--requirement":
		if len(fields) != 2 {
			return "", true, false
		}
		return fields[1], true, true
	}
	if strings.HasPrefix(fields[0], "--requirement=") {
		value := strings.TrimPrefix(fields[0], "--requirement=")
		return value, true, value != ""
	}
	if strings.HasPrefix(fields[0], "-r=") {
		value := strings.TrimPrefix(fields[0], "-r=")
		return value, true, value != ""
	}
	return "", false, false
}

func stripRequirementComment(line string) string {
	for index := 0; index < len(line); index++ {
		if line[index] != '#' {
			continue
		}
		if index == 0 || line[index-1] == ' ' || line[index-1] == '\t' {
			return line[:index]
		}
	}
	return line
}

func stripInlineOptions(line string) (string, string) {
	fields := strings.Fields(line)
	kept := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.HasPrefix(field, "--hash=") {
			continue
		}
		if strings.HasPrefix(field, "--") || (strings.HasPrefix(field, "-") && field != "-") {
			return "", field
		}
		kept = append(kept, field)
	}
	return strings.Join(kept, " "), ""
}

func isUnsupportedRequirementSource(line string) bool {
	lower := strings.ToLower(line)
	if strings.Contains(lower, " @ ") {
		return true
	}
	prefixes := []string{
		"git+", "hg+", "svn+", "bzr+",
		"http://", "https://", "ftp://", "file:",
		"./", "../", "~/", "/",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func parsePythonRequirement(spec string) (pythonRequirement, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return pythonRequirement{}, fmt.Errorf("empty requirement")
	}

	namePart := spec
	versionPart := ""
	if index := strings.Index(spec, "["); index >= 0 {
		closeIndex := strings.Index(spec[index:], "]")
		if closeIndex < 0 {
			return pythonRequirement{}, fmt.Errorf("unterminated extras")
		}
		namePart = spec[:index]
		versionPart = spec[index+closeIndex+1:]
	} else if index := strings.IndexAny(spec, "<>=!~ "); index >= 0 {
		namePart = spec[:index]
		versionPart = spec[index:]
	}

	namePart = strings.TrimSpace(namePart)
	if namePart == "" || strings.ContainsAny(namePart, " \t") {
		return pythonRequirement{}, fmt.Errorf("invalid package name")
	}
	pkg, err := model.NewPackageIdentity(model.EcosystemPyPI, namePart)
	if err != nil {
		return pythonRequirement{}, err
	}

	versionPart = strings.TrimSpace(versionPart)
	requirement := pythonRequirement{name: pkg.Name}
	if versionPart == "" {
		requirement.other = true
		return requirement, nil
	}
	specifiers, err := parsePythonSpecifiers(versionPart)
	if err != nil {
		return pythonRequirement{}, err
	}
	constraints := make([]string, 0, len(specifiers))
	for _, specifier := range specifiers {
		constraints = append(constraints, specifier.operator+specifier.version)
	}
	requirement.constraint = strings.Join(constraints, ", ")

	if len(specifiers) == 1 {
		specifier := specifiers[0]
		if (specifier.operator == "==" || specifier.operator == "===") && !strings.Contains(specifier.version, "*") {
			requirement.exact = true
			requirement.exactVersion = specifier.version
			return requirement, nil
		}
	}
	requirement.other = true
	return requirement, nil
}

func parsePythonSpecifiers(value string) ([]pythonSpecifier, error) {
	parts := strings.Split(value, ",")
	specifiers := make([]pythonSpecifier, 0, len(parts))
	operators := []string{"===", "==", "~=", "!=", "<=", ">=", "<", ">"}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty version specifier")
		}
		matched := false
		for _, operator := range operators {
			if !strings.HasPrefix(part, operator) {
				continue
			}
			version := strings.TrimSpace(strings.TrimPrefix(part, operator))
			if version == "" || strings.ContainsAny(version, " \t") {
				return nil, fmt.Errorf("invalid version in specifier %q", part)
			}
			specifiers = append(specifiers, pythonSpecifier{operator: operator, version: version})
			matched = true
			break
		}
		if !matched {
			return nil, fmt.Errorf("unsupported version specifier %q", part)
		}
	}
	return specifiers, nil
}

func buildPythonNodes(parser *pythonParser) ([]Node, int, int, int) {
	aggregates := make(map[string]*pythonAggregate)
	for _, requirement := range parser.requirements {
		aggregate := aggregates[requirement.name]
		if aggregate == nil {
			aggregate = &pythonAggregate{name: requirement.name, exact: make(map[string]struct{})}
			aggregates[requirement.name] = aggregate
		}
		if requirement.constraint != "" {
			aggregate.constraints = append(aggregate.constraints, requirement.constraint)
		}
		if requirement.exact {
			aggregate.exact[requirement.exactVersion] = struct{}{}
		}
		if requirement.other {
			aggregate.other = true
		}
		if requirement.unsupported {
			aggregate.unsupported = true
		}
	}

	names := make([]string, 0, len(aggregates))
	for name := range aggregates {
		names = append(names, name)
	}
	sort.Strings(names)

	nodes := make([]Node, 0, len(names))
	resolved := 0
	unresolved := 0
	conflicts := 0
	for _, name := range names {
		if len(nodes) >= parser.limits.MaxNodes {
			parser.addLimit("NODE_LIMIT_EXCEEDED", fmt.Sprintf("The dependency graph is limited to %d nodes; additional direct requirements were not listed.", parser.limits.MaxNodes))
			parser.incomplete = true
			break
		}
		aggregate := aggregates[name]
		constraints := dedupeStrings(aggregate.constraints)
		constraint := strings.Join(constraints, ", ")
		version := ""
		switch {
		case aggregate.unsupported:
			unresolved++
		case len(aggregate.exact) == 1 && !aggregate.other:
			for exact := range aggregate.exact {
				version = exact
			}
			resolved++
		case len(aggregate.exact) > 0:
			parser.addLimit("CONFLICTING_REQUIREMENTS", fmt.Sprintf("Requirement %q combines conflicting exact versions or an exact pin with a range (%s); it is unresolved.", name, displayConstraint(constraint)))
			conflicts++
			unresolved++
		default:
			parser.addLimit("UNRESOLVED_VERSION", fmt.Sprintf("Requirement %q does not use an exact == or === pin (%s); it is unresolved and no latest version is substituted.", name, displayConstraint(constraint)))
			unresolved++
		}

		nodeID := "pypi:" + name
		node := Node{
			DependencyNode: model.DependencyNode{
				ID:             nodeID,
				Package:        model.PackageIdentity{Ecosystem: model.EcosystemPyPI, Name: name},
				Relationship:   model.DependencyDirect,
				DependencyPath: []string{rootNodeID, nodeID},
				Dependencies:   []string{},
			},
			Constraint: constraint,
		}
		if version != "" {
			node.Version = stringPointer(version)
		}
		nodes = append(nodes, node)
	}
	return nodes, resolved, unresolved, conflicts
}

func displayConstraint(constraint string) string {
	if strings.TrimSpace(constraint) == "" {
		return "no version specifier"
	}
	return constraint
}

func truncateText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
