# DepScope Data Contracts

## 1. Status and scope

Phase 1 established the Go module, the shared report model, the canonical JSON Schema, deterministic demo fixtures, and the interfaces that the engine, CLI/TUI, API, and website will share.

Phase 2 implements the real security engine behind those interfaces: live npm, PyPI, and OSV adapters; version selection; dependency inspection; lifecycle-script and typosquat heuristics; explainable risk rules; bounded caching and retries; and the scan orchestrator. The report schema remains version `1.0.0`; no wire fields were added.

Phase 3 adds two read-only engines on top of the scanner: project dependency scanning (`internal/project`) and global package inventory (`internal/inventory`). Both reuse the shared model types, provider batch scanning, and risk rules. Their exported result types are separate from `model.ScanReport`, so the v1 wire schema is unchanged; integrating them with the schema is an open decision (D16).

Phase 4 adds the terminal surface and the offline demo runtime. `internal/app` defines the `Service` contract shared by the CLI and TUI. `internal/render` converts shared results into plain text or limited ANSI output and sanitizes untrusted text. `internal/tui` implements the Bubble Tea interface, `internal/cli` and `cmd/depscope` implement commands and exit codes, and `internal/demo` implements deterministic offline scenarios through the shared engine. No wire fields were added; the schema remains `1.0.0`. Project and inventory results are still separate Go types, rendered through the shared `render.Document` model rather than the wire schema.

Phase 5 adds the HTTP transport (`internal/api`, `cmd/depscope-api`). It consumes `app.Service` and returns canonical `model.ScanReport` values validated through `internal/contract`; the report schema remains `1.0.0`. The API adds its own request contract, comparison envelope, stable error codes, and process-local limits. Project and inventory results are not exposed, and no report wire fields were added. The full HTTP contract is in `docs/api.md`.

Phases 1–5 do **not** implement recursive npm or Python dependency resolution outside a supported lockfile graph, machine-readable terminal JSON output, or the website. Scan scope and evidence rules are documented in `docs/risk-rules.md`.

The Go module path is provisionally `depscope`. It contains no remote host and must be replaced with the canonical module path before publication.

## 2. Contract source of truth

| Artifact | Location | Responsibility |
| --- | --- | --- |
| Canonical report schema | `schemas/scan-report.v1.schema.json` | JSON structure, enums, required fields, and cross-field status/demo rules |
| Go report types | `internal/model/report.go` | Typed Go representation of the same contract |
| Go semantic validation | `internal/model/validate.go` | Referential integrity, counts, graph consistency, and provider rules that JSON Schema cannot express |
| Schema-backed validation | `internal/contract/contract.go` | Embeds the schema and validates raw JSON or a Go report |
| TypeScript types | Future `web/` application | Must be generated from the schema or checked against it; not hand-copied |

- Schema ID: `urn:depscope:schema:scan-report:1.0.0`
- `schemaVersion`: `1.0.0`
- Format: JSON Schema 2020-12
- Field naming: camelCase
- Additional properties: rejected at every object level

Any contract change must update the schema, Go types, tests, and version decision together. With `additionalProperties: false`, field changes are breaking by default.

## 3. Report model

A `ScanReport` contains:

| Field | Purpose |
| --- | --- |
| `schemaVersion` | Contract version; currently `1.0.0` |
| `dataMode` | `live` or `demo` |
| `demoNotice` | Required non-empty notice for demo reports; null for live reports |
| `scannedAt` | RFC 3339 scan timestamp |
| `status` | Final scan status |
| `target` | Ecosystem, canonical package name, and resolved version when known |
| `summary` | Confirmed/heuristic counts and highest severity, with null when no findings exist |
| `findings` | Confirmed findings and heuristic warnings |
| `checks` | Per-check outcomes |
| `dependencies` | Root node and dependency nodes |
| `sources` | Data-source references with retrieval timestamps |
| `limitations` | Report-level coverage and availability limits |

Nullability is explicit. `target.resolvedVersion`, `finding.version`, `finding.risk`, and `summary.highestSeverity` are present as `null` when unknown. Arrays are always present and may be empty; JSON `null` is not a valid collection.

### Package identity

