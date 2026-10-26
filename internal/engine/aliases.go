package engine

import (
	"sort"
	"strings"
)

// dedupeAdvisoriesByAlias 合并指向同一底层漏洞的多条公告记录。
//
// OSV 会为同一个漏洞分别收录 GHSA、PYSEC、CVE 等记录，它们通过 aliases
// 相互指向。只按 id 去重会把同一漏洞统计多次，因此这里用并查集把
// “记录 id + 全部别名”聚成连通分量，每个分量只保留一条公告作为代表。
// 被合并记录的标识会并入代表公告的 Aliases，避免丢失追溯信息。
//
// 返回去重后的公告（保持输入顺序）与被合并掉的记录数。
func dedupeAdvisoriesByAlias(advisories []Advisory) ([]Advisory, int) {
	if len(advisories) < 2 {
		if len(advisories) == 1 {
			return []Advisory{cloneAdvisory(advisories[0])}, 0
		}
		return nil, 0
	}

	parent := make(map[string]string, len(advisories)*3)
	var find func(string) string
	find = func(key string) string {
		root, ok := parent[key]
		if !ok {
			parent[key] = key
			return key
		}
		if root == key {
			return key
		}
		resolved := find(root)
		parent[key] = resolved
		return resolved
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			// 固定按字典序选择根节点，保证结果与输入顺序无关。
			if ra < rb {
				parent[rb] = ra
			} else {
				parent[ra] = rb
			}
		}
	}

	identity := func(advisory Advisory) []string {
		keys := make([]string, 0, len(advisory.Aliases)+1)
		if id := strings.TrimSpace(advisory.ID); id != "" {
			keys = append(keys, identityKey(id))
		}
		for _, alias := range advisory.Aliases {
			if trimmed := strings.TrimSpace(alias); trimmed != "" {
				keys = append(keys, identityKey(trimmed))
			}
		}
		return keys
	}

	for _, advisory := range advisories {
		keys := identity(advisory)
		if len(keys) == 0 {
			continue
		}
		find(keys[0])
		for _, key := range keys[1:] {
			union(keys[0], key)
		}
	}

	// 按连通分量分组；组内保留输入顺序，便于产出稳定的代表。
	groups := make(map[string][]Advisory, len(advisories))
	order := make([]string, 0, len(advisories))
	for _, advisory := range advisories {
		keys := identity(advisory)
		if len(keys) == 0 {
			// 没有可用标识的记录各自独立，不与任何公告合并。
			absolute := "missing-identity:" + advisory.ID
			groups[absolute] = append(groups[absolute], advisory)
			order = append(order, absolute)
			continue
		}
		root := find(keys[0])
		if _, exists := groups[root]; !exists {
			order = append(order, root)
		}
		groups[root] = append(groups[root], advisory)
	}

	result := make([]Advisory, 0, len(groups))
	merged := 0
	for _, key := range order {
		members := groups[key]
		representative := cloneAdvisory(members[0])
		if len(members) > 1 {
			representative.Aliases = mergeAliasSets(members, representative.ID)
			merged += len(members) - 1
		}
		result = append(result, representative)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, merged
}

// identityKey 统一别名比较口径：去空白并忽略大小写。
func identityKey(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

// mergeAliasSets 汇总组内全部标识（不含代表公告自身的 id），去重后排序。
func mergeAliasSets(members []Advisory, representativeID string) []string {
	seen := make(map[string]struct{}, len(members)*3)
	aliases := make([]string, 0, len(members)*3)
	add := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || identityKey(trimmed) == identityKey(representativeID) {
			return
		}
		key := identityKey(trimmed)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		aliases = append(aliases, trimmed)
	}
	for _, advisory := range members {
		add(advisory.ID)
		for _, alias := range advisory.Aliases {
			add(alias)
		}
	}
	sort.Strings(aliases)
	return aliases
}

// cloneAdvisory 复制公告，避免调用方修改切片共享的底层数组。
func cloneAdvisory(advisory Advisory) Advisory {
	cloned := advisory
	cloned.Aliases = append([]string(nil), advisory.Aliases...)
	cloned.References = append([]string(nil), advisory.References...)
	cloned.FixedVersions = append([]string(nil), advisory.FixedVersions...)
	cloned.AffectedRanges = append([]string(nil), advisory.AffectedRanges...)
	return cloned
}
