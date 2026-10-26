package model

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
)

// Validate 执行跨字段语义校验；JSON Schema 负责结构校验，本方法负责一致性规则。
func (r ScanReport) Validate() error {
	var errs []error
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	if r.SchemaVersion != SchemaVersion {
		add("schemaVersion = %q, want %q", r.SchemaVersion, SchemaVersion)
	}
	if !r.DataMode.IsValid() {
		add("dataMode %q is not supported", r.DataMode)
	}
	if r.ScannedAt.IsZero() {
		add("scannedAt must be set")
	}
	if !r.Status.IsValid() {
		add("status %q is not supported", r.Status)
	} else if !r.Status.IsFinal() {
		add("status %q must be final in a completed report", r.Status)
	}

	switch r.DataMode {
	case DataModeDemo:
		if r.DemoNotice == nil || strings.TrimSpace(*r.DemoNotice) == "" {
			add("demoNotice is required when dataMode is demo")
		}
	case DataModeLive:
		if r.DemoNotice != nil {
			add("demoNotice must be null when dataMode is live")
		}
	}

	if err := r.Target.Validate(); err != nil {
		add("%v", err)
	}

	if r.Findings == nil {
		add("findings must be an empty array, not null")
	}
	if r.Checks == nil {
		add("checks must be an empty array, not null")
	}
	if r.Sources == nil {
		add("sources must be an empty array, not null")
	}
	if r.Limitations == nil {
		add("limitations must be an empty array, not null")
	}
	if r.Dependencies.Nodes == nil {
		add("dependencies.nodes must be an empty array, not null")
	}

	sourceIDs := make(map[string]struct{}, len(r.Sources))
	for index, source := range r.Sources {
		path := fmt.Sprintf("sources[%d]", index)
		if err := source.Validate(); err != nil {
			add("%s: %v", path, err)
		}
		if source.ID == "" {
			add("%s: id must not be empty", path)
			continue
		}
		if _, exists := sourceIDs[source.ID]; exists {
			add("%s: duplicate id %q", path, source.ID)
		}
		sourceIDs[source.ID] = struct{}{}

		switch r.DataMode {
		case DataModeDemo:
			if !source.Provider.IsDemo() {
				add("%s: demo data sources must use provider %q", path, SourceProviderDemo)
			}
		case DataModeLive:
			if source.Provider.IsDemo() {
				add("%s: live data must not reference demo sources", path)
			}
		}
	}

	nodes, rootCount := validateDependencyNodes(r.Dependencies, add)
	if r.Dependencies.RootNodeID == "" {
		add("dependencies.rootNodeId must not be empty")
	}
	root, hasRoot := nodes[r.Dependencies.RootNodeID]
	if r.Dependencies.RootNodeID != "" && !hasRoot {
		add("dependencies.rootNodeId %q does not reference a node", r.Dependencies.RootNodeID)
	}
	if rootCount != 1 {
		add("dependencies must contain exactly one root node, found %d", rootCount)
	}
	if hasRoot {
		if root.Package != r.Target.Package {
			add("root node package %s does not match target package %s", root.Package, r.Target.Package)
		}
		if !stringPointersEqual(root.Version, r.Target.ResolvedVersion) {
			add("root node version does not match target.resolvedVersion")
		}
	}

	nodeVersions := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		if node.Version != nil {
			nodeVersions[packageVersionKey(node.Package, *node.Version)] = struct{}{}
		}
	}

	findingIDs := make(map[string]struct{}, len(r.Findings))
	confirmedCount := 0
	heuristicCount := 0
	for index, finding := range r.Findings {
		path := fmt.Sprintf("findings[%d]", index)
		if finding.ID == "" {
			add("%s: id must not be empty", path)
		} else if _, exists := findingIDs[finding.ID]; exists {
			add("%s: duplicate id %q", path, finding.ID)
		} else {
			findingIDs[finding.ID] = struct{}{}
		}
		if !finding.Kind.IsValid() {
			add("%s: kind %q is not supported", path, finding.Kind)
		} else if finding.Kind == FindingConfirmed {
			confirmedCount++
		} else {
			heuristicCount++
		}
		if strings.TrimSpace(finding.Title) == "" {
			add("%s: title must not be empty", path)
		}
		if strings.TrimSpace(finding.Description) == "" {
			add("%s: description must not be empty", path)
		}
		if !finding.Severity.IsValid() {
			add("%s: severity %q is not supported", path, finding.Severity)
		}
		if err := finding.Package.Validate(); err != nil {
			add("%s.package: %v", path, err)
		}
		if finding.Version != nil {
			if strings.TrimSpace(*finding.Version) == "" {
				add("%s: version must not be empty when present", path)
			} else if _, exists := nodeVersions[packageVersionKey(finding.Package, *finding.Version)]; !exists {
				add("%s: package/version is not present in the dependency graph", path)
			}
		}
		if finding.Kind == FindingConfirmed && finding.Version == nil {
			add("%s: confirmed finding requires a resolved version", path)
		}

		if len(finding.Evidence) == 0 {
			add("%s: evidence must not be empty", path)
		}
		hasAdvisoryEvidence := false
		for evidenceIndex, evidence := range finding.Evidence {
			evidencePath := fmt.Sprintf("%s.evidence[%d]", path, evidenceIndex)
			if !evidence.Kind.IsValid() {
				add("%s: kind %q is not supported", evidencePath, evidence.Kind)
			}
			if evidence.Kind == EvidenceAdvisory {
				hasAdvisoryEvidence = true
				if strings.TrimSpace(evidence.Reference) == "" {
					add("%s: advisory evidence requires a reference", evidencePath)
				}
			}
			if strings.TrimSpace(evidence.Summary) == "" {
				add("%s: summary must not be empty", evidencePath)
			}
			if evidence.SourceID == "" {
				add("%s: sourceId must not be empty", evidencePath)
			} else if _, exists := sourceIDs[evidence.SourceID]; !exists {
				add("%s: sourceId %q does not reference a source", evidencePath, evidence.SourceID)
			}
		}
		if finding.Kind == FindingConfirmed && !hasAdvisoryEvidence {
			add("%s: confirmed finding requires advisory evidence", path)
		}

		if finding.Risk != nil {
			if math.IsNaN(finding.Risk.Score) || math.IsInf(finding.Risk.Score, 0) || finding.Risk.Score < 0 || finding.Risk.Score > 100 {
				add("%s.risk.score must be finite and between 0 and 100", path)
			}
			if strings.TrimSpace(finding.Risk.Rationale) == "" {
				add("%s.risk.rationale must not be empty", path)
			}
		}
	}

	checkIDs := make(map[string]struct{}, len(r.Checks))
	for index, check := range r.Checks {
		path := fmt.Sprintf("checks[%d]", index)
		if check.ID == "" {
			add("%s: id must not be empty", path)
		} else if _, exists := checkIDs[check.ID]; exists {
			add("%s: duplicate id %q", path, check.ID)
		} else {
			checkIDs[check.ID] = struct{}{}
		}
		if strings.TrimSpace(check.Name) == "" {
			add("%s: name must not be empty", path)
		}
		if !check.Status.IsValid() {
			add("%s: status %q is not supported", path, check.Status)
		}
		if strings.TrimSpace(check.Message) == "" {
			add("%s: message must not be empty", path)
		}
		if check.SourceIDs == nil {
			add("%s: sourceIds must be an empty array, not null", path)
		}
		for sourceIndex, sourceID := range check.SourceIDs {
			if _, exists := sourceIDs[sourceID]; !exists {
				add("%s.sourceIds[%d]: sourceId %q does not reference a source", path, sourceIndex, sourceID)
			}
		}
	}

	if r.Summary.ConfirmedFindings != confirmedCount {
		add("summary.confirmedFindings = %d, findings contain %d", r.Summary.ConfirmedFindings, confirmedCount)
	}
	if r.Summary.HeuristicWarnings != heuristicCount {
		add("summary.heuristicWarnings = %d, findings contain %d", r.Summary.HeuristicWarnings, heuristicCount)
	}
	highest := HighestSeverity(r.Findings)
	if !severityPointersEqual(r.Summary.HighestSeverity, highest) {
		add("summary.highestSeverity does not match findings")
	}
	if r.Summary.HighestSeverity != nil && !r.Summary.HighestSeverity.IsValid() {
		add("summary.highestSeverity %q is not supported", *r.Summary.HighestSeverity)
	}

	for index, limitation := range r.Limitations {
		path := fmt.Sprintf("limitations[%d]", index)
		if strings.TrimSpace(limitation.Code) == "" {
			add("%s: code must not be empty", path)
		}
		if strings.TrimSpace(limitation.Message) == "" {
			add("%s: message must not be empty", path)
		}
	}

	derived := DeriveFinalStatus(r.Checks, r.Findings)
	switch r.Status {
	case StatusClean, StatusFindings:
		if len(r.Sources) == 0 {
			add("status %q requires at least one source reference", r.Status)
		}
		if derived != r.Status {
			add("status %q is inconsistent with derived status %q", r.Status, derived)
		}
	case StatusIncomplete:
		if derived != StatusIncomplete {
			add("status %q is inconsistent with derived status %q", r.Status, derived)
		}
	case StatusFailed:
		if !hasFailedCheck(r.Checks) && len(r.Limitations) == 0 {
			add("status %q requires a failed check or a limitation", r.Status)
		}
	}

	return errors.Join(errs...)
}