`PackageIdentity` contains:

- `ecosystem`: `npm` or `pypi`.
- `name`: canonical name.

Canonicalization rules:

- npm names are lowercased. Scoped names must have exactly one slash and a non-empty scope and package name.
- PyPI names are lowercased and normalized with the PEP 503 rule: runs of `-`, `_`, and `.` become a single hyphen.

Constructors normalize names, while `Validate` rejects non-canonical values that arrive through JSON.

### Findings and evidence

Findings have `kind: confirmed | heuristic`, a severity, package identity, optional version, at least one evidence item, and optional risk assessment.

- A confirmed finding requires a resolved version and at least one `advisory` evidence item with a reference.
- A heuristic warning may have an unknown severity and may omit the version when it cannot be resolved.
- Unknown severity is a valid value. It does not mean safe, and any finding still prevents a clean status.
- Evidence must reference a source ID declared in `sources`.

### Dependency graph

`DependencyGraph` contains one root node and zero or more direct/transitive nodes:

- Every node has a stable ID, package identity, nullable version, relationship, path, and outgoing dependency IDs.
- Every path starts with the root node ID and ends with the node's own ID.
- Each consecutive path edge must exist in the parent node's `dependencies`.
- The root package and version must match `target`.
- Findings that specify a version must match a node in the graph.
- An unresolved node uses `version: null` and must not be treated as resolved.

## 4. Status semantics

Final statuses in a report: `clean`, `findings`, `incomplete`, `failed`, `cancelled`.

`pending` and `running` exist for orchestration but are not valid in a completed report.

Derivation for a normal completed scan:

| Condition | Derived status |
| --- | --- |
| No checks | `incomplete` |
| Any check is `unavailable`, `skipped`, or `failed` | `incomplete` |
| All checks `succeeded`, at least one finding | `findings` |
| All checks `succeeded`, no findings | `clean` |

`failed` and `cancelled` are set explicitly by orchestration. A `failed` report requires either a failed check or a limitation.

**Invariant:** a missing or unsuccessful result can never become `clean`. The JSON Schema enforces this by allowing `clean` only with at least one check, no findings, at least one source, and no non-succeeded checks. `incomplete` requires either no checks or at least one non-succeeded check.

### Check outcomes

| Status | Meaning |
| --- | --- |
| `succeeded` | The check completed over its intended scope |
| `unavailable` | A required source or capability was not available |
| `skipped` | The check was deliberately not performed |
| `failed` | The check ran but did not complete successfully |

Every non-succeeded outcome produces an `incomplete` report, even when other findings exist.

## 5. Demo-data rules

All synthetic fixtures live under `testdata/fixtures/demo/`.

- `dataMode` must be `demo`.
- `demoNotice` must contain an unmistakable `DEMO DATA` label.
- Every source must use `provider: demo`.
- Package names must be clearly synthetic and contain `depscope-demo`.
- Identifiers use `DEMO-` prefixes; real advisory IDs are never invented.
- URLs are omitted rather than synthesized.
- Timestamps are fixed, and output is deterministic.

The schema rejects demo reports without a notice and demo reports that reference a real provider. Go validation also rejects live reports that reference demo providers. Live failures must never fall back to demo data.

### Demo runtime

`internal/demo` provides the runtime activation for the Phase 4 offline demo. It does not read recorded live responses and does not make network calls.

| Scenario | Package | Demonstrates |
| --- | --- | --- |
| `vulnerable` | `@depscope-demo/vulnerable-package@2.0.0` | Confirmed critical and medium advisories with fixed versions and risk scores |
| `install-scripts` | `@depscope-demo/scripted-package@1.2.0` | `preinstall`/`postinstall` declarations and a deprecation signal; commands are never executed |
| `clean` | `@depscope-demo/clean-package@1.0.0` | No advisories, no install scripts, and complete successful checks |

Every scenario uses a fixed clock (`2026-01-01T00:00:00Z`), the shared `engine.Scanner`, the shared risk evaluator, demo-only source references, and the shared demo notice. The scenario list and data are code-defined for determinism.

