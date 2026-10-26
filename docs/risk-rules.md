# DepScope Risk and Evidence Rules

This document describes the explainable rules implemented in Phase 2. The rules are deterministic, derivable from report data, and intentionally conservative. They are not a prediction of exploitation or maliciousness.

## 1. Scope of a scan

Phase 2 scans a single npm or PyPI package at an explicitly resolved version.

- The target package is resolved from registry metadata. An explicitly requested version must appear in the registry version list; the scanner does not guess or coerce versions.
- Declared direct dependencies are read and shown as unresolved direct nodes in the dependency graph.
- Direct dependencies are **not** recursively resolved or vulnerability-scanned in this phase.
- If a package declares no direct dependencies, the `dependency-graph` check succeeds and the root package is the complete scanned graph.
- If a package declares dependencies, the `dependency-graph` check is `skipped`, a `DEPENDENCY_RESOLUTION_UNAVAILABLE` limitation is added, and the report is `incomplete`. Such a report can never be `clean`.

This boundary prevents a partially scanned project from being presented as vulnerability-free.

## 2. Confidence, severity, and evidence quality

DepScope separates three concepts that must not be conflated:

| Concept | Representation | Meaning |
| --- | --- | --- |
| Confidence | `finding.kind` = `confirmed` or `heuristic` | Whether the signal is backed by matched advisory data or by a review heuristic |
| Severity | `finding.severity` | Source-reported or normalized impact; `unknown` is valid and is not safe |
| Evidence quality | `finding.evidence` entries | What the conclusion is based on, with a source ID and reference where applicable |

Rules:

- A `confirmed` finding requires a resolved version and at least one `advisory` evidence item with a reference. It comes from OSV data matched to the resolved package and version.
- A `heuristic` warning is a signal for review. It is never described as proof of malicious behavior.
- Unknown severity remains a finding. It does not become clean or low by default.
- Confirmed vulnerabilities are never suppressed, downgraded, or hidden because a package appears popular or well maintained. DepScope does not collect popularity data in this phase.
- Source and evidence limitations are reported in `checks` and `limitations`; they are not silently discarded.

## 3. Confirmed vulnerability findings

Source: OSV.dev `POST /v1/query` with the resolved version and the ecosystem mapping `npm` or `PyPI`.

The provider:

1. Sends only the package identity and version.
2. Keeps only advisory entries whose affected package matches the requested canonical package identity.
3. Records `OSV_AFFECTED_PACKAGE_MISMATCH` when a returned record cannot be matched.
4. Normalizes advisory IDs, aliases, references, affected ranges, fixed versions, and details.
5. Normalizes severity in this order:
   - advisory `database_specific.severity`
   - affected-entry `database_specific.severity`
   - affected-entry `ecosystem_specific.severity`
   - numeric CVSS score (`>= 9` critical, `>= 7` high, `>= 4` medium, `> 0` low)
   - otherwise `unknown` (a CVSS vector string alone is not converted)

### 3.1 Alias deduplication

OSV.dev indexes the same underlying vulnerability more than once: for PyPI it commonly returns both a `GHSA-…` record and a `PYSEC-…` record that list each other, and both list the same `CVE-…`, in `aliases`. A query for `pypi:urllib3@1.26.5` returns 20 records for 10 distinct vulnerabilities.

Before findings are produced, records are grouped by identifier connectivity: each record contributes its `id` and every alias, and all records reachable from one another form a single group.

- Comparison is case-insensitive and ignores surrounding whitespace.
- One group produces exactly one `confirmed` finding. The representative keeps the ID of the first record in the source order, and the identifiers of the merged records are added to its alias set so the evidence remains traceable.
- Distinct vulnerabilities never merge; only shared identifiers connect records.
- The number of merged records is reported in an `OSV_ALIAS_MERGED` limitation, so a reduced count is disclosed rather than silent.
- Merging is normalization, not a coverage gap. It never changes the `vulnerabilities` check status and never turns a complete scan into `incomplete`; the limitation is informational and is the only visible effect on the report.
- The rule applies to both the single-package scan and the batch path used by project scans and global inventory.

