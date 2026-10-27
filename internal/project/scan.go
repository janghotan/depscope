package project

import (
	"fmt"
	"sort"
	"strings"

	"github.com/janghotan/depscope/internal/engine"
	"github.com/janghotan/depscope/internal/model"
)

type parsedProject struct {
	candidate   Candidate
	source      model.SourceReference
	nodes       []Node
	checks      []model.CheckResult
	limitations []model.Limitation
	fatal       bool
}

type projectScanStats struct {
	ecosystem           model.Ecosystem
	resolved            int
	unresolved          int
	failed              int
	advisories          int
	riskMissing         int
	resolutionSucceeded bool
	sourceIDs           []string
}

func countProjectNodes(nodes []Node) (int, int) {
	resolved := 0
	unresolved := 0
	for _, node := range nodes {
		if node.Version == nil {
			unresolved++
			continue
		}
		resolved++
	}
	return resolved, unresolved
}

func ecosystemCheckPrefix(ecosystem model.Ecosystem) string {
	if ecosystem == model.EcosystemPyPI {
		return "python"
	}
	return string(ecosystem)
}

func ecosystemCheckID(ecosystem model.Ecosystem, suffix string) string {
	return ecosystemCheckPrefix(ecosystem) + ":" + suffix
}

func mergeBatchResults(result *Result, parsed []parsedProject, batch []engine.PackageScanResult) {
	nodes := make(map[string]Node)
	stats := make(map[model.Ecosystem]*projectScanStats)
	projectSources := make(map[model.Ecosystem]string)
	resolvedSeen := make(map[model.Ecosystem]map[string]struct{})
	formats := make(map[model.Ecosystem]FormatInfo, len(result.Formats))
	for _, format := range result.Formats {
		formats[format.Ecosystem] = format
	}
	if result.FindingContexts == nil {
		result.FindingContexts = map[string]DependencyContext{}
	}

	for _, project := range parsed {
		ecosystem := project.candidate.Ecosystem
		item := stats[ecosystem]
		if item == nil {
			item = &projectScanStats{ecosystem: ecosystem, resolutionSucceeded: true}
			stats[ecosystem] = item
			resolvedSeen[ecosystem] = make(map[string]struct{})
		}
		if project.source.ID != "" {
			projectSources[ecosystem] = project.source.ID
		}
		for _, node := range project.nodes {
			nodes[node.ID] = node
			if node.Version == nil {
				item.unresolved++
				continue
			}
			key := node.Package.String() + "@" + *node.Version
			if _, exists := resolvedSeen[ecosystem][key]; !exists {
				resolvedSeen[ecosystem][key] = struct{}{}
				item.resolved++
			}
		}
		for _, check := range project.checks {
			if check.ID == ecosystemCheckID(ecosystem, "dependency-resolution") && check.Status != model.CheckSucceeded {
				item.resolutionSucceeded = false
			}
		}
	}

	paths := make(map[string][][]string)

	for _, item := range batch {
		ecosystem := item.Package.Ecosystem
		itemStats := stats[ecosystem]
		if itemStats == nil {
			itemStats = &projectScanStats{ecosystem: ecosystem, resolutionSucceeded: true}
			stats[ecosystem] = itemStats
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
			finding.ID = projectFindingID(finding)
			key := finding.Package.String()
			if finding.Version != nil {
				key += "@" + *finding.Version
			}
			findingPaths, computed := paths[key]
			if !computed {
				targets := findingNodeIDs(nodes, finding.Package, finding.Version)
				findingPaths = dependencyPaths(nodes, rootNodeID, targets, maxDependencyPathEvidence)
				if len(findingPaths) == 0 {
					findingPaths = nodePathFallback(nodes, targets, maxDependencyPathEvidence)
				}
				paths[key] = findingPaths
			}
			addDependencyPathEvidence(&finding, findingPaths, nodes, projectSources[finding.Package.Ecosystem], result)
			if format, exists := formats[finding.Package.Ecosystem]; exists {
				if context, available := findingDependencyContext(nodes, finding, format); available {
					result.FindingContexts[finding.ID] = context
				}
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

		vulnerabilityCheck := findBatchCheck(item.Checks, "vulnerabilities")
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
		item := stats[ecosystem]
		sourceIDs := dedupeStrings(item.sourceIDs)
		status, message := vulnerabilityAggregateCheck(item)
		addResultCheck(result, model.CheckResult{
			ID:        ecosystemCheckID(ecosystem, "vulnerabilities"),
			Name:      "OSV vulnerability query",
			Status:    status,
			Message:   message,
			SourceIDs: sourceIDs,
		})
		if item.advisories > 0 {
			riskStatus := model.CheckSucceeded
			riskMessage := fmt.Sprintf("Risk scores were computed for all %d %s finding(s).", item.advisories, ecosystem)
			if item.riskMissing > 0 {
				riskStatus = model.CheckFailed
				riskMessage = fmt.Sprintf("Risk scoring was unavailable for some %s findings; they are retained without risk scores.", ecosystem)
			}
			addResultCheck(result, model.CheckResult{
				ID:        ecosystemCheckID(ecosystem, "risk-evaluation"),
				Name:      "Risk evaluation",
				Status:    riskStatus,
				Message:   riskMessage,
				SourceIDs: []string{},
			})
		}
	}
}

func vulnerabilityAggregateCheck(item *projectScanStats) (model.CheckStatus, string) {
	ecosystem := item.ecosystem
	if item.resolved == 0 {
		if item.unresolved == 0 && item.resolutionSucceeded {
			return model.CheckSucceeded, fmt.Sprintf("The resolved %s dependency graph is empty; there are no package versions to query.", ecosystem)
		}
		return model.CheckUnavailable, fmt.Sprintf("No exact %s package versions could be resolved, so OSV was not queried; coverage is incomplete.", ecosystem)
	}
	if item.failed > 0 {
		return model.CheckUnavailable, fmt.Sprintf("OSV data for %d of %d resolved %s package version(s) was unavailable or incomplete; coverage is incomplete.", item.failed, item.resolved, ecosystem)
	}
	if item.advisories == 0 {
		return model.CheckSucceeded, fmt.Sprintf("OSV returned no known advisories affecting %d resolved %s package version(s).", item.resolved, ecosystem)
	}
	return model.CheckSucceeded, fmt.Sprintf("OSV returned %d advisory(ies) affecting %d resolved %s package version(s).", item.advisories, item.resolved, ecosystem)
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

func projectFindingID(finding model.Finding) string {
	parts := []string{finding.ID, finding.Package.String()}
	if finding.Version != nil && *finding.Version != "" {
		parts = append(parts, *finding.Version)
	}
	return strings.Join(parts, "@")
}

func addDependencyPathEvidence(finding *model.Finding, paths [][]string, nodes map[string]Node, sourceID string, result *Result) {
	if len(paths) == 0 {
		return
	}
	if strings.TrimSpace(sourceID) == "" {
		addResultLimitation(result, "INVALID_SOURCE_REFERENCE", fmt.Sprintf("Dependency paths for %s could not be attributed to a project source.", finding.Package.String()))
		return
	}

	seen := make(map[string]struct{})
	pathEvidence := 0
	for _, evidence := range finding.Evidence {
		if evidence.Kind == model.EvidenceDependencyPath {
			seen[evidence.Summary] = struct{}{}
			pathEvidence++
		}
	}

	for _, path := range paths {
		if pathEvidence >= maxDependencyPathEvidence {
			const omission = "Additional dependency paths are omitted after the first 5 paths."
			if _, exists := seen[omission]; !exists {
				finding.Evidence = append(finding.Evidence, model.Evidence{
					Kind:     model.EvidenceDependencyPath,
					Summary:  omission,
					SourceID: sourceID,
				})
			}
			break
		}
		summary := "Dependency path: " + strings.Join(pathLabels(path, nodes), " -> ")
		if _, exists := seen[summary]; exists {
			continue
		}
		seen[summary] = struct{}{}
		finding.Evidence = append(finding.Evidence, model.Evidence{
			Kind:     model.EvidenceDependencyPath,
			Summary:  summary,
			SourceID: sourceID,
		})
		pathEvidence++
	}
}

func pathLabels(path []string, nodes map[string]Node) []string {
	labels := make([]string, 0, len(path))
	for _, id := range path {
		if id == rootNodeID {
			labels = append(labels, rootNodeID)
			continue
		}
		node, exists := nodes[id]
		if !exists {
			labels = append(labels, id)
			continue
		}
		if node.Version == nil {
			labels = append(labels, node.Package.String()+" (unresolved)")
			continue
		}
		labels = append(labels, node.Package.String()+"@"+*node.Version)
	}
	return labels
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
	sort.SliceStable(result.Dependencies.Nodes, func(i, j int) bool {
		left, right := result.Dependencies.Nodes[i], result.Dependencies.Nodes[j]
		if relationshipRank(left.Relationship) != relationshipRank(right.Relationship) {
			return relationshipRank(left.Relationship) < relationshipRank(right.Relationship)
		}
		if left.Package.String() != right.Package.String() {
			return left.Package.String() < right.Package.String()
		}
		leftVersion, rightVersion := nodeVersion(left), nodeVersion(right)
		if leftVersion != rightVersion {
			return leftVersion < rightVersion
		}
		return left.ID < right.ID
	})
	for index := range result.Dependencies.Nodes {
		sort.Strings(result.Dependencies.Nodes[index].Dependencies)
		result.Dependencies.Nodes[index].Dependencies = dedupeStrings(result.Dependencies.Nodes[index].Dependencies)
	}

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

func relationshipRank(relationship model.DependencyRelationship) int {
	switch relationship {
	case model.DependencyRoot:
		return 0
	case model.DependencyDirect:
		return 1
	default:
		return 2
	}
}

func nodeVersion(node Node) string {
	if node.Version == nil {
		return ""
	}
	return *node.Version
}

func riskScoreValue(risk *model.RiskAssessment) float64 {
	if risk == nil {
		return -1
	}
	return risk.Score
}