Demo reports are activated by `depscope demo [scenario]`, `depscope demo list`, or the TUI demo picker. The scanner's data mode is fixed at construction: the live scanner rejects `dataMode: demo`, and the demo scanner rejects live requests. There is no fallback from a failed live scan to demo data.

## 6. Engine interfaces

The interfaces live in `internal/engine`. Phase 2 provides live implementations for npm, PyPI, and OSV, plus the scan orchestrator in `internal/engine/scanner.go`.

### `ScanOrchestrator`

```go
Scan(ctx context.Context, request ScanRequest) (model.ScanReport, error)
```

The orchestrator coordinates resolution, vulnerability lookup, risk evaluation, and report assembly. It must honor context cancellation and must never return a clean report when a required check is missing or unsuccessful.

### `RegistryMetadataProvider`

```go
Ecosystem() model.Ecosystem
PackageMetadata(ctx context.Context, pkg model.PackageIdentity) (RegistryMetadata, error)
Dependencies(ctx context.Context, pkg model.PackageIdentity, version string) ([]DependencyRequirement, error)
```

A provider is bound to one ecosystem. It returns registry metadata, available versions, dependency requirements, source references, and limitations.

Version metadata is an optional capability:

```go
type VersionMetadataProvider interface {
    VersionMetadata(ctx context.Context, pkg model.PackageIdentity, version string) (VersionMetadata, error)
}
```

`VersionMetadata` carries the selected version's publication time, repository URL, deprecation message, lifecycle scripts, declared dependencies, source, and limitations. These details are internal to the engine; they do not add wire fields. The orchestrator uses them for dependency inspection, maintenance context, and npm lifecycle-script warnings.

### `VulnerabilityProvider`

```go
Query(ctx context.Context, pkg model.PackageIdentity, version string) (VulnerabilityResult, error)
```

The provider returns advisories already matched to the requested version. It must not fabricate or fill in missing vulnerability data.

Advisory data includes the advisory ID, aliases, summary, details, normalized severity, references, fixed versions, and affected ranges. The OSV adapter maps `npm` and `pypi` to the OSV ecosystem values `npm` and `PyPI` respectively.

### `RiskEvaluator`

```go
Evaluate(ctx context.Context, input RiskInput) (model.RiskAssessment, error)
```

Phase 2 implements `RulesRiskEvaluator`, which returns a deterministic 0-100 prioritization score with a rationale. The score is not a probability of exploitation or maliciousness. Confirmed and heuristic findings use separate score bands. Full rules are in `docs/risk-rules.md`.

### `Cache`

```go
Get(ctx context.Context, key string) ([]byte, bool, error)
Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
```

`NoopCache` disables caching. Cache implementations must honor context cancellation and document expiry semantics. The Phase 2 HTTP client uses cache keys containing the HTTP method, full request URL, and a SHA-256 hash of the request body, namespaced under `depscope:fetch:v1`. Successful responses are cached; error responses are not.

### Provider failures

`ProviderError` classifies failures as `not_found`, `unavailable`, `rate_limited`, `invalid_response`, or `timeout`. Orchestration maps them to non-succeeded check outcomes; it never treats them as clean.

| Failure | Orchestration result |
| --- | --- |
| Registry metadata or version resolution failure | `registry-metadata` check `failed`; report `failed` |
| Version metadata failure | `dependency-metadata` and `dependency-graph` checks `unavailable`; OSV query still runs; report `incomplete` |
| OSV failure | `vulnerabilities` check `unavailable`; `VULNERABILITY_DATA_UNAVAILABLE` limitation; report `incomplete` |
| Typosquat reference unavailable | `typosquat` check `unavailable`; report `incomplete` |
| Risk evaluator failure | Findings retained with `risk: null`; `risk-evaluation` check `failed`; report `incomplete` |
| Cancellation or scan timeout | report `cancelled`; context error returned |

## 6.1 Scan orchestration

`internal/engine/scanner.go` implements `ScanOrchestrator`:

1. Validate the request and require `dataMode: live`; demo mode is rejected by the live engine.
2. Apply the scan timeout and create a root dependency node.
3. Read package metadata and resolve either the requested version or the registry latest version. A requested version must appear in the registry version list.
4. Read version metadata when the provider supports it.
5. Add registry, dependency, lifecycle-script, typosquat, vulnerability, and risk checks.
6. Sort findings, checks, sources, and limitations deterministically, then compute the summary and final status.

