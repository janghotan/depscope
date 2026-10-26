package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"depscope/internal/model"
)

type npmManifest struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	Workspaces           json.RawMessage   `json:"workspaces"`
}

type npmLockDocument struct {
	Name            string                    `json:"name"`
	Version         string                    `json:"version"`
	LockfileVersion json.Number               `json:"lockfileVersion"`
	Packages        map[string]npmLockPackage `json:"packages"`
}

type npmLockPackage struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Link                 bool              `json:"link"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

type npmDeclaredDependency struct {
	name       string
	constraint string
	section    string
}

type npmDependencyRequirement struct {
	name       string
	constraint string
}

func parseNPMProject(ctx context.Context, root string, candidate Candidate, budget *fileBudget, clock func() time.Time) (parsedProject, error) {
	result := parsedProject{
		candidate:   candidate,
		nodes:       []Node{},
		checks:      []model.CheckResult{},
		limitations: []model.Limitation{},
	}
	addLimit := limitationCollector(&result.limitations)
	identifier := candidate.ManifestPath
	if candidate.LockfilePath != "" {
		identifier += " + " + candidate.LockfilePath
	}
	result.source = model.SourceReference{
		ID:          projectNPMPSourceID,
		Provider:    model.SourceProviderNPM,
		Identifier:  identifier,
		RetrievedAt: clock().UTC(),
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}
	manifestData, err := readProjectFile(root, candidate.ManifestPath, budget, addLimit)
	if err != nil {
		result.fatal = true
		addLimit("MANIFEST_UNREADABLE", fmt.Sprintf("package.json could not be read: %v", err))
		result.checks = append(result.checks, model.CheckResult{
			ID:        checkNPMManifest,
			Name:      "npm manifest",
			Status:    model.CheckFailed,
			Message:   fmt.Sprintf("package.json could not be read: %v", err),
			SourceIDs: []string{projectNPMPSourceID},
		})
		result.checks = append(result.checks, unavailableResolutionCheck(model.EcosystemNPM, "Dependency resolution could not run because package.json was unreadable.", projectNPMPSourceID))
		return result, nil
	}
	var manifest npmManifest
	if err := decodeJSONDocument(manifestData, &manifest); err != nil {
		result.fatal = true
		addLimit("MALFORMED_MANIFEST", fmt.Sprintf("package.json is not valid JSON: %v", err))
		result.checks = append(result.checks, model.CheckResult{
			ID:        checkNPMManifest,
			Name:      "npm manifest",
			Status:    model.CheckFailed,
			Message:   fmt.Sprintf("package.json is not valid JSON: %v", err),
			SourceIDs: []string{projectNPMPSourceID},
		})
		result.checks = append(result.checks, unavailableResolutionCheck(model.EcosystemNPM, "Dependency resolution could not run because package.json was malformed.", projectNPMPSourceID))
		return result, nil
	}

	declared, manifestIncomplete := npmDeclaredDependencies(manifest, addLimit)
	manifestName := strings.TrimSpace(manifest.Name)
	if manifestName == "" {
		manifestName = "unnamed npm project"
	}
	manifestMessage := fmt.Sprintf("Read package.json for %s with %d declared direct dependency(ies).", manifestName, len(declared))
	manifestStatus := model.CheckSucceeded
	if manifestIncomplete {
		manifestStatus = model.CheckUnavailable
		manifestMessage += " Some declared dependency metadata could not be interpreted."
	}
	result.checks = append(result.checks, model.CheckResult{
		ID:        checkNPMManifest,
		Name:      "npm manifest",
		Status:    manifestStatus,
		Message:   manifestMessage,
		SourceIDs: []string{projectNPMPSourceID},
	})

	if candidate.LockfilePath == "" {
		result.checks = append(result.checks, model.CheckResult{
			ID:        checkNPMLockfile,
			Name:      "npm lockfile",
			Status:    model.CheckUnavailable,
			Message:   "package-lock.json is missing; resolved versions are unavailable and latest versions are never substituted.",
			SourceIDs: []string{projectNPMPSourceID},
		})
		npmUnresolvedDirectNodes(&result, declared, budget.limits)
		result.checks = append(result.checks, unavailableResolutionCheck(
			model.EcosystemNPM,
			fmt.Sprintf("Resolved 0 of %d direct dependency(ies); package-lock.json is required to determine installed versions.", len(declared)),
			projectNPMPSourceID,
		))
		return result, nil
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}
	lockData, err := readProjectFile(root, candidate.LockfilePath, budget, addLimit)
	if err != nil {
		result.fatal = true
		addLimit("LOCKFILE_UNREADABLE", fmt.Sprintf("package-lock.json could not be read: %v", err))
		result.checks = append(result.checks, model.CheckResult{
			ID:        checkNPMLockfile,
			Name:      "npm lockfile",
			Status:    model.CheckFailed,
			Message:   fmt.Sprintf("package-lock.json could not be read: %v", err),
			SourceIDs: []string{projectNPMPSourceID},
		})
		result.checks = append(result.checks, unavailableResolutionCheck(model.EcosystemNPM, "Dependency resolution could not run because package-lock.json was unreadable.", projectNPMPSourceID))
		return result, nil
	}
	var lock npmLockDocument
	if err := decodeJSONDocument(lockData, &lock); err != nil {
		result.fatal = true
		addLimit("MALFORMED_LOCKFILE", fmt.Sprintf("package-lock.json is not valid JSON: %v", err))
		result.checks = append(result.checks, model.CheckResult{
			ID:        checkNPMLockfile,
			Name:      "npm lockfile",
			Status:    model.CheckFailed,
			Message:   fmt.Sprintf("package-lock.json is not valid JSON: %v", err),
			SourceIDs: []string{projectNPMPSourceID},
		})
		result.checks = append(result.checks, unavailableResolutionCheck(model.EcosystemNPM, "Dependency resolution could not run because package-lock.json was malformed.", projectNPMPSourceID))
		return result, nil
	}
	lockVersion, err := strconv.Atoi(strings.TrimSpace(lock.LockfileVersion.String()))
	if err != nil {
		result.fatal = true
		addLimit("MALFORMED_LOCKFILE", "package-lock.json does not contain an integer lockfileVersion.")
		result.checks = append(result.checks, model.CheckResult{
			ID:        checkNPMLockfile,
			Name:      "npm lockfile",
			Status:    model.CheckFailed,
			Message:   "package-lock.json does not contain an integer lockfileVersion.",
			SourceIDs: []string{projectNPMPSourceID},
		})
		result.checks = append(result.checks, unavailableResolutionCheck(model.EcosystemNPM, "Dependency resolution could not run because the lockfile version is missing or invalid.", projectNPMPSourceID))
		return result, nil
	}
	if lockVersion != 2 && lockVersion != 3 {
		addLimit("UNSUPPORTED_LOCKFILE_VERSION", fmt.Sprintf("package-lock.json lockfileVersion %d is not supported; this version reads only lockfileVersion 2 and 3.", lockVersion))
		result.checks = append(result.checks, model.CheckResult{
			ID:        checkNPMLockfile,
			Name:      "npm lockfile",
			Status:    model.CheckUnavailable,
			Message:   fmt.Sprintf("package-lock.json lockfileVersion %d is unsupported; supported versions are 2 and 3.", lockVersion),
			SourceIDs: []string{projectNPMPSourceID},
		})
		npmUnresolvedDirectNodes(&result, declared, budget.limits)
		result.checks = append(result.checks, unavailableResolutionCheck(
			model.EcosystemNPM,
			"Dependency resolution is unavailable because the lockfile version is unsupported.",
			projectNPMPSourceID,
		))
		return result, nil
	}
	if lock.Packages == nil {
		result.fatal = true
		addLimit("MALFORMED_LOCKFILE", "package-lock.json does not contain a packages map required by lockfileVersion 2 and 3.")
		result.checks = append(result.checks, model.CheckResult{
			ID:        checkNPMLockfile,
			Name:      "npm lockfile",
			Status:    model.CheckFailed,
			Message:   "package-lock.json does not contain a packages map required by lockfileVersion 2 and 3.",
			SourceIDs: []string{projectNPMPSourceID},
		})
		result.checks = append(result.checks, unavailableResolutionCheck(model.EcosystemNPM, "Dependency resolution could not run because the lockfile structure is incomplete.", projectNPMPSourceID))
		return result, nil
	}

	rootEntry, hasRoot := lock.Packages[""]
	lockIncomplete := false
	lockMessage := fmt.Sprintf("Read package-lock.json with lockfileVersion %d and %d package entry(ies).", lockVersion, len(lock.Packages))
	if !hasRoot {
		lockIncomplete = true
		addLimit("LOCKFILE_ROOT_MISSING", "package-lock.json does not contain the root entry required to compare declared dependencies.")
		lockMessage += " The root entry is missing."
	} else {
		conflicts := npmMetadataConflicts(manifest, rootEntry)
		for _, conflict := range conflicts {
			addLimit("CONFLICTING_DEPENDENCY_METADATA", conflict)
		}
		if len(conflicts) > 0 {
			lockIncomplete = true
		}
		if strings.TrimSpace(manifest.Name) != "" && strings.TrimSpace(rootEntry.Name) != "" && manifest.Name != rootEntry.Name {
			addLimit("CONFLICTING_PROJECT_METADATA", fmt.Sprintf("package.json name %q does not match package-lock.json name %q.", manifest.Name, rootEntry.Name))
			lockIncomplete = true
		}
		if strings.TrimSpace(manifest.Version) != "" && strings.TrimSpace(rootEntry.Version) != "" && manifest.Version != rootEntry.Version {
			addLimit("CONFLICTING_PROJECT_METADATA", fmt.Sprintf("package.json version %q does not match package-lock.json version %q.", manifest.Version, rootEntry.Version))
			lockIncomplete = true
		}
	}
	lockStatus := model.CheckSucceeded
	if lockIncomplete {
		lockStatus = model.CheckUnavailable
	}
	result.checks = append(result.checks, model.CheckResult{
		ID:        checkNPMLockfile,
		Name:      "npm lockfile",
		Status:    lockStatus,
		Message:   lockMessage,
		SourceIDs: []string{projectNPMPSourceID},
	})

	builder := npmGraphBuilder{
		packages: lock.Packages,
		limits:   budget.limits,
		nodes:    []Node{},
		nodeIDs:  make(map[string]struct{}),
		visited:  make(map[string]struct{}),
		queue:    []npmQueueItem{},
		addLimit: addLimit,
	}
	builder.build(declared)
	result.nodes = builder.nodes
	if detectNPMCycle(builder.nodes) {
		addLimit("CYCLIC_DEPENDENCY", "The npm dependency graph contains a cycle; traversal stops at already visited package instances.")
	}

	directCount := len(declared)
	resolvedCount := builder.resolved
	unresolvedCount := builder.unresolved
	resolutionIncomplete := manifestIncomplete || lockIncomplete || builder.incomplete
	resolutionStatus := model.CheckSucceeded
	resolutionMessage := fmt.Sprintf("Resolved %d package instance(s) from package-lock.json for %d direct dependency(ies); %d dependency edge(s) remain unresolved.", resolvedCount, directCount, unresolvedCount)
	if resolutionIncomplete {
		resolutionStatus = model.CheckUnavailable
		resolutionMessage = fmt.Sprintf("Dependency resolution is incomplete: %d package instance(s) resolved for %d direct dependency(ies), %d unresolved. Unresolved packages are not scanned and latest versions are never substituted.", resolvedCount, directCount, unresolvedCount)
	}
	result.checks = append(result.checks, model.CheckResult{
		ID:        checkNPMResolution,
		Name:      "npm dependency resolution",
		Status:    resolutionStatus,
		Message:   resolutionMessage,
		SourceIDs: []string{projectNPMPSourceID},
	})
	return result, nil
}

func unavailableResolutionCheck(ecosystem model.Ecosystem, message string, sourceID string) model.CheckResult {
	return model.CheckResult{
		ID:        ecosystemCheckID(ecosystem, "dependency-resolution"),
		Name:      "Dependency resolution",
		Status:    model.CheckUnavailable,
		Message:   message,
		SourceIDs: []string{sourceID},
	}
}

func decodeJSONDocument(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return errors.New("unexpected trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("unexpected trailing data: %w", err)
	}
	return nil
}

func npmDeclaredDependencies(manifest npmManifest, add func(code, message string)) (map[string]npmDeclaredDependency, bool) {
	declared := make(map[string]npmDeclaredDependency)
	incomplete := false
	sections := []struct {
		name string
		deps map[string]string
	}{
		{"dependencies", manifest.Dependencies},
		{"optionalDependencies", manifest.OptionalDependencies},
		{"devDependencies", manifest.DevDependencies},
	}
	for _, section := range sections {
		for _, name := range sortedKeys(section.deps) {
			constraint := strings.TrimSpace(section.deps[name])
			if existing, exists := declared[name]; exists {
				if existing.constraint != constraint {
					add("CONFLICTING_DEPENDENCY_METADATA", fmt.Sprintf("package.json declares %q as %q in %s and %q in %s.", name, existing.constraint, existing.section, constraint, section.name))
					incomplete = true
				}
				continue
			}
			declared[name] = npmDeclaredDependency{name: name, constraint: constraint, section: section.name}
		}
	}
	if len(manifest.PeerDependencies) > 0 {
		add("PEER_DEPENDENCIES_NOT_SCANNED", fmt.Sprintf("package.json declares %d peer dependency(ies); peer dependencies are not resolved by this scan.", len(manifest.PeerDependencies)))
		incomplete = true
	}
	if hasWorkspaces(manifest.Workspaces) {
		add("WORKSPACES_NOT_SCANNED", "package.json declares workspaces; workspace packages and their links are not resolved by this scan.")
		incomplete = true
	}
	return declared, incomplete
}

func hasWorkspaces(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	switch trimmed {
	case "", "null", "[]", "{}", `""`:
		return false
	default:
		return true
	}
}

func npmMetadataConflicts(manifest npmManifest, rootEntry npmLockPackage) []string {
	manifestDeps, _ := npmDeclaredDependencies(manifest, func(string, string) {})
	lockDeps := make(map[string]npmDeclaredDependency)
	sections := []struct {
		name string
		deps map[string]string
	}{
		{"dependencies", rootEntry.Dependencies},
		{"optionalDependencies", rootEntry.OptionalDependencies},
		{"devDependencies", rootEntry.DevDependencies},
	}
	for _, section := range sections {
		for _, name := range sortedKeys(section.deps) {
			if _, exists := lockDeps[name]; exists {
				continue
			}
			lockDeps[name] = npmDeclaredDependency{name: name, constraint: strings.TrimSpace(section.deps[name]), section: section.name}
		}
	}

	conflicts := []string{}
	for _, name := range sortedKeysFromDeclared(manifestDeps) {
		manifestDep := manifestDeps[name]
		lockDep, exists := lockDeps[name]
		if !exists {
			conflicts = append(conflicts, fmt.Sprintf("package-lock.json root entry does not declare %q, which package.json declares.", name))
			continue
		}
		if manifestDep.constraint != lockDep.constraint {
			conflicts = append(conflicts, fmt.Sprintf("package.json declares %q with constraint %q but package-lock.json root entry declares %q.", name, manifestDep.constraint, lockDep.constraint))
		}
	}
	for _, name := range sortedKeysFromDeclared(lockDeps) {
		if _, exists := manifestDeps[name]; !exists {
			conflicts = append(conflicts, fmt.Sprintf("package-lock.json root entry declares %q, which package.json does not declare.", name))
		}
	}
	return conflicts
}

type npmQueueItem struct {
	key        string
	parentID   string
	path       []string
	constraint string
}

type npmGraphBuilder struct {
	packages          map[string]npmLockPackage
	limits            Limits
	nodes             []Node
	nodeIDs           map[string]struct{}
	visited           map[string]struct{}
	queue             []npmQueueItem
	addLimit          func(code, message string)
	incomplete        bool
	stopped           bool
	resolved          int
	unresolved        int
	missingDirect     []string
	missingTransitive []string
	linked            []string
	invalid           []string
	empty             []string
	nodeIndex         map[string]int
}

func (b *npmGraphBuilder) build(declared map[string]npmDeclaredDependency) {
	for _, name := range sortedKeysFromDeclared(declared) {
		dep := declared[name]
		pkg, err := model.NewPackageIdentity(model.EcosystemNPM, dep.name)
		if err != nil {
			b.invalid = append(b.invalid, dep.name)
			b.incomplete = true
			continue
		}
		entryKey := "node_modules/" + pkg.Name
		entry, exists := b.packages[entryKey]
		if !exists {
			b.addUnresolvedDirect(pkg, entryKey, dep.constraint)
			b.missingDirect = append(b.missingDirect, dep.name)
			b.incomplete = true
			continue
		}
		if entry.Link {
			b.addUnresolvedDirect(pkg, entryKey, dep.constraint)
			b.linked = append(b.linked, dep.name)
			b.incomplete = true
			continue
		}
		if entry.Name != "" && entry.Name != pkg.Name {
			b.addUnresolvedDirect(pkg, entryKey, dep.constraint)
			b.invalid = append(b.invalid, dep.name)
			b.incomplete = true
			continue
		}
		if strings.TrimSpace(entry.Version) == "" {
			b.addUnresolvedDirect(pkg, entryKey, dep.constraint)
			b.empty = append(b.empty, dep.name)
			b.incomplete = true
			continue
		}
		nodeID := "npm:" + entryKey
		node := Node{
			DependencyNode: model.DependencyNode{
				ID:             nodeID,
				Package:        pkg,
				Version:        stringPointer(entry.Version),
				Relationship:   model.DependencyDirect,
				DependencyPath: []string{rootNodeID, nodeID},
				Dependencies:   []string{},
			},
			Constraint: dep.constraint,
		}
		if !b.appendNode(node) {
			return
		}
		b.resolved++
		b.visited[entryKey] = struct{}{}
		b.queue = append(b.queue, npmQueueItem{key: entryKey, parentID: nodeID, path: node.DependencyPath, constraint: dep.constraint})
	}

	for index := 0; index < len(b.queue) && !b.stopped; index++ {
		item := b.queue[index]
		entry, exists := b.packages[item.key]
		if !exists {
			continue
		}
		for _, requirement := range npmEntryDependencies(entry) {
			if b.stopped {
				break
			}
			pkg, err := model.NewPackageIdentity(model.EcosystemNPM, requirement.name)
			if err != nil {
				b.invalid = append(b.invalid, requirement.name)
				b.incomplete = true
				continue
			}
			depth := len(item.path)
			if depth > b.limits.MaxDepth {
				b.addLimit("DEPENDENCY_DEPTH_EXCEEDED", fmt.Sprintf("Dependencies deeper than %d package levels were not traversed; coverage is incomplete.", b.limits.MaxDepth))
				b.incomplete = true
				b.stopped = true
				break
			}
			entryKey, found := resolveNPMKey(item.key, requirement.name, b.packages)
			if !found {
				unresolvedID := "npm:unresolved:" + item.key + "/" + requirement.name
				b.appendUnresolvedTransitive(item, unresolvedID, pkg, requirement.constraint)
				b.missingTransitive = append(b.missingTransitive, requirement.name)
				b.incomplete = true
				continue
			}
			entry, exists = b.packages[entryKey]
			if !exists || entry.Link || (entry.Name != "" && entry.Name != pkg.Name) || strings.TrimSpace(entry.Version) == "" {
				unresolvedID := "npm:unresolved:" + entryKey
				b.appendUnresolvedTransitive(item, unresolvedID, pkg, requirement.constraint)
				if entry.Link {
					b.linked = append(b.linked, requirement.name)
				} else if !exists {
					b.missingTransitive = append(b.missingTransitive, requirement.name)
				} else {
					b.empty = append(b.empty, requirement.name)
				}
				b.incomplete = true
				continue
			}
			nodeID := "npm:" + entryKey
			if _, seen := b.visited[entryKey]; seen {
				b.addEdge(item.parentID, nodeID)
				continue
			}
			node := Node{
				DependencyNode: model.DependencyNode{
					ID:             nodeID,
					Package:        pkg,
					Version:        stringPointer(entry.Version),
					Relationship:   model.DependencyTransitive,
					DependencyPath: append(append([]string(nil), item.path...), nodeID),
					Dependencies:   []string{},
				},
				Constraint: requirement.constraint,
			}
			if !b.appendNode(node) {
				break
			}
			b.visited[entryKey] = struct{}{}
			b.resolved++
			b.addEdge(item.parentID, nodeID)
			b.queue = append(b.queue, npmQueueItem{key: entryKey, parentID: nodeID, path: node.DependencyPath, constraint: requirement.constraint})
		}
	}

	if len(b.missingDirect) > 0 {
		b.addLimit("DIRECT_DEPENDENCY_UNRESOLVED", fmt.Sprintf("%d direct dependency(ies) have no installed package entry and were not scanned: %s. Latest versions were not substituted.", len(b.missingDirect), listExamples(b.missingDirect)))
	}
	if len(b.missingTransitive) > 0 {
		b.addLimit("TRANSITIVE_DEPENDENCY_UNRESOLVED", fmt.Sprintf("%d transitive dependency edge(s) have no installed package entry and were not scanned: %s. Latest versions were not substituted.", len(b.missingTransitive), listExamples(b.missingTransitive)))
	}
	if len(b.linked) > 0 {
		b.addLimit("LINKED_DEPENDENCY_UNSUPPORTED", fmt.Sprintf("%d linked or workspace dependency(ies) were not followed: %s.", len(b.linked), listExamples(b.linked)))
	}
	if len(b.empty) > 0 {
		b.addLimit("UNRESOLVED_VERSION", fmt.Sprintf("%d installed package entry(ies) have no version: %s.", len(b.empty), listExamples(b.empty)))
	}
	if len(b.invalid) > 0 {
		b.addLimit("INVALID_DEPENDENCY_NAME", fmt.Sprintf("%d dependency name(s) could not be normalized or matched to lockfile entries: %s.", len(b.invalid), listExamples(b.invalid)))
	}
}

func (b *npmGraphBuilder) appendNode(node Node) bool {
	if len(b.nodes) >= b.limits.MaxNodes {
		b.addLimit("NODE_LIMIT_EXCEEDED", fmt.Sprintf("The dependency graph is limited to %d nodes; additional packages were not traversed.", b.limits.MaxNodes))
		b.incomplete = true
		b.stopped = true
		return false
	}
	if _, exists := b.nodeIDs[node.ID]; exists {
		return false
	}
	b.nodeIDs[node.ID] = struct{}{}
	if b.nodeIndex == nil {
		b.nodeIndex = make(map[string]int)
	}
	b.nodeIndex[node.ID] = len(b.nodes)
	b.nodes = append(b.nodes, node)
	return true
}

func (b *npmGraphBuilder) addUnresolvedDirect(pkg model.PackageIdentity, entryKey string, constraint string) {
	nodeID := "npm:unresolved:" + entryKey
	if _, exists := b.nodeIDs[nodeID]; exists {
		return
	}
	if !b.appendNode(Node{
		DependencyNode: model.DependencyNode{
			ID:             nodeID,
			Package:        pkg,
			Version:        nil,
			Relationship:   model.DependencyDirect,
			DependencyPath: []string{rootNodeID, nodeID},
			Dependencies:   []string{},
		},
		Constraint: constraint,
	}) {
		return
	}
	b.unresolved++
}

func (b *npmGraphBuilder) appendUnresolvedTransitive(item npmQueueItem, nodeID string, pkg model.PackageIdentity, constraint string) bool {
	if _, exists := b.nodeIDs[nodeID]; exists {
		b.addEdge(item.parentID, nodeID)
		return false
	}
	node := Node{
		DependencyNode: model.DependencyNode{
			ID:             nodeID,
			Package:        pkg,
			Version:        nil,
			Relationship:   model.DependencyTransitive,
			DependencyPath: append(append([]string(nil), item.path...), nodeID),
			Dependencies:   []string{},
		},
		Constraint: constraint,
	}
	if !b.appendNode(node) {
		return false
	}
	b.unresolved++
	b.addEdge(item.parentID, nodeID)
	return true
}

func (b *npmGraphBuilder) addEdge(parentID, childID string) {
	index, exists := b.nodeIndex[parentID]
	if !exists {
		return
	}
	b.nodes[index].Dependencies = append(b.nodes[index].Dependencies, childID)
}

func npmEntryDependencies(entry npmLockPackage) []npmDependencyRequirement {
	merged := make(map[string]npmDependencyRequirement)
	sections := []struct {
		deps map[string]string
	}{
		{entry.Dependencies},
		{entry.OptionalDependencies},
	}
	for _, section := range sections {
		for name, constraint := range section.deps {
			if _, exists := merged[name]; exists {
				continue
			}
			merged[name] = npmDependencyRequirement{name: name, constraint: strings.TrimSpace(constraint)}
		}
	}
	result := make([]npmDependencyRequirement, 0, len(merged))
	for _, name := range sortedKeysFromRequirements(merged) {
		result = append(result, merged[name])
	}
	return result
}

func resolveNPMKey(parentKey, name string, packages map[string]npmLockPackage) (string, bool) {
	prefix := parentKey
	for {
		candidate := prefix + "/node_modules/" + name
		if prefix == "" {
			candidate = "node_modules/" + name
		}
		if _, exists := packages[candidate]; exists {
			return candidate, true
		}
		if prefix == "" {
			return "", false
		}
		index := strings.LastIndex(prefix, "/node_modules/")
		if index < 0 {
			prefix = ""
			continue
		}
		prefix = prefix[:index]
	}
}

func npmUnresolvedDirectNodes(result *parsedProject, declared map[string]npmDeclaredDependency, limits Limits) {
	addLimit := limitationCollector(&result.limitations)
	for _, name := range sortedKeysFromDeclared(declared) {
		pkg, err := model.NewPackageIdentity(model.EcosystemNPM, name)
		if err != nil {
			continue
		}
		if len(result.nodes) >= limits.MaxNodes {
			addLimit("NODE_LIMIT_EXCEEDED", fmt.Sprintf("The dependency graph is limited to %d nodes; additional unresolved direct dependencies were not listed.", limits.MaxNodes))
			return
		}
		nodeID := "npm:unresolved:node_modules/" + pkg.Name
		result.nodes = append(result.nodes, Node{
			DependencyNode: model.DependencyNode{
				ID:             nodeID,
				Package:        pkg,
				Version:        nil,
				Relationship:   model.DependencyDirect,
				DependencyPath: []string{rootNodeID, nodeID},
				Dependencies:   []string{},
			},
			Constraint: declared[name].constraint,
		})
	}
}

func detectNPMCycle(nodes []Node) bool {
	adjacency := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		adjacency[node.ID] = node.Dependencies
	}
	state := make(map[string]int, len(nodes))
	var visit func(string) bool
	visit = func(id string) bool {
		switch state[id] {
		case 1:
			return true
		case 2:
			return false
		}
		state[id] = 1
		for _, child := range adjacency[id] {
			if _, exists := adjacency[child]; !exists {
				continue
			}
			if visit(child) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	ids := make([]string, 0, len(adjacency))
	for id := range adjacency {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if visit(id) {
			return true
		}
	}
	return false
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysFromDeclared(values map[string]npmDeclaredDependency) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysFromRequirements(values map[string]npmDependencyRequirement) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stringPointer(value string) *string {
	copy := value
	return &copy
}

func listExamples(values []string) string {
	const limit = 5
	unique := dedupeStrings(values)
	if len(unique) <= limit {
		return strings.Join(unique, ", ")
	}
	return fmt.Sprintf("%s (+%d more)", strings.Join(unique[:limit], ", "), len(unique)-limit)
}