### 3.2 Result counts

A finding is a **package-to-vulnerability match**, not a distinct vulnerability. One advisory can match several installed packages, and one package can match several advisories. Reports never present a single ambiguous total:

- `summary.confirmedFindings` counts findings (matches), not distinct vulnerabilities.
- The global inventory report adds an `OVERVIEW` section with three separate numbers: installed packages, affected packages, and advisory matches.

Each matched advisory produces one `confirmed` finding:

- ID: `VULN-<advisory ID>`
- Title/description: advisory summary/details, with a neutral fallback when the source omits them
- Evidence: advisory evidence with a source ID and either the first source reference or the canonical OSV URL for that advisory
- Remediation: the listed fixed versions when present; otherwise a statement that no fixed version was listed
- Risk: the confirmed-finding rule in Section 5

If OSV fails, the `vulnerabilities` check becomes `unavailable`, a `VULNERABILITY_DATA_UNAVAILABLE` limitation is added, and the report is `incomplete`. It is never `clean`.

## 4. Heuristic signals

### 4.1 npm lifecycle scripts

The npm provider reads `preinstall`, `install`, and `postinstall` scripts from version metadata.

- Each non-empty declared script produces one `heuristic` finding with severity `low` and ID `LIFECYCLE-<SCRIPT>`.
- The command is included as evidence for inspection but is **never executed**.
- The description states explicitly that a lifecycle script is a signal for review, not proof of malicious behavior.
- The `lifecycle-scripts` check succeeds when all scripts were inspected, including when no scripts exist.
- If version metadata is unavailable, the check is `unavailable` and the report is `incomplete`.

The current rule does not score command contents. A future rule may add command-specific heuristics only with tests and documentation.

### 4.2 Typosquat similarity

The typosquat check compares the canonical package name with a versioned, embedded reference dataset using Damerau-Levenshtein distance.

- npm names are lowercased; PyPI names use PEP 503 normalization before comparison.
- Scoped npm package names are skipped because scope semantics differ from unscoped names.
- Names shorter than four characters are not compared.
- The match threshold is edit distance `1`, raised to `2` only for names of length eight or greater.
- The closest match is chosen deterministically, with lexical tie-breaking.
- A match produces one `heuristic` finding with severity `medium`, ID `TYPOSQUAT-<ecosystem>:<name>`, and evidence containing the reference name, dataset version, update time, and distance.
- An exact name match is not flagged.
- If the reference dataset has no entries for the ecosystem, the `typosquat` check is `unavailable`, a `TYPOSQUAT_REFERENCE_UNAVAILABLE` limitation is added, and no match is invented.

The embedded dataset at `internal/engine/reference/packages-v1.json` is a manually curated seed of recognizable package names. It is **not** a live top-N download list, is not ordered by popularity, and contains no download counts. Its provenance, version, and update instructions are stored in the file and validated at load time. To update it, verify candidate names against the official registry, edit the file, bump `datasetVersion` and `updatedAt`, and run `go test ./internal/engine/`.

### 4.3 Maintenance metadata

Publication time, repository URL, deprecation status, and yanked-release status are reported as context in the `registry-metadata` check message and, where relevant, as limitations.

They do **not** produce findings by themselves:

- A stale publication date is context, not malware evidence.
- A missing repository URL is context, not malware evidence.
- A yanked PyPI release is reported as a `YANKED_RELEASE` limitation but is not treated as malicious.
- Maintenance metadata never suppresses a confirmed vulnerability.

## 5. Risk scoring rules

Risk scores are **prioritization values from 0 to 100**. They are not probabilities of exploitation, compromise, or maliciousness. No validated probabilistic model is implemented.