Checks emitted by Phase 2 are `registry-metadata`, `dependency-metadata`, `dependency-graph`, `lifecycle-scripts` (npm only), `typosquat`, `vulnerabilities`, and `risk-evaluation` (only on evaluator failure). Declared direct dependencies are added as unresolved direct nodes; the dependency graph check is `skipped` whenever dependencies exist, which forces `incomplete`.

## 6.2 Project scan contract

`internal/project` scans a project directory with:

```go
Scan(ctx context.Context, root string, opts Options) (Result, error)
```

`Result` is not a `model.ScanReport` and is not validated by the v1 schema. It reuses `model.Finding`, `model.Evidence`, `model.CheckResult`, `model.SourceReference`, `model.Limitation`, and `model.Summary`, and adds:

| Field | Purpose |
| --- | --- |
| `Root` | Absolute project root that was scanned |
| `Formats` | Each detected format with resolved and unresolved package counts |
| `Dependencies` | Synthetic root ID `project`, shared dependency nodes, and per-node declared constraints |

`DependencyNode` values keep the shared shape: stable ID, package identity, nullable version, relationship, path, outgoing edges. Findings carry dependency-path evidence from the project root to the flagged package.

### Detection and supported formats

`Detect` inspects only fixed filenames in the project root. It does not recurse into `node_modules`, vendor directories, build outputs, or arbitrary paths, and it does not follow symlinks.

| Format constant | Input | Resolution |
| --- | --- | --- |
| `npm-package-lock-v2-v3` | `package.json` with `package-lock.json` whose `lockfileVersion` is 2 or 3 | Full graph from the lockfile `packages` map, including nested `node_modules` paths and dependency paths |
| `python-requirements-txt` | `requirements.txt` with exact `==` or `===` pins | Direct dependencies only; transitives remain unavailable |

Known unsupported files are recorded rather than parsed: `yarn.lock`, `pnpm-lock.yaml`, `npm-shrinkwrap.json`, `bun.lockb`, `bun.lock`, `poetry.lock`, `Pipfile`, `Pipfile.lock`, `pyproject.toml`, `setup.py`, `setup.cfg`, `uv.lock`, and `pdm.lock`. npm lockfile version 1 is unsupported.

A missing lockfile produces `LOCKFILE_MISSING`, unresolved direct nodes, and an `incomplete` result. The registry latest version is never substituted for a project-pinned version.

### Project checks and limitations

| Check | Meaning |
| --- | --- |
| `project:detection` | A supported manifest was detected in the root |
| `npm:manifest` | `package.json` was readable and structurally valid |
| `npm:lockfile` | The lockfile was readable and its version and root entry were usable |
| `npm:dependency-resolution` | Every lockfile node and direct requirement was resolved |
| `python:manifest` | `requirements.txt` and its includes were fully parsed |
| `python:dependency-resolution` | Every Python requirement was resolved |

Representative limitation codes include `NO_SUPPORTED_MANIFEST`, `UNSUPPORTED_FORMAT`, `MANIFEST_MISSING`, `MALFORMED_MANIFEST`, `MALFORMED_LOCKFILE`, `UNSUPPORTED_LOCKFILE_VERSION`, `LOCKFILE_MISSING`, `DIRECT_DEPENDENCY_UNRESOLVED`, `TRANSITIVE_DEPENDENCY_UNRESOLVED`, `PYTHON_TRANSITIVE_UNAVAILABLE`, `ENVIRONMENT_MARKER_UNSUPPORTED`, `UNSUPPORTED_REQUIREMENT_SOURCE`, `EDITABLE_UNSUPPORTED`, `WORKSPACES_NOT_SCANNED`, `PEER_DEPENDENCIES_NOT_SCANNED`, `CONFLICTING_PROJECT_METADATA`, `CONFLICTING_DEPENDENCY_METADATA`, `CYCLIC_DEPENDENCY`, `DEPENDENCY_DEPTH_EXCEEDED`, `NODE_LIMIT_EXCEEDED`, `FILE_LIMIT_EXCEEDED`, and the include-safety codes `INCLUDE_CYCLE`, `INCLUDE_LIMIT_EXCEEDED`, `INCLUDE_OUTSIDE_ROOT`, and `INCLUDE_UNREADABLE`.

