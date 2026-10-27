package project

import (
	"sort"
	"strings"

	"github.com/janghotan/depscope/internal/model"
)

// dependencyScopeRank 固定分类输出顺序，保证多次扫描结果稳定。
var dependencyScopeRank = map[DependencyScope]int{
	DependencyScopeProduction:  0,
	DependencyScopeDevelopment: 1,
	DependencyScopeOptional:    2,
	DependencyScopeUnknown:     3,
}

// normalizeScopes 去重并排序分类；没有可用分类时返回明确的 unknown。
func normalizeScopes(scopes []DependencyScope) []DependencyScope {
	seen := make(map[DependencyScope]struct{}, len(scopes))
	result := make([]DependencyScope, 0, len(scopes))
	for _, scope := range scopes {
		if _, known := dependencyScopeRank[scope]; !known {
			continue
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	if len(result) == 0 {
		return []DependencyScope{DependencyScopeUnknown}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return dependencyScopeRank[result[i]] < dependencyScopeRank[result[j]]
	})
	return result
}

// mergeScopes 合并多个来源的分类并保持去重顺序。
func mergeScopes(groups ...[]DependencyScope) []DependencyScope {
	combined := []DependencyScope{}
	for _, group := range groups {
		combined = append(combined, group...)
	}
	return normalizeScopes(combined)
}

// normalizeEvidence 去重并按路径、字段排序分类依据。
func normalizeEvidence(evidence []DependencyEvidence) []DependencyEvidence {
	seen := make(map[DependencyEvidence]struct{}, len(evidence))
	result := make([]DependencyEvidence, 0, len(evidence))
	for _, item := range evidence {
		if strings.TrimSpace(item.Path) == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		return result[i].Field < result[j].Field
	})
	return result
}

// resolutionFromChecks 根据对应生态的依赖解析检查推导格式级完整程度。
func resolutionFromChecks(checks []model.CheckResult, ecosystem model.Ecosystem) DependencyResolution {
	id := ecosystemCheckID(ecosystem, "dependency-resolution")
	for _, check := range checks {
		if check.ID != id {
			continue
		}
		if check.Status == model.CheckSucceeded {
			return DependencyResolutionComplete
		}
		return DependencyResolutionPartial
	}
	return DependencyResolutionUnknown
}

// findingNodeIDs 返回与发现包和版本匹配的所有图节点 ID。
func findingNodeIDs(nodes map[string]Node, pkg model.PackageIdentity, version *string) []string {
	ids := make([]string, 0)
	for id, node := range nodes {
		if node.Package != pkg {
			continue
		}
		if version != nil && (node.Version == nil || *node.Version != *version) {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// findingDependencyContext 从匹配节点汇总关系、分类与证据；
// 没有匹配节点时返回 false，调用方不应展示虚构上下文。
func findingDependencyContext(nodes map[string]Node, finding model.Finding, format FormatInfo) (DependencyContext, bool) {
	ids := findingNodeIDs(nodes, finding.Package, finding.Version)
	if len(ids) == 0 {
		return DependencyContext{}, false
	}
	relationship := model.DependencyTransitive
	scopes := []DependencyScope{}
	evidence := []DependencyEvidence{}
	for _, id := range ids {
		node := nodes[id]
		if node.Relationship == model.DependencyDirect {
			relationship = model.DependencyDirect
		}
		scopes = append(scopes, node.Scopes...)
		evidence = append(evidence, node.ScopeEvidence...)
	}
	if len(evidence) == 0 {
		evidence = append(evidence, DependencyEvidence{Path: format.ManifestPath})
		if format.Ecosystem == model.EcosystemNPM && format.LockfilePath != "" {
			evidence = append(evidence, DependencyEvidence{Path: format.LockfilePath})
		}
	}
	return DependencyContext{
		Relationship: relationship,
		Scopes:       normalizeScopes(scopes),
		Resolution:   format.Resolution,
		Evidence:     normalizeEvidence(evidence),
	}, true
}

// dependencyPaths 返回从项目根到目标节点的最多 limit 条不同路径。
// 采用按路径广度优先的受控枚举：每个节点最多排队 limit 条路径，
// 既保留直接路径与多父节点路径，又避免在大型图中无限增长。
func dependencyPaths(nodes map[string]Node, rootID string, targets []string, limit int) [][]string {
	if limit <= 0 || len(targets) == 0 || strings.TrimSpace(rootID) == "" {
		return nil
	}
	children := make(map[string][]string, len(nodes)+1)
	for id, node := range nodes {
		children[id] = append([]string(nil), node.Dependencies...)
	}
	rootChildren := []string{}
	for id, node := range nodes {
		if node.Relationship == model.DependencyDirect {
			rootChildren = append(rootChildren, id)
		}
	}
	children[rootID] = rootChildren
	for id := range children {
		sort.Strings(children[id])
	}

	targetSet := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		targetSet[target] = struct{}{}
	}

	type pathState struct {
		id   string
		path []string
	}
	queue := []pathState{{id: rootID, path: []string{rootID}}}
	queued := map[string]int{rootID: 1}
	results := make([][]string, 0, limit)
	seen := make(map[string]struct{}, limit)
	for index := 0; index < len(queue) && len(results) < limit; index++ {
		state := queue[index]
		if _, isTarget := targetSet[state.id]; isTarget {
			key := strings.Join(state.path, "\x00")
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				results = append(results, state.path)
				if len(results) >= limit {
					break
				}
			}
		}
		for _, child := range children[state.id] {
			if containsPathNode(state.path, child) {
				continue
			}
			if queued[child] >= limit {
				continue
			}
			queued[child]++
			next := append(append([]string(nil), state.path...), child)
			queue = append(queue, pathState{id: child, path: next})
		}
	}
	return results
}

// containsPathNode 判断节点是否已在当前路径中，用于阻止循环路径。
func containsPathNode(path []string, id string) bool {
	for _, item := range path {
		if item == id {
			return true
		}
	}
	return false
}

// nodePathFallback 返回匹配节点记录的单条路径，用于图中不存在可达路径的情况。
func nodePathFallback(nodes map[string]Node, ids []string, limit int) [][]string {
	paths := [][]string{}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		node, exists := nodes[id]
		if !exists || len(node.DependencyPath) == 0 {
			continue
		}
		path := append([]string(nil), node.DependencyPath...)
		key := strings.Join(path, "\x00")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		paths = append(paths, path)
		if len(paths) >= limit {
			break
		}
	}
	return paths
}