// Validate 检查来源引用是否完整且可解析。
func (s SourceReference) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("id must not be empty")
	}
	if !s.Provider.IsValid() {
		return fmt.Errorf("provider %q is not supported", s.Provider)
	}
	if strings.TrimSpace(s.Identifier) == "" {
		return fmt.Errorf("identifier must not be empty")
	}
	if s.RetrievedAt.IsZero() {
		return fmt.Errorf("retrievedAt must be set")
	}
	if s.URL != nil {
		if strings.TrimSpace(*s.URL) == "" {
			return fmt.Errorf("url must not be empty when present")
		}
		parsed, err := url.ParseRequestURI(*s.URL)
		if err != nil || parsed.Scheme == "" {
			return fmt.Errorf("url %q must be an absolute URI", *s.URL)
		}
	}
	return nil
}

func validateDependencyNodes(graph DependencyGraph, add func(string, ...any)) (map[string]DependencyNode, int) {
	nodes := make(map[string]DependencyNode, len(graph.Nodes))
	rootCount := 0

	for index, node := range graph.Nodes {
		path := fmt.Sprintf("dependencies.nodes[%d]", index)
		if node.ID == "" {
			add("%s: id must not be empty", path)
			continue
		}
		if _, exists := nodes[node.ID]; exists {
			add("%s: duplicate id %q", path, node.ID)
			continue
		}
		nodes[node.ID] = node

		if err := node.Package.Validate(); err != nil {
			add("%s.package: %v", path, err)
		}
		if node.Version != nil && strings.TrimSpace(*node.Version) == "" {
			add("%s: version must not be empty when present", path)
		}
		if !node.Relationship.IsValid() {
			add("%s: relationship %q is not supported", path, node.Relationship)
		}
		if node.Relationship == DependencyRoot {
			rootCount++
		}
		if node.DependencyPath == nil {
			add("%s: dependencyPath must be an empty array, not null", path)
		}
		if node.Dependencies == nil {
			add("%s: dependencies must be an empty array, not null", path)
		}
	}

	for id, node := range nodes {
		path := fmt.Sprintf("dependencies.nodes[%s]", id)
		if len(node.DependencyPath) == 0 {
			add("%s: dependencyPath must not be empty", path)
		} else {
			if node.DependencyPath[0] != graph.RootNodeID {
				add("%s: dependencyPath must start with root node %q", path, graph.RootNodeID)
			}
			if node.DependencyPath[len(node.DependencyPath)-1] != id {
				add("%s: dependencyPath must end with the node id", path)
			}
			for _, pathID := range node.DependencyPath {
				if _, exists := nodes[pathID]; !exists {
					add("%s: dependencyPath references unknown node %q", path, pathID)
				}
			}
			for index := 0; index < len(node.DependencyPath)-1; index++ {
				current := nodes[node.DependencyPath[index]]
				next := node.DependencyPath[index+1]
				if !containsString(current.Dependencies, next) {
					add("%s: dependencyPath edge %q -> %q is not present in dependencies", path, current.ID, next)
				}
			}
		}

		switch node.Relationship {
		case DependencyRoot:
			if len(node.DependencyPath) != 1 || node.DependencyPath[0] != node.ID {
				add("%s: root dependencyPath must contain only the root node id", path)
			}
		case DependencyDirect:
			if len(node.DependencyPath) != 2 {
				add("%s: direct dependency path length = %d, want 2", path, len(node.DependencyPath))
			}
		case DependencyTransitive:
			if len(node.DependencyPath) < 3 {
				add("%s: transitive dependency path length = %d, want at least 3", path, len(node.DependencyPath))
			}
		}

		for _, dependencyID := range node.Dependencies {
			if _, exists := nodes[dependencyID]; !exists {
				add("%s: dependencies references unknown node %q", path, dependencyID)
			}
		}
	}

	return nodes, rootCount
}

func hasFailedCheck(checks []CheckResult) bool {
	for _, check := range checks {
		if check.Status == CheckFailed {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func packageVersionKey(identity PackageIdentity, version string) string {
	return identity.String() + "@" + version
}

func stringPointersEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func severityPointersEqual(left, right *Severity) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