Every unresolved version is reported explicitly and prevents a clean result.

## 6.3 Global inventory contract

`internal/inventory` exposes:

```go
Scan(ctx context.Context, opts Options) (Result, error)
```

`Result` is not a `model.ScanReport`. It contains the scan timestamp, status, installed packages, findings, checks, sources, limitations, and summary.

`InstalledPackage` records the ecosystem, canonical package identity, installed version, detecting manager (`npm` or `pip`), and the metadata source path. Installed packages are matched against the shared engine through the batch API.

### Listing mechanisms

| Manager | Mechanism | Coverage |
| --- | --- | --- |
| npm | `npm root -g`, then read each top-level installed `package.json` | Top-level global packages only; nested trees are not walked; symlinked entries are skipped |
| pip | `python3 -m pip list --format=json --disable-pip-version-check`, falling back to `python` if the first interpreter cannot be invoked | The selected interpreter's default environment only |

Inventory never installs, upgrades, imports, or executes a package, and never runs lifecycle scripts.

### Inventory checks and limitations

Checks are `npm:global-detection`, `python:global-detection`, `npm:vulnerabilities`, `python:vulnerabilities`, and `npm:risk-evaluation`/`python:risk-evaluation` when advisories exist.

Limitations explain detection and coverage failures instead of returning an empty success: `NPM_NOT_CONFIGURED`, `PYTHON_NOT_CONFIGURED`, `NPM_GLOBAL_DETECTION_FAILED`, `PYTHON_GLOBAL_DETECTION_FAILED`, `NPM_GLOBAL_TOP_LEVEL_ONLY`, `PYTHON_SINGLE_ENVIRONMENT`, `NO_INSTALLED_PACKAGES`, `PACKAGE_METADATA_MISSING`, `PACKAGE_METADATA_UNREADABLE`, `PACKAGE_METADATA_TOO_LARGE`, `MALFORMED_PACKAGE_METADATA`, `INVALID_PACKAGE_METADATA`, `CONFLICTING_PACKAGE_METADATA`, `UNRESOLVED_VERSION`, `SYMLINKED_PACKAGE_SKIPPED`, and `PACKAGE_LIMIT_EXCEEDED`.

A manager that cannot be configured makes the result `incomplete`. If every configured manager fails detection, the result is `failed`.

### Phase 3 local limits

| Setting | Default |
| --- | --- |
| Project files | 64 |
| Project file size | 4 MiB |
| Project graph nodes | 5,000 |
| Dependency depth | 32 |
| Included requirements files | 16 |
| Inventory packages | 10,000 |
| Inventory metadata file size | 4 MiB |
| Package-manager command timeout | 15 seconds |
| Package-manager output limit | 8 MiB |

Project cancellation returns `cancelled` with the context error. Inventory cancellation likewise returns `cancelled`.

## 6.4 Application service contract

`internal/app` is the single scanning boundary for the terminal surfaces. The CLI and TUI must not construct scanners or read source packages directly.

```go
type Service interface {
    ScanPackage(ctx context.Context, request PackageRequest, progress func(Progress)) (model.ScanReport, error)
    ScanProject(ctx context.Context, request ProjectRequest, progress func(Progress)) (project.Result, error)
    ScanInventory(ctx context.Context, progress func(Progress)) (inventory.Result, error)
    Compare(ctx context.Context, request CompareRequest, progress func(Progress)) (Comparison, error)
    Demo(ctx context.Context, scenario string, progress func(Progress)) (model.ScanReport, error)
    DemoScenarios() []DemoScenario
}
```