| Finding kind | Critical | High | Medium | Low | Unknown |
| --- | ---: | ---: | ---: | ---: | ---: |
| Confirmed advisory | 95 | 80 | 55 | 35 | 45 |
| Heuristic signal | 50 | 45 | 35 | 15 | 25 |

Rules:

- The two bands are separate. A heuristic critical signal (50) never outranks a confirmed high finding (80).
- Every finding receives a rationale stating that the score is a prioritization aid, not a probability, and identifying the evidence basis.
- If the risk evaluator fails, findings are retained with `risk: null`; a failed `risk-evaluation` check and a `RISK_EVALUATION_UNAVAILABLE` limitation are added, and the report is `incomplete`.
- Risk scoring never removes or alters source-reported severity.

## 6. Provider failure and reliability rules

- Every external call uses the scan context plus a provider-specific timeout.
- Provider concurrency is bounded by the engine limiter.
- HTTP success responses may be cached with a bounded TTL and a bounded number of entries. Error responses are not cached. Cache hits preserve the original upstream retrieval timestamp in the report.
- Transient HTTP statuses (`429`, `5xx`) are retried a bounded number of times with bounded backoff; `Retry-After` is honored but capped at two seconds.
- Non-200 responses, malformed JSON, oversized bodies, timeouts, rate limits, and cancellation are classified as `not_found`, `unavailable`, `rate_limited`, `invalid_response`, or `timeout`.
- A registry failure that prevents version resolution produces `failed`.
- Ancillary provider failures produce `unavailable` checks and `incomplete`, while any confirmed findings already obtained are retained.
- Cancellation produces `cancelled` and returns the context error.
- Live failures never fall back to demo data or stale fabricated values.
- Package code is never downloaded or executed.

## 7. Determinism and ordering

- Findings are ordered confirmed-first, then by severity rank descending, then by finding ID.
- Checks are ordered by check ID; source IDs within a check are sorted.
- Sources are ordered by source ID.
- Limitations are deduplicated and ordered by code and message.
- The clock is injectable, so tests use fixed timestamps.

Given identical inputs and provider responses, report content is deterministic. Live source responses can change over time; that is a property of the sources, not the scoring rules.

## 8. Project scanning and inventory coverage

Phase 3 applies the same finding, evidence, and risk rules to project scans and installed inventory, with these coverage boundaries.

### 8.1 Project scans

- Findings are produced only for packages whose version is actually resolved from the project's supported input. The latest registry version is never substituted for a project version.
- npm lockfile v2/v3 nodes inherit version-specific OSV findings and lifecycle-script heuristics. Dependency-path evidence identifies how each package was introduced.
- Python `requirements.txt` supports exact `==`/`===` pins only. Range, wildcard, unpinned, URL, editable, and marker-dependent requirements are reported as unresolved or unsupported.
- Python transitive dependencies are unavailable in this phase. A Python project scan therefore cannot claim complete vulnerability coverage.
- Unresolved nodes, malformed input, unsupported formats, cycle and depth limits, and conflicting metadata produce non-succeeded checks or limitations and prevent a `clean` status.
- Project manifests and lockfiles are untrusted input: package names are validated, sizes are bounded, includes cannot escape the project root, and symlinked files are not followed.

### 8.2 Installed inventory

- Findings are produced only for installed packages whose identity and version are readable from installed metadata or a read-only package-manager listing.
- npm inventory covers top-level global packages. Nested dependencies and symlinked entries are not scanned.
- pip inventory covers the selected interpreter's default environment only. Other virtual environments and interpreters are not covered.
- Detection failures, metadata conflicts, unreadable or oversized metadata, and unresolved versions are surfaced as limitations; they never become an empty clean result.
- Inventory never installs, upgrades, imports, or executes a package, and never runs lifecycle scripts.
- Because coverage is partial by design, inventory results must state these limitations alongside any findings.

