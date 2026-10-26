# DepScope

DepScope is an open-source dependency security tool for npm and PyPI projects. It is intended to resolve project dependencies, correlate them with vulnerability data from OSV.dev, and present consistent, explainable risk results in both a terminal interface and a website.

> **Status:** Phase 5 (production-oriented Go HTTP API) is complete. The repository contains the shared Go engine, canonical report schema, live npm/PyPI/OSV adapters, project scanning and global inventory, a report renderer, a shared application service, an interactive Bubble Tea terminal app, a non-interactive CLI, a deterministic offline demo, and an HTTP API over the same service. The Next.js website is not implemented yet.

## Approved architecture

DepScope is planned as a single repository with the following components:

- **Shared security engine (Go):** dependency resolution, vulnerability lookup and matching, risk assessment, and report construction.
- **CLI and TUI (Go + Bubble Tea):** command-line scanning and an interactive terminal interface that render engine results.
- **HTTP API (Go):** a transport layer over the same engine, serving reports to the website.
- **Website (Next.js + TypeScript + Tailwind CSS):** a web interface that consumes the shared report format.
- **Shared report schema:** a single contract used by the terminal and the website so both surfaces present consistent results.
- **Offline demo:** deterministic, clearly labelled demonstration data that does not depend on live network access.

## Initial scope

- Supported ecosystems: **npm** and **PyPI**.
- Initial security data sources: **OSV.dev**, the **npm Registry**, and **PyPI**.
- Initial surfaces: **terminal (CLI/TUI)**, **HTTP API**, and **website**.
- Additional capabilities referenced for later phases, including a dependency graph and comparison features, are not yet specified in detail.

## Implemented in Phase 1

- Go module with the provisional local module path `depscope`.
- Canonical package identity and ecosystem normalization.
- Scan status, severity, finding, evidence, check result, dependency node, source reference, limitation, and complete scan report models.
- Final-status derivation that never treats missing or unsuccessful checks as clean.
- Cross-field semantic validation and schema-backed validation.
- Versioned JSON Schema 2020-12 contract at `schemas/scan-report.v1.schema.json`.
- Interfaces for registry metadata, vulnerability lookup, risk evaluation, caching, and scan orchestration.
- Configurable timeouts, bounded concurrency, and context cancellation primitives.
- Deterministic, unmistakably labelled demo fixtures under `testdata/fixtures/demo/`.
- Contract, model, configuration, and limiter tests.

No live network calls are made by automated tests. Real vulnerability data, source references, and registry metadata are available through the live engine path.

## Implemented in Phase 2

- Live npm Registry provider: package metadata, version-specific metadata, declared dependencies, deprecation, repository normalization, and `preinstall`/`install`/`postinstall` inspection. Commands are never executed.
- Live PyPI provider: project and release metadata, requested/latest version handling, PEP 503 normalization, `requires_dist` parsing, upload times, repository metadata, and yanked-release limitations.
- Live OSV.dev provider: version-specific queries with correct `npm`/`PyPI` ecosystem mapping, advisory IDs, aliases, affected ranges, fixed versions, references, and severity normalization.
- Scan orchestrator that resolves a version, assembles checks, findings, dependency nodes, sources, and limitations, and derives final status.
- Explainable risk rules that separate confirmed advisory findings from heuristic signals and never claim a probability of maliciousness.
- Typosquat detection using canonical names, Damerau-Levenshtein distance, and a versioned embedded reference dataset with provenance and update instructions.
- Bounded in-memory TTL cache, shared HTTP client, provider and scan timeouts, bounded retries for `429`/`5xx`, body limits, and failure classification.
- Live scanner factory wiring npm, PyPI, OSV, cache, limiter, and risk rules.
- Provider tests with sanitized `httptest` fixtures and full-path engine integration tests.
- Opt-in, build-tagged live smoke test separated from the default offline suite.

Scan scope is intentionally narrow: the target package is resolved and scanned; declared direct dependencies are inspected but not recursively resolved. A package with declared dependencies therefore produces an `incomplete` report rather than a misleading `clean` result. See [docs/risk-rules.md](docs/risk-rules.md).

## Implemented in Phase 3

Phase 3 adds two engines built on top of the shared scanner. Both are read-only, both reuse the shared engine, model types, and risk rules, and neither executes project code, package code, lifecycle scripts, or package-manager installation commands.

### Project dependency scanning (`internal/project`)

Scans the current project directory and reports confirmed and heuristic findings against the versions the project actually uses, with the dependency path from a direct dependency to each flagged transitive package.