- `PackageRequest` holds a normalized ecosystem, canonical package name, and optional version. `ParsePackageSpec` accepts `lodash@4.17.21`, `@scope/pkg`, `pypi:requests==2.31.0`, and explicit `npm:`/`pypi:` prefixes.
- `ProjectRequest` holds a project path. `CompareRequest` holds two package requests.
- `Progress` reports a stage, message, and optional completed/total counts. It represents real work only; no synthetic percentages or delays are allowed.
- `Comparison` keeps the left and right reports and errors independently, so one unavailable side does not erase the other.
- `DemoScenarios` returns the fixed offline scenario list.
- Cancellation flows through the caller's context. Implementations must stop work and return the context error; they must not return partial results as success.
- `NewLiveService` constructs wiring only. It does not scan, access the network, or execute package-manager commands.

## 6.5 Terminal output contract

### Commands and exit codes

```text
depscope                          Open the interactive terminal app
depscope scan <package>           Scan one npm or PyPI package
depscope project [path]           Scan a project directory (default: .)
depscope compare <left> <right>   Compare two packages side by side
depscope inventory                List global npm and Python packages
depscope demo [scenario]          Run an offline deterministic demo
depscope demo list                List demo scenarios
depscope help                     Show usage
depscope version                  Show the version
```

Global flags before the command: `--no-color`, `--color=auto|always|never`, `--help`/`-h`, `--version`/`-V`, and `--` to end flag parsing.

| Exit code | Meaning |
| --- | --- |
| `0` | Command completed; report status was not `failed` or `cancelled` |
| `1` | Operational failure, including a `failed` or `cancelled` scan, both comparison sides unavailable, or a non-interruption app error |
| `2` | Usage error: unknown command or flag, missing argument, or unknown demo scenario |
| `130` | Interactive app interrupted by `ctrl+c` or context cancellation |

A report with status `incomplete` still exits `0`; the incomplete status and coverage gaps are shown in the output. `compare` exits `1` only when both sides are unavailable.

### Terminal detection and color

- With no arguments, the interactive app starts only when both stdin and stdout are terminals and `TERM` is not `dumb`. Otherwise help is printed and the process exits `0`.
- `NO_COLOR` with any non-empty value and `TERM=dumb` disable color regardless of `--color`.
- Color-disabled output contains no ANSI escape sequences. Severity, status, finding kind, check outcome, and demo mode are always explicit text labels.
- Narrow output remains readable: comparison switches to stacked per-side layout below 76 content columns. The interactive app shows a plain resize message below 46 columns by 14 rows.

### Report content and navigation

Every rendered report includes package identity and resolved version when known, overall status, check coverage, confirmed and heuristic findings with severity and evidence, source references, lifecycle-script warnings, dependency information where applicable, limitations, and next actions. The compact summary is a short copyable block that includes the target, counts, highest severity, sources, and the demo notice when applicable.

The interactive report has separate views for overview, finding detail, checks, dependency tree, package details, sources and limitations, and summary. Finding detail includes evidence and remediation. The comparison view shows aligned fields, independent source coverage, versions, findings, and data quality for each side.

### Untrusted text

`render.Sanitize` removes control characters, CSI sequences, and OSC sequences while preserving newlines and tabs where allowed. `render.SanitizeMultiline` does the same for error blocks. Documents are sanitized on copies, so the original report is not mutated. All remote text and error strings pass through this boundary before terminal display.

## 6.6 HTTP API contract

The API is transport only. It calls `app.Service` for package scans and comparisons and does not parse project trees, inventory global packages, or recompute risk. The full public reference, including examples, is `docs/api.md`.

### Endpoints and methods

| Method | Path | Body | Success |
| --- | --- | --- | --- |
| `GET` | `/healthz` | none | `200`, `{"status":"ok"}` only |
| `POST` | `/api/v1/scan` | one package object | `200`, canonical `ScanReport` |
| `POST` | `/api/v1/compare` | `left` and `right` package objects | `200`, `{left, right}` comparison envelope |

Package object fields are `ecosystem` (`npm` or `pypi`, required), `name` (required), and `version` (optional exact release). Unknown fields, null strings, query parameters, non-JSON media types, multiple JSON values, and bodies over the configured limit are rejected. Names and versions are bounded and restricted before any provider request is built; URL-shaped values are invalid.

### Responses

