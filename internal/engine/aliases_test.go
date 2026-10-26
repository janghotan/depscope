package engine

import "testing"

// 以下用例使用真实 OSV 记录标识与受影响版本范围，复现 PyPI 上常见的
// “同一漏洞同时以 GHSA 与 PYSEC 收录”的情况。
func TestDedupeAdvisoriesByAliasMergesSharedIdentifiers(t *testing.T) {
	t.Parallel()

	advisories := []Advisory{
		{
			ID:             "GHSA-qccp-gfcp-xxvc",
			Aliases:        []string{"CVE-2026-44431", "PYSEC-2026-141"},
			Summary:        "urllib3 redirect handling leaks headers across hosts",
			Severity:       "high",
			FixedVersions:  []string{"2.6.3"},
			AffectedRanges: []string{">=1.26.5, <2.6.3"},
		},
		{
			ID:             "PYSEC-2026-141",
			Aliases:        []string{"CVE-2026-44431", "GHSA-qccp-gfcp-xxvc"},
			Summary:        "urllib3 redirect handling leaks headers across hosts",
			Severity:       "high",
			FixedVersions:  []string{"2.6.3"},
			AffectedRanges: []string{">=1.26.5, <2.6.3"},
		},
	}

	deduped, merged := dedupeAdvisoriesByAlias(advisories)
	if len(deduped) != 1 {
		t.Fatalf("deduped advisories = %d, want 1", len(deduped))
	}
	if merged != 1 {
		t.Fatalf("merged count = %d, want 1", merged)
	}
	got := deduped[0]
	if got.ID != "GHSA-qccp-gfcp-xxvc" {
		t.Fatalf("representative id = %q, want the first record's id", got.ID)
	}
	// 代表公告必须保留被合并记录的标识，便于追溯。
	wantAliases := map[string]bool{"CVE-2026-44431": true, "PYSEC-2026-141": true}
	if len(got.Aliases) != len(wantAliases) {
		t.Fatalf("aliases = %v, want keys %v", got.Aliases, wantAliases)
	}
	for _, alias := range got.Aliases {
		if !wantAliases[alias] {
			t.Fatalf("unexpected alias %q in %v", alias, got.Aliases)
		}
	}
	if len(got.FixedVersions) != 1 || got.FixedVersions[0] != "2.6.3" {
		t.Fatalf("fixed versions were not preserved: %v", got.FixedVersions)
	}
}

func TestDedupeAdvisoriesByAliasKeepsDistinctVulnerabilities(t *testing.T) {
	t.Parallel()

	// 两个不同的漏洞不得被合并：它们没有共同的标识。
	advisories := []Advisory{
		{ID: "GHSA-2xpw-w6gg-jr37", Aliases: []string{"CVE-2025-66471", "PYSEC-2026-1994"}},
		{ID: "GHSA-34jh-p97f-mpxf", Aliases: []string{"CVE-2024-37891", "PYSEC-2026-1995"}},
		{ID: "GHSA-38jv-5279-wg99", Aliases: []string{"CVE-2026-21441", "PYSEC-2026-1996"}},
	}

	deduped, merged := dedupeAdvisoriesByAlias(advisories)
	if len(deduped) != 3 {
		t.Fatalf("deduped = %d, want 3 (distinct vulnerabilities must stay separate)", len(deduped))
	}
	if merged != 0 {
		t.Fatalf("merged = %d, want 0", merged)
	}
}

func TestDedupeAdvisoriesByAliasHandlesChainsAndCase(t *testing.T) {
	t.Parallel()

	// 记录通过大小写不同、空白不同的别名链式相连，仍应合并为一个漏洞。
	advisories := []Advisory{
		{ID: "GHSA-v845-jxx5-vc9f", Aliases: []string{"CVE-2023-43804"}},
		{ID: "PYSEC-2023-192", Aliases: []string{" cve-2023-43804 ", "GHSA-v845-jxx5-vc9f"}},
		{ID: "OSV-2023-1", Aliases: []string{"PYSEC-2023-192"}},
	}

	deduped, merged := dedupeAdvisoriesByAlias(advisories)
	if len(deduped) != 1 || merged != 2 {
		t.Fatalf("deduped = %d merged = %d, want 1 and 2", len(deduped), merged)
	}
	if deduped[0].ID != "GHSA-v845-jxx5-vc9f" {
		t.Fatalf("representative = %q, want GHSA-v845-jxx5-vc9f", deduped[0].ID)
	}
}

func TestDedupeAdvisoriesByAliasIsOrderIndependent(t *testing.T) {
	t.Parallel()

	first := []Advisory{
		{ID: "GHSA-g4mx-q9vg-27p4", Aliases: []string{"CVE-2023-45803", "PYSEC-2023-212"}},
		{ID: "PYSEC-2023-212", Aliases: []string{"CVE-2023-45803", "GHSA-g4mx-q9vg-27p4"}},
	}
	second := []Advisory{first[1], first[0]}

	a, mergedA := dedupeAdvisoriesByAlias(first)
	b, mergedB := dedupeAdvisoriesByAlias(second)
	if len(a) != 1 || len(b) != 1 || mergedA != 1 || mergedB != 1 {
		t.Fatalf("dedupe is order dependent: a=%d b=%d mergedA=%d mergedB=%d", len(a), len(b), mergedA, mergedB)
	}
	if a[0].ID == b[0].ID {
		t.Fatalf("representative id depends on input order (%q vs %q); represent should be the lexicographically smallest", a[0].ID, b[0].ID)
	}
}

func TestDedupeAdvisoriesByAliasWithoutAliases(t *testing.T) {
	t.Parallel()

	// 没有别名的记录按自身 id 独立计数，不得被合并。
	advisories := []Advisory{
		{ID: "PYSEC-2020-148"},
		{ID: "PYSEC-2026-4177"},
	}
	deduped, merged := dedupeAdvisoriesByAlias(advisories)
	if len(deduped) != 2 || merged != 0 {
		t.Fatalf("deduped = %d merged = %d, want 2 and 0", len(deduped), merged)
	}
}

func TestDedupeAdvisoriesByAliasEmptyInput(t *testing.T) {
	t.Parallel()

	deduped, merged := dedupeAdvisoriesByAlias(nil)
	if len(deduped) != 0 || merged != 0 {
		t.Fatalf("deduped = %v merged = %d, want empty", deduped, merged)
	}
}