Supported formats are limited and reported precisely:

| Ecosystem | Supported input | Behavior |
| --- | --- | --- |
| npm | `package.json` plus `package-lock.json` with `lockfileVersion` 2 or 3 | Full graph from the lockfile `packages` map, including nested `node_modules` paths, direct/transitive relationships, and dependency paths. |
| npm | `package.json` without a usable lockfile | Direct dependencies are reported as unresolved. The registry latest version is never substituted. The scan reports `incomplete`. |
| Python | `requirements.txt` with exact `==` or `===` pins | Direct dependencies only. Environment markers, options, URLs, and unpinned or ranged requirements are reported as unsupported or unresolved. |

Explicitly unsupported for v1:

- npm lockfile `lockfileVersion` 1, `yarn.lock`, `pnpm-lock.yaml`, `npm-shrinkwrap.json`, `bun.lockb`, and `bun.lock`.
- Python lockfiles and manifests: `poetry.lock`, `Pipfile`, `Pipfile.lock`, `uv.lock`, `pdm.lock`, `pyproject.toml`, `setup.py`, and `setup.cfg`.
- Python transitive dependency resolution. Python project scans therefore always report incomplete coverage.
- npm workspaces and peer dependencies. Workspace links are not followed or scanned.
- Vendor directories, `node_modules` contents, build output, and arbitrary filesystem paths outside the fixed project-root manifests.

Missing lockfiles, malformed lockfiles or manifests, unsupported syntax, unresolved version ranges, cycle and depth limits, and conflicting metadata are reported as limitations or non-succeeded checks; they never produce a `clean` result.

### Global package inventory (`internal/inventory`)

Read-only inventory of installed packages. It uses documented package-manager listing mechanisms and installed metadata only:

- npm: `npm root -g` for the global root, then top-level installation metadata (`package.json`) under that root. The nested tree is not scanned and symlinked entries are skipped.
- Python: `python3 -m pip list --format=json --disable-pip-version-check`, falling back to `python` only if the first interpreter cannot be invoked.

Inventory results match installed package identities and versions against the shared security engine. Detection failures are explained as unavailable checks and limitations rather than being reported as an empty success. Virtual environments other than the selected interpreter's environment, alternate npm prefixes, and nested npm trees are not covered.

## Implemented in Phase 4

Phase 4 exposes the existing engine through a terminal surface. It does not add scanning logic: project parsing, dependency resolution, vulnerability matching, risk rules, and report assembly remain in the shared engine.

### Interactive terminal app (`internal/tui`)

Running `depscope` with no arguments in an interactive terminal opens a Bubble Tea menu:

- Check a single package
- Scan my project
- Compare two packages
- Check my global installs
- Run the offline demo
- Settings

Every menu item maps to the same application service used by the CLI. Scans show real progress reported by the engine, not fabricated percentages. `esc` on the working screen cancels the in-flight scan; results and progress messages from a cancelled or superseded scan are discarded. The interface includes a keyboard help screen, a confirmation step before reading the machine's global packages, and a plain-text guard for terminals smaller than 46 columns by 14 rows.

Each report opens on an overview and exposes separate views for finding detail, checks, dependency tree, package details, sources and limitations, and a compact copyable summary. Findings show evidence, source references, remediation text, and next actions. Severity and finding kind are always written as explicit labels (`[CRITICAL]`, `[CONFIRMED]`, `[HEURISTIC]`); color is never the only signal.

### Non-interactive CLI (`internal/cli`, `cmd/depscope`)

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

Package syntax accepts `lodash@4.17.21`, `@scope/package`, `pypi:requests==2.31.0`, and explicit `npm:` prefixes. Global flags recognized before the command are `--no-color`, `--color=auto|always|never`, `--help`/`-h`, and `--version`/`-V`. Exit codes are `0` for success, `1` for an operational failure, `2` for a usage error, and `130` for interruption.

With no terminal attached, `depscope` prints help instead of starting the interactive menu. `NO_COLOR` and `TERM=dumb` disable ANSI color. Output is plain text suitable for redirection and scripting; there is no machine-readable JSON mode yet.

### Report rendering and terminal safety (`internal/render`)

The renderer converts shared scan reports, project results, and inventory results into plain text or limited ANSI styling. It sanitizes untrusted remote text (advisory details, package names, URLs, error strings) before display, removing control and escape sequences while preserving intentional theme styling. Sanitization works on copies, so the underlying report is not mutated. Color-disabled output contains no ANSI escapes.