- A scan response is the report object itself. It is validated with `contract.ValidateReport` and must be `dataMode: live`, match the requested package identity, and match the requested version when one was supplied. Reports that fail these checks are never served.
- A compare response contains `left` and `right` objects. Each has `request`, and either `report` (a canonical report) or a generic `error`. One failed side does not erase the other. Both sides failing returns `502`.
- Embedded report statuses such as `findings`, `incomplete`, and `failed` are returned with HTTP `200`; they are scan outcomes, not transport errors.
- Every response sets `Cache-Control: no-store` and `X-Content-Type-Options: nosniff`. The API does not cache complete reports, so an unavailable check is never replayed as a success.
- Error responses use `{"error":{"code":"...","message":"..."}}` with stable codes. Internal error text, environment values, and secrets are never returned.

### HTTP status mapping

| Status | Meaning |
| --- | --- |
| `400` | Malformed JSON, unknown field, invalid field, or unsupported query parameter |
| `403` | Disallowed CORS preflight origin |
| `404` | Unknown path |
| `405` | Wrong method; `Allow` is set |
| `413` | Body limit exceeded |
| `415` | Content type is not `application/json` |
| `429` | Per-client process-local rate limit; `Retry-After` is set |
| `499` | Request context cancelled before completion |
| `500` | Service error, panic, or invalid report |
| `502` | Both compare sides failed |
| `503` | Concurrency capacity reached; `Retry-After` is set |
| `504` | Request deadline exceeded |

### Process-local protections

Each request gets a context deadline. Scan and compare work is capped by a non-blocking concurrency gate. A per-client token bucket keyed by TCP peer address limits bursts; forwarding headers are not trusted. CORS is disabled unless exact `http(s)` origins are configured; wildcards are rejected at startup. Panics are recovered and logged without detail in the response. The binary shuts down gracefully on `SIGTERM` or interrupt within the configured shutdown timeout.

These limits and counters are per process. There is no authentication, persistence, telemetry, shared rate-limit state, readiness probe against upstream sources, project/inventory transport, or demo endpoint. Authentication and deployment topology remain open (D10). Project and inventory serialization remains open (D16).

## 7. Runtime limits

`engine.Config` provides provisional defaults:

| Setting | Default |
| --- | --- |
| `ScanTimeout` | 2 minutes |
| `ProviderTimeout` | 15 seconds |
| `MaxConcurrency` | 8 |
| `CacheEnabled` | true |
| `CacheTTL` | 10 minutes |

`Config.Validate` rejects non-positive timeouts and concurrency, negative TTLs, and an enabled cache with a zero TTL. `ScanContext` and `ProviderContext` apply deadlines to parent contexts.

`engine.Limiter` is a bounded semaphore. `Acquire` waits for a slot or returns the context error; a pre-cancelled context returns immediately. `Release` returns a slot. Callers must pair every successful acquisition with exactly one release.

Phase 2 live policy:

| Policy | Implemented value |
| --- | --- |
| HTTP timeout | 30 seconds on the default HTTP client |
| Provider call timeout | `Config.ProviderTimeout`, default 15 seconds |
| Scan timeout | `Config.ScanTimeout`, default 2 minutes |
| Response body limit | 8 MiB |
| Retries | Transient `429` and `5xx` only; live factory defaults to 2 retries with 250 ms backoff |
| `Retry-After` | Honored, capped at 2 seconds |
| Cache | Bounded in-memory TTL cache; default 512 entries and 10-minute TTL |
| Client identification | `User-Agent: DepScope/0.1` |
| Cancellation | All HTTP, cache, and limiter operations honor context cancellation |

No error response is cached, no retry is unbounded, and no package code is downloaded or executed.

Phase 5 API defaults:

| Setting | Default |
| --- | --- |
| Listen address | `127.0.0.1:8080` |
| Request timeout | 90 seconds |
| Request body limit | 16 KiB |
| Max concurrent scans/comparisons | 8 |
| Per-client rate | 60 requests/minute, burst 10 |
| CORS origins | none (disabled) |
| Graceful shutdown wait | 15 seconds |
| Server read-header / read / write / idle | 5 s / 15 s / request timeout + 15 s / 60 s |

The API request deadline cancels the engine scan context. The engine retains its own 15-second provider timeout and 2-minute scan timeout. Limits and counters are process-local. Configuration variables and constraints are listed in `docs/api.md`.