### Offline demo (`internal/demo`)

The demo runtime uses the shared engine with deterministic demo providers, a fixed clock, fixed timestamps, demo-only sources, and no network access. It provides three scenarios:

| Scenario | Demonstrates |
| --- | --- |
| `vulnerable` | Confirmed critical and medium advisories with fixed versions and risk scores |
| `install-scripts` | Declared `preinstall`/`postinstall` scripts and a deprecation signal, without executing them |
| `clean` | A release with no advisories, no install scripts, and complete successful checks |

Every demo report is labelled `DEMO DATA`, uses synthetic `@depscope-demo/...` package names, and uses `DEMO-` advisory identifiers. Demo data cannot be mixed into live scans: the live scanner rejects demo requests and the demo scanner rejects live requests.

## Implemented in Phase 5

Phase 5 exposes the shared application service over HTTP. It adds transport, request validation, and process-local resource protection only; package resolution, vulnerability matching, risk rules, and report assembly remain in `internal/app` and the shared engine. There is no second report model: scan responses are canonical `model.ScanReport` values validated through `internal/contract` before they are written.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/healthz` | Lightweight process health check; returns `{"status":"ok"}` and no configuration or infrastructure details |
| `POST` | `/api/v1/scan` | Scan one npm or PyPI package from `ecosystem`, `name`, and an optional exact `version` |
| `POST` | `/api/v1/compare` | Scan two packages; each side carries either a canonical report or a generic side error |

Launch the API from the repository root:

```sh
go run ./cmd/depscope-api
```

or build it with `go build ./cmd/depscope-api`. It listens on `127.0.0.1:8080` by default. Configuration comes from `DEPSCOPE_API_*` environment variables; see [docs/api.md](docs/api.md) for the full table, endpoint schemas, error codes, and operational limitations.

Request handling is deliberately strict:

- Requests must be a single `application/json` object with only the documented fields. Unknown fields, null values where a string is required, trailing JSON values, query parameters, and unsupported media types are rejected with stable error codes.
- The request body is capped (16 KiB by default), package names are limited to 214 characters, and versions are limited to 256 characters of version-identifier characters. URL-shaped names and versions, path separators, and whitespace are rejected before any provider request is built.
- Package identities are normalized by the shared model and provider requests are constructed from validated values only. The API never accepts an arbitrary fetch URL and never executes package code or lifecycle scripts.

Operational protections are process-local:

- Every API request receives a bounded context deadline. The engine and its HTTP providers honor cancellation, and the binary also sets bounded server read-header, read, write, and idle timeouts.
- Concurrency is capped; requests beyond capacity receive `503` with `Retry-After` instead of queueing without limit.
- A per-client token bucket limits request bursts. It uses the TCP peer address, does not trust spoofable forwarding headers, and is not shared between processes.
- CORS is disabled unless an exact origin allowlist is configured. Wildcard origins are rejected at startup.
- Panics are contained and logged without exposing internals. Service errors are logged with detail but returned as generic JSON errors.
- Responses are not cached and carry `Cache-Control: no-store`; a partial or unavailable check can never appear as a cached success.
- The binary shuts down gracefully on `SIGTERM` or interrupt, waiting up to the configured shutdown timeout for in-flight requests.

There is no authentication, database, scan-history persistence, telemetry, or horizontal rate-limit coordination. See [docs/api.md](docs/api.md) for deployment limitations.

## Repository layout

```text
.
├── cmd/depscope/               # terminal binary entry point and signal handling
├── cmd/depscope-api/           # HTTP API binary, environment config, graceful shutdown
├── docs/                       # architecture, API reference, contracts, risk rules, plan, audit
├── internal/
│   ├── api/                    # HTTP handlers, validation, limits, CORS, rate limiting
│   ├── app/                    # shared application service for TUI, CLI, and API
│   ├── cache/                  # bounded in-memory TTL cache
│   ├── cli/                    # non-interactive commands, flags, exit codes
│   ├── contract/               # schema-backed contract validation
│   ├── demo/                   # offline deterministic demo scenarios
│   ├── engine/                 # orchestrator, interfaces, risk, typosquat, config
│   ├── fetch/                  # bounded JSON HTTP client with cache and retries
│   ├── inventory/              # read-only global npm and Python package inventory
│   ├── live/                   # live provider factory and opt-in smoke test
│   ├── model/                  # report model, status rules, validation
│   ├── project/                # project manifest and lockfile scanning
│   ├── render/                 # report documents, comparison, sanitization, color
│   ├── sources/                # npm, PyPI, and OSV provider adapters
│   └── tui/                    # Bubble Tea interactive terminal application
├── schemas/                    # canonical JSON Schema and embedded loader
├── testdata/fixtures/demo/     # deterministic synthetic reports
├── testdata/fixtures/inventory/ # synthetic installed-metadata fixtures
├── testdata/fixtures/projects/ # synthetic project trees and lockfiles
├── web/                        # placeholder; website planned for Phase 6
├── AGENTS.md
├── go.mod
└── go.sum
```

The Next.js application is not created yet.

## Verification

Run from the repository root:

```sh
gofmt -l .
go build -buildvcs=false ./...
go vet ./...
go test -count=1 ./...
```

`-buildvcs=false` is required in this environment because a broken empty `.git` directory exists in the parent home directory, which makes Go's VCS stamping fail with `error obtaining VCS status: exit status 128`. This project directory itself is not a Git repository. On a machine without that condition, plain `go build ./...` should be used.

`go test ./internal/contract/` validates every demo fixture against the canonical schema and checks the status, demo-labelling, missing-evidence, unknown-severity, and incomplete-result rules.

Project and inventory tests use synthetic fixture trees and injected command runners. They never modify installed global packages, never execute package code, and never make public network requests. Provider and orchestrator tests use sanitized `httptest` fixtures. Phase 4 terminal tests exercise the TUI state machine, CLI behavior, renderer formatting, sanitization, color policy, cancellation, and offline demo determinism without requiring a live terminal or network. Phase 5 API tests use an in-memory fake service to cover valid scans, canonical report validation, malformed and oversized payloads, unknown fields, timeouts, cancellation, provider-failure reports, compare side failures, CORS, rate limits, concurrency limits, and panic containment. The live smoke test is build-tagged and requires an explicit environment variable:

```sh
DEPSCOPE_LIVE_TESTS=1 go test -tags live -run TestLiveSmokeNPM ./internal/live/
```

## Documentation

- [docs/architecture.md](docs/architecture.md) - approved architecture, component boundaries, and open design decisions.
- [docs/api.md](docs/api.md) - HTTP endpoints, request and response contracts, error codes, environment variables, and operational limitations.
- [docs/contracts.md](docs/contracts.md) - report model, status semantics, interfaces, runtime limits, and demo rules.
- [docs/risk-rules.md](docs/risk-rules.md) - scan scope, evidence rules, risk scoring, typosquat dataset, and provider failure behavior.
- [docs/implementation-plan.md](docs/implementation-plan.md) - phased plan, dependencies, and exit criteria.
- [docs/repository-audit.md](docs/repository-audit.md) - verified repository state and toolchain details.
- [schemas/README.md](schemas/README.md) - canonical schema and versioning rules.
- [AGENTS.md](AGENTS.md) - persistent instructions for future coding agents.

## Current development state

- The Go module builds and all offline tests pass.
- The canonical report schema and demo fixtures are checked in.
- Live npm, PyPI, and OSV adapters and the scan orchestrator are implemented behind the existing interfaces.
- Project scanning supports npm lockfile v2/v3 graphs and exact-pinned Python `requirements.txt`; every other format is reported as unsupported rather than approximated.
- Global inventory covers top-level npm global packages and the selected Python interpreter's pip environment, using read-only listing mechanisms.
- The interactive TUI covers single-package scans, project scans, comparisons, global inventory, settings, the offline demo, and separate report views.
- The CLI exposes the same workflows non-interactively and falls back to help when no terminal is attached.
- The HTTP API serves canonical scan reports and comparisons over the same application service, with strict input validation, bounded concurrency and request timeouts, process-local rate limiting, an opt-in CORS allowlist, panic containment, and graceful shutdown.
- The offline demo runs without credentials or network access and is deterministic.
- Report text is sanitized at the render boundary; color is optional and never the sole indicator of risk.
- No live network request was made during Phase 5 verification; source connectivity and terms remain unverified.
- Git has not been initialized.
- No license has been selected.
- No website, database, authentication, scan-history persistence, or telemetry exists.
- Recursive dependency resolution beyond the npm lockfile graph, Python transitive resolution, and live verification remain for later phases.
- A machine-readable JSON output mode for the terminal has not been designed (D14); the HTTP API has its own JSON transport contract.

## License

No license has been selected. A license must be chosen before the project is described as distributable or published.