## 8. Verification

Run from the repository root:

```sh
gofmt -l .
go build -buildvcs=false ./...
go vet ./...
go test -count=1 ./...
```

`-buildvcs=false` is required in the current environment because a broken empty `.git` directory exists in the parent home directory, causing Go VCS stamping to fail. This project directory is not a Git repository. On a machine without that condition, plain `go build ./...` should be used.

`go test ./internal/contract/` performs schema validation of all demo fixtures and checks the status, demo-source, missing-evidence, unknown-severity, and incomplete-result rules. Provider tests use sanitized `httptest` fixtures. `internal/engine/integration_test.go` exercises the full npm/PyPI/OSV provider-to-report path without public network access. Project tests read synthetic trees under `testdata/fixtures/projects/`. Inventory tests use synthetic metadata under `testdata/fixtures/inventory/` and injected command runners, so no installed global package is modified. Phase 4 tests cover the application service, CLI commands and exit codes, TUI state transitions and cancellation, report and comparison rendering, sanitization, color policy, offline demo determinism, and non-TTY behavior. Phase 5 tests use an in-memory fake service to cover canonical scan responses, report validation, malformed and oversized payloads, unknown fields, timeouts, cancellation, partial reports, compare side failures, CORS, rate limits, concurrency limits, panic containment, and configuration parsing. They do not require a live terminal, network, or external process. The live smoke test is isolated behind the `live` build tag and `DEPSCOPE_LIVE_TESTS=1`; it was not run during Phase 5 verification. No frontend exists, so there is no frontend type check yet.

## 9. Remaining decisions

Phase 2 narrows the following Phase 1 decisions:

- Scan semantics (D8): the single-package scanner still covers only the target package. Phase 3 adds project scanning for npm lockfile v2/v3 graphs and exact-pinned `requirements.txt`; Python transitive resolution, other lockfile formats, workspace and peer dependency coverage, and installed-environment scanning remain open.
- Risk scoring (D9): deterministic Phase 2 rules are documented in `docs/risk-rules.md`. A validated probabilistic model is explicitly not claimed and remains out of scope.
- Caching, timeout, retry, and partial-failure policy (D12): implemented for Phase 2 and documented above. Source-specific retry limits and cache sizes may still be tuned after live verification.

Phase 3 adds:

- D16: how project and inventory results integrate with the v1 wire schema. Both currently expose separate Go result types built from shared model types, and the v1 `ScanReport` still requires exactly one target package. A schema version for project/inventory reports, a transport envelope, or an adapter remains to be decided before the API or website consumes them.

Phase 4 narrows:

- D11: the offline demo is now implemented in `internal/demo` with three deterministic scenarios, activated by the CLI and TUI, and isolated from live mode. Website demo presentation remains future work.
- D14: the CLI is human-readable text only. A machine-readable JSON mode and any command-envelope schema remain open.
- D15: terminal dependency-tree and package-comparison presentation are implemented. Wire representation and website comparison remain unspecified.

Phase 5 narrows:

- D10 (API endpoints, CORS, limits): `GET /healthz`, `POST /api/v1/scan`, and `POST /api/v1/compare` are fixed, with strict request validation, body and concurrency limits, request deadlines, per-client process-local rate limiting, and an exact-origin CORS allowlist. Authentication, authorization, trusted-proxy handling, shared multi-instance enforcement, and deployment remain open.
- D16 remains open: project and inventory results are deliberately not exposed through the API. Website integration must not expose them as ad hoc JSON.
- The report schema remains `1.0.0`; the API comparison envelope composes two canonical reports and is not a report schema change.

Still open:

- Canonical Go module path and repository URL.
- License and Git initialization.
- Live source terms, rate limits, and connectivity verification.
- Recursive Python resolution and additional npm/Python lockfile formats.
- API authentication, trusted-proxy handling, shared multi-instance limits, and deployment.
- Website implementation, JavaScript package manager/toolchain, and TypeScript type generation.
- Machine-readable CLI output.
- Project and inventory result serialization.
- Dependency graph and comparison wire contracts beyond the Phase 3 project graph and terminal/API package comparison.

See `docs/architecture.md` Section 11.
