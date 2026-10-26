# DepScope Implementation Plan

## 1. Purpose and status

This is a provisional, dependency-ordered plan derived from the approved architecture. It exists to keep future work sequenced and to make phase boundaries explicit.

- **Phase 0 is complete.**
- **Phase 1 is complete:** the Go module, report contract, schema validation, engine interfaces, configuration primitives, demo fixtures, and tests exist.
- **Phase 2 is complete:** the real security engine, live npm/PyPI/OSV adapters, risk rules, typosquat detection, reliability controls, and live scanner factory exist.
- **Phase 3 is complete:** project dependency scanning for npm lockfile v2/v3 graphs and exact-pinned Python requirements, plus read-only global npm and pip inventory, exist.
- **Phase 4 is complete:** the shared application service, report renderer, Bubble Tea terminal interface, non-interactive CLI, and deterministic offline demo exist.
- **Phase 5 is complete:** the production-oriented Go HTTP API, its strict transport contract, process-local limits, tests, and operational documentation exist.
- **Phase 6 is complete (2026-10-26):** the Next.js website uses the resolved pnpm/Node/Next/Tailwind toolchain. It includes the live scan proxy, schema-validated reports, comparison, an isolated offline demo, and browser verification.
- **Phase 7 remains deferred:** recursive dependency resolution, broader lockfile coverage, and opt-in live verification require an agreed D8 resolution semantics and a separate explicit instruction.
- Phase numbering and scope are provisional. The project owner's Phase 2 instruction implemented the engine and live adapters together, Phase 3 implemented project scanning and inventory, Phase 4 implemented the terminal experience, and Phase 5 implemented the HTTP API ahead of recursive resolution. This document records the revised sequence: the prior Phase 5 (recursive resolution) is now Phase 7, the prior Phase 6 (HTTP API) is Phase 5, and the prior Phase 7 (website) is Phase 6. Phases 8 and 9 are unchanged.

Each phase has a gate: work does not begin until the previous phase's exit criteria are met and the project owner approves the next phase.

## 2. Guiding rules

1. Do not implement a later phase early, even if the current phase appears small.
2. Resolve blocking decisions before writing code that depends on them.
3. Keep the shared engine as the only place where dependency analysis and risk logic live.
4. Build against deterministic fixtures first; live network integration comes after the fixture path is stable.
5. Add tests in the phase that introduces behavior, not in a later cleanup phase.
6. Do not add CI, deployment, or release automation until the hardening phase is explicitly approved.
7. Do not commit, push, publish, or deploy without explicit authorization.

## 3. Phase sequence

### Phase 0 — Repository setup and context (complete)

**Objective:** establish an accurate repository audit, approved architecture context, and minimal documentation scaffolding.

**Deliverables:**

- `README.md`
- `docs/architecture.md`
- `docs/implementation-plan.md`
- `docs/repository-audit.md`
- `AGENTS.md`

**Exit criteria:**

- Repository state and toolchain versions are recorded accurately.
- Approved architecture and boundaries are documented.
- Open decisions and blockers are listed.
- No functional application code, dependencies, CI, or deployment work has been added.

### Phase 1 — Repository foundations and contracts (complete)

**Objective:** make the repository buildable and fix the cross-cutting contracts before feature work.

**Status:** complete on 2026-10-25.

**Preconditions resolved:** D7 (schema source of truth) is resolved. D3, D5, D6, D8, D9, D11, and D12 are partially resolved to the extent needed for contracts. D1, D2, and D4 remain open and were not invented.

**Delivered:**

- Go module `depscope` targeting Go 1.26.8, with only the schema-validation dependency and its indirect text dependency.
- Domain model for ecosystem and package identity, canonical name normalization, data mode, scan status and transitions, severity ranking, finding kinds, check status, evidence, dependency graph, sources, limitations, and the complete scan report.
- Canonical JSON Schema `schemas/scan-report.v1.schema.json`, embedded in the Go binary and enforced by schema plus semantic cross-field validation.
- Status invariant: no checks, or any unavailable/skipped/failed check, yields `incomplete`, never `clean`.
- Engine interfaces for registry metadata, vulnerability lookup, risk evaluation, caching, and scan orchestration, with context-aware signatures.
- Scan configuration for timeouts, bounded concurrency, cache TTL, and cancellation, plus a concurrency limiter and a no-op cache.
- Deterministic demo fixtures with fixed timestamps, synthetic names, `DEMO-` identifiers, `dataMode: "demo"`, and demo-only sources.
- Tests for identity normalization, status transitions, serialization, schema validation, missing evidence, unknown severity, demo labeling, and incomplete results.
- `docs/contracts.md`, schema and fixture README files, and updated repository documentation.

**Deliberately deferred:**

- Git initialization, default branch, and remote URL (D1) — not authorized.
- License selection (D2) — not authorized.
- Frontend scaffold, package manager, Node/Next/Tailwind versions (D4) — unresolved.
- Any scan behavior or source access — assigned to Phases 2 and 3.
- Retry/backoff and live caching policy — deferred in Phase 1 and delivered in Phase 2 (D12).
- Final canonical Go module path — provisional until a remote exists (D3).

**Exit criteria met:**

- `go build ./...`, `go test ./...`, and `go vet ./...` pass; `gofmt -l .` produces no output.
- The schema is checked in, versioned `1.0.0`, and validated by automated contract tests.
- Layout and tooling decisions are documented.
- No registry, OSV, scanning, TUI, API, or website behavior was implemented.

### Phase 2 — Real security engine and live source adapters (complete)

**Objective:** implement evidence-based scanning of a resolved npm or PyPI package using the approved public sources, without redesigning the Phase 1 contracts.

**Status:** complete on 2026-10-25.

**Delivered:**

- npm Registry provider: package and version metadata, dependency and optional-dependency declarations, lifecycle scripts, deprecation, repository normalization, publication times, and scoped-name handling.
- PyPI provider: project and release JSON, requested and latest versions, PEP 503 normalization, `requires_dist` parsing, upload times, project URLs, yanked-release limitations, and missing-metadata limitations.
- OSV.dev provider: version-specific queries with `npm`/`PyPI` ecosystem mapping, package matching, advisory IDs, aliases, affected ranges, fixed versions, references, details, and severity normalization.
- Scan orchestrator: request validation, version resolution, check assembly, confirmed and heuristic findings, maintenance context, deterministic ordering, summary, and final-status derivation.
- Explainable risk rules separating confirmed findings from heuristic signals, with explicit statements that scores are prioritization values, not probabilities.
- npm lifecycle-script heuristics with a no-execution boundary.
- Typosquat detection using Damerau-Levenshtein distance and a versioned embedded reference dataset with provenance and update instructions.
- Bounded in-memory TTL cache, shared HTTP JSON client, scan/provider timeouts, bounded `429`/`5xx` retries, capped `Retry-After`, body limits, and classified provider errors.
- Live scanner factory wiring npm, PyPI, OSV, cache, limiter, risk rules, and the injectable clock.
- Provider tests with sanitized `httptest` fixtures, full-path engine integration tests, and a build-tagged opt-in live smoke test.

**Deliberately deferred:**

- Recursive dependency resolution and transitive vulnerability coverage. Declared dependencies are displayed as unresolved direct nodes and force an `incomplete` result.
- Lockfile, manifest, and installed-environment interpretation for project-level scans.
- Demo runtime activation and surface integration.
- CLI/TUI, HTTP API, and website work.
- Git initialization, license, CI, release, and deployment.

**Exit criteria met:**

- Real npm and PyPI provider paths produce schema-valid, evidence-based reports through the engine.
- Tests prove an OSV, version-metadata, typosquat-reference, or risk-evaluator failure cannot produce `clean`.
- Risk rules and scan scope are documented in `docs/risk-rules.md`.
- Provider failures, malformed responses, rate limits, cancellation, stale cache, and partial failures are covered by tests.
- Default tests make no public network requests and never execute package code.

### Phase 3 — Project scanning and global inventory (complete)

**Objective:** scan real project dependency declarations and installed package metadata with the shared engine, without executing project code or package-manager commands that modify the system.

**Status:** complete on 2026-10-25.

**Delivered:**

- Project detection restricted to fixed filenames in the project root, with no recursion into `node_modules`, vendor directories, or build outputs and no symlink traversal.
- npm project scanning for `package.json` paired with `package-lock.json` lockfile version 2 or 3: full lockfile graph, nested `node_modules` paths, direct/transitive relationships, dependency-path evidence, cycle and depth handling, and node limits.
- Explicit unresolved-node reporting when the lockfile is missing or an entry cannot be resolved. The registry latest version is never substituted for a project version.
- Python project scanning for `requirements.txt` with exact `==`/`===` pins only, including safe includes, comment and line-continuation handling, and explicit rejection of environment markers, options, editable installs, URLs, and unsupported lockfile formats.
- Distinct unsupported-format detection for yarn, pnpm, bun, npm shrinkwrap, Poetry, Pipenv, uv, PDM, `pyproject.toml`, `setup.py`, and `setup.cfg`.
- Shared batch scanning of resolved packages, with per-package partial failures preserved and findings ranked by severity and evidence.
- Read-only global npm inventory from `npm root -g` and top-level installed metadata, and read-only pip inventory from `python3`/`python -m pip list --format=json`.
- Inventory detection failures and coverage gaps reported as checks and limitations instead of empty success; symlinked and oversized or malformed metadata handled explicitly.
- Bounded file size and count, graph nodes, dependency depth, included files, inventory packages, command output, and command timeouts.
- Cancellation and progress callbacks for project and inventory scans.
- Synthetic project and inventory fixtures; tests for healthy trees, vulnerable transitives, missing and malformed lockfiles, unpinned requirements, unsupported formats, nested dependencies, conflicts, limits, detection failures, determinism, and incomplete coverage.

**Deliberately deferred:**

- Recursive Python resolution and Python lockfile formats.
- npm workspaces and peer dependency scanning.
- Additional npm lockfile formats such as v1, yarn, pnpm, bun, and shrinkwrap.
- Project and inventory serialization as a versioned wire report (D16).
- CLI/TUI, HTTP API, website, and demo runtime integration.
- Live network verification.
- Git initialization, license, CI, release, and deployment.

**Exit criteria met:**

- Supported formats are reported precisely, and unsupported formats are named rather than approximated.
- Resolved npm lockfile graphs expose transitive dependency paths and findings.
- Installed npm and pip packages are matched against the shared engine.
- Every detection failure or incomplete coverage path is visible as a non-succeeded check or limitation and cannot produce `clean`.
- Tests run without modifying installed global packages, executing package code, or making public network requests.

### Phase 4 — Polished terminal experience (complete)

**Objective:** make DepScope understandable and usable from the terminal for a first-time user, with interactive and non-interactive workflows over the existing engine, and a deterministic offline demo that requires no credentials or network.

**Status:** complete on 2026-10-26.

**Delivered:**

- `internal/app` service contract shared by the CLI and TUI, covering package scans, project scans, comparisons, global inventory, offline demo, progress callbacks, and cancellation. It delegates all scanning to the existing engine, project scanner, and inventory scanner.
- `internal/render` report documents for scan reports, project results, and inventory results, with overview, finding detail, checks, dependency tree, package details, sources and limitations, next actions, and compact summaries.
- Side-by-side package comparison with aligned fields, source coverage, data quality, and independent left/right failure handling; narrow terminals switch to a stacked layout.
- Terminal sanitization of untrusted remote text and error strings, applied to copies so reports are not mutated.
- Color policy with explicit `--no-color` and `--color=auto|always|never`, plus `NO_COLOR` and `TERM=dumb` overrides. Risk is always expressed with text labels, not color alone.
- Bubble Tea interface with the approved menu, keyboard navigation, forms, package/project/comparison/inventory/demo workflows, settings, help, and separate report views.
- Real progress reported by the engine, cancellable scans, stale-message protection, and no fabricated animation or delays.
- Small-terminal guard, help fallback when no terminal is attached, and no ANSI escapes when color is disabled.
- Non-interactive CLI commands (`scan`, `project`, `compare`, `inventory`, `demo`, `demo list`, `help`, `version`), global flags, documented exit codes, signal handling, and terminal detection.
- Deterministic offline demo with `vulnerable`, `install-scripts`, and `clean` scenarios, built through the shared engine with demo providers, a fixed clock, demo-only sources, and explicit `DEMO DATA` labelling.
- Tests for menu and screen transitions, all primary workflows, cancellation propagation and stale messages, progress handling, report views, comparison behavior, color policy, small terminals, non-TTY behavior, sanitization, and offline demo determinism.
- Updated README, architecture, contracts, audit, fixture rules, and agent instructions.

**Deliberately deferred:**

- Machine-readable JSON output and any command-envelope schema (D14).
- Recursive dependency resolution, broader lockfile coverage, and live verification (Phase 7).
- HTTP API (delivered in Phase 5) and website integration (Phase 6).
- Project and inventory wire serialization (D16).
- Git initialization, license, CI, release, and deployment.

**Exit criteria met:**

- A first-time user can run the tool, navigate the menu, complete the primary workflows, understand the evidence, and run the offline demo without credentials or network access.
- The CLI and TUI obtain all results through the shared application service; no risk rules, dependency resolution, or source logic are duplicated.
- Scans can be cancelled, progress reflects real work, and cancelled or superseded results are not displayed.
- Color-disabled output is plain text with explicit severity and status labels.
- Demo output is deterministic, offline, and unmistakably labelled; live and demo modes cannot mix.
- The full offline test suite passes without a live terminal or public network requests.

### Phase 5 — Production-oriented Go HTTP API (complete)

**Objective:** expose the shared scan service over a validated, resource-bounded HTTP API without duplicating engine logic or the report contract.

**Status:** complete on 2026-10-26.

**Delivered:**

- `internal/api` transport package with `GET /healthz`, `POST /api/v1/scan`, and `POST /api/v1/compare` handlers over the shared `app.Service`.
- Strict JSON request validation: single object, documented fields only, required string fields, case-sensitive keys, query rejection, content-type check, and configured body-size limit.
- Ecosystem, package-name, and version validation before provider calls, including length caps and a version identifier pattern that rejects URLs, paths, whitespace, and control characters.
- Canonical `model.ScanReport` responses validated through `internal/contract`, with live data-mode, target identity, and requested-version consistency checks before serving.
- Comparison envelope with independent side reports and generic side errors; both sides failing returns `502`.
- Stable error envelope and documented status/code mapping without leaking internal errors or secrets.
- Per-request deadlines, capped concurrency with `503` backpressure, process-local per-client token-bucket rate limiting, panic containment, exact-origin CORS allowlist, `Cache-Control: no-store`, and security headers.
- `cmd/depscope-api` with environment configuration, JSON logging, explicit server timeouts, and graceful `SIGTERM`/interrupt shutdown.
- `internal/api` tests using an in-memory fake service for valid scans, contract validation, malformed/oversized/unknown-field requests, timeouts, cancellation, partial reports, compare failures, CORS, rate limits, concurrency limits, panic containment, and configuration parsing.
- `docs/api.md` plus updates to README, architecture, contracts, audit, and agent instructions.

**Deliberately deferred:**

- Authentication, authorization, trusted-proxy handling, shared multi-instance rate limits, and deployment topology (D10 remains partial).
- Project and inventory wire serialization and API exposure (D16 remains open).
- Response caching, scan-history persistence, telemetry, and a demo endpoint.
- Website implementation (Phase 6).
- Recursive dependency resolution and live verification (Phase 7).

**Exit criteria met:**

- The API builds and exposes the documented endpoints.
- Handler tests pass without public network access or a live terminal.
- Malformed, empty, unknown-field, oversized, non-JSON, and URL-shaped inputs are rejected with stable errors before scanning.
- Request timeouts, concurrency, body size, rate limiting, CORS, panic recovery, and shutdown are bounded and tested.
- Scan and compare responses use the shared engine and canonical report contract; partial or unavailable checks remain visible and cannot be confused with clean results.
- No internal error text, environment value, or secret is returned to clients.

### Phase 6 — Next.js website

**Objective:** present reports in a typed, accessible web interface.

**Status:** complete on 2026-10-26. D4 was resolved before the existing scaffold was created.

**Dependencies:** Phase 1 schema; Phase 5 API contract; D4, D10, and D16. The scan and compare endpoints are available, but project/inventory results are not part of the wire contract (D16), so the website must not display them as ad hoc JSON. UI work may begin against schema-valid fixtures once the stack is chosen.

**Delivered:**

- Next.js App Router, TypeScript, Tailwind CSS v4 and pnpm app with schema-generated TypeScript types.
- Same-origin server proxy to the Go API, request bounds/timeouts, report validation, and explicit cancellation, error, and partial-result views.
- Live package scans, side-by-side comparisons, evidence-led reports, JSON export, and selectable dependency nodes with a textual path list.
- Deterministic offline demo and comparison pages using validated fixtures, with visible demo labels and no scan or comparison API requests.
- SEO and Open Graph metadata with a generated social image; optional repository URL configuration.
- CLI install and quick-start, supported scope, source and methodology, limitation, API configuration, accessibility, and contribution documentation.
- Responsive dark inspection interface, keyboard navigation, reduced-motion handling, Vitest coverage, Playwright browser tests, and a production build.

**Exit criteria:**

- The website renders schema-valid live API reports and explicitly labelled offline fixtures.
- Live scans and comparisons use only the shared Go API; browser code has no independent security rules.
- The production build, lint, typecheck, and unit tests pass.
- Browser tests cover the package scan journey, demo/live separation, and mobile layout.

### Phase 7 — Recursive dependency resolution and live verification

**Objective:** extend scanning beyond the supported lockfile graph to broader dependency closures, and verify the live adapters against real public services under controlled, opt-in conditions.

**Status:** deferred. Requires a separate explicit instruction and an agreed D8 resolution semantics.

**Dependencies:** Phase 3 complete; D8 resolution semantics must be agreed before implementation. Confirmed source terms, client identification, and rate limits are required before live verification is treated as available. D16 should be resolved before recursive project results are exposed through transport surfaces. Any new wire fields require an explicit schema decision.

**Proposed deliverables:**

- Agreed resolution semantics for npm and Python, including additional lockfile/manifest inputs, workspace and peer dependency policy, transitive depth, and development-dependency policy.
- Recursive Python resolution and vulnerability queries for every resolved node, with dependency-path evidence.
- Broader npm lockfile support only where semantics are agreed and testable.
- Cycle, duplicate, conflict, and unresolved-node handling across the full graph.
- Partial-failure behavior for mixed registry and OSV results across the graph.
- Recorded fixtures derived from representative live responses with retained provenance.
- Opt-in live verification results and documented source terms/rate-limit observations.

**Exit criteria:**

- A project with Python dependencies can complete a resolved scan, or clearly reports which nodes remain unresolved.
- Transitive findings carry dependency-path evidence and appear in the graph.
- Fixture tests remain deterministic and network-free.
- Live verification is opt-in, documented, and does not gate the default test suite.
- No source failure, unresolved node, or skipped check can produce `clean`.

### Phase 8 — Cross-surface integration and demonstration

**Objective:** prove that terminal and website results agree and that the offline demo is reproducible.

**Dependencies:** Phases 4–7; D11 (demo placement); D15 if graph or comparison features are approved.

**Proposed deliverables:**

- End-to-end offline demo for both surfaces.
- Cross-surface consistency checks on the same fixture scan.
- Dependency graph and comparison features only if separately specified and approved.
- Accessibility, performance, and error-state review.
- Updated user-facing documentation.

**Exit criteria:**

- The same scan input produces consistent report content across surfaces.
- Demo output is deterministic and clearly labelled.
- Unapproved features remain out of scope.

### Phase 9 — Hardening, release, and operations

**Objective:** prepare the project for distribution and operation.

**Dependencies:** Phase 8 complete; D1, D2, D13, and deployment decisions resolved.

**Proposed deliverables:**

- CI pipelines and required checks.
- Release packaging for the CLI and API.
- Website deployment configuration.
- Security review, dependency pinning, and vulnerability response process.
- Operational documentation and rollback plan.
- Final license, contribution, and security-policy documents.

**Exit criteria:**

- CI verifies build, tests, schema contract, and website build.
- Release artifacts are reproducible and documented.
- Deployment requires separate explicit approval.
- All known blockers are either resolved or documented as accepted risks.

## 4. Dependency summary

```text
Phase 0 (context, complete)
    └─> Phase 1 (foundations and contracts, complete)
            └─> Phase 2 (engine, live npm/PyPI/OSV adapters, risk rules, complete)
                    └─> Phase 3 (project scanning and global inventory, complete)
                            ├─> Phase 4 (terminal experience, complete)
                            ├─> Phase 5 (HTTP API, complete)
                            │       └─> Phase 6 (website)
                            └─> Phase 7 (recursive resolution and live verification, deferred)
Phase 1 ─────────────────────────> Phase 6 (may start against fixtures)
Phase 5 ─────────────────────────> Phase 6
Phase 6 ─────────────────────────> Phase 8 (integration and demo)
Phase 7 ─────────────────────────> Phase 8 (if approved)
                                        └─> Phase 9 (hardening and release)
```

Phases 4, 5, and 6 are complete. The website uses a same-origin server proxy; the Go API CORS allowlist is not used by browser requests in this deployment shape. Phase 7 is deferred and requires agreed D8 semantics before code is written. Phase 7 can proceed independently of the website only when explicitly authorized. D16 must be resolved before project or inventory results are served by the API or rendered by the website.

## 5. Verification strategy

### Verified in Phase 0

The toolchain and repository state commands were executed and recorded in `docs/repository-audit.md`: `go version`, `node --version`, `npm --version`, `pnpm --version`, `yarn --version`, `bun --version`, `git --version`, `make --version`, `docker --version`, `git status --short --branch`, and `git log --oneline -15`.

### Verified in Phase 1

The following commands were executed for Phase 1:

| Area | Command | Result |
| --- | --- | --- |
| Go build | `go build ./...` | Passes |
| Go tests | `go test ./...` | Passes |
| Go static checks | `go vet ./...` | Passes |
| Go formatting | `gofmt -l .` | No output |
| Schema validation | Exercised through `internal/contract` tests against the embedded schema and all demo fixtures | Passes |
| Frontend type check | Not applicable | No frontend exists |

### Verified in Phase 2

The following commands were executed for Phase 2. Default tests use only `httptest` servers and sanitized fixtures; they make no public network requests.

| Area | Command | Result |
| --- | --- | --- |
| Go build | `go build ./...` | Passes |
| Go tests | `go test -count=1 ./...` | Passes |
| Go static checks | `go vet ./...` | Passes |
| Go formatting | `gofmt -l .` | No output |
| Provider and integration tests | `go test ./internal/sources/... ./internal/engine/ ./internal/live/ ./internal/fetch/ ./internal/cache/` | Passes |
| Opt-in live smoke compile check | `go test -tags live -run '^$' ./internal/live/` | Compiles; not executed against public services |
| Live smoke run | `DEPSCOPE_LIVE_TESTS=1 go test -tags live -run TestLiveSmokeNPM ./internal/live/` | Not run; requires public network access and is intentionally opt-in |

The live smoke test exists but was not executed during Phase 2, so live connectivity to npm, PyPI, and OSV remains unverified in this environment.

### Verified in Phase 3

The following commands were executed for Phase 3. Default tests use synthetic project trees, synthetic inventory metadata, and injected command runners. They do not modify installed global packages, execute package code, or make public network requests.

| Area | Command | Result |
| --- | --- | --- |
| Go build | `go build ./...` | Passes |
| Go tests | `go test -count=1 ./...` | Passes |
| Go static checks | `go vet ./...` | Passes |
| Go formatting | `gofmt -l .` | No output |
| Project scanning tests | `go test -count=1 ./internal/project/` | Passes |
| Inventory tests | `go test -count=1 ./internal/inventory/` | Passes |
| Full offline suite | `go test -count=1 ./...` | Passes for all packages, including contract, engine, live, sources, project, and inventory |
| Opt-in live smoke compile check | `go test -tags live -run '^$' ./internal/live/` | Compiles; not executed against public services |
| Live smoke run | `DEPSCOPE_LIVE_TESTS=1 go test -tags live -run TestLiveSmokeNPM ./internal/live/` | Not run; requires public network access and is intentionally opt-in |

Live connectivity to npm, PyPI, and OSV remains unverified. No global npm or pip inventory was executed against the real machine during verification; command behavior is covered through injected runners and synthetic metadata.

### Verified in Phase 4

The following commands were executed for Phase 4. Terminal tests exercise the state machine and rendering with in-memory input/output and injected services; they do not require a live terminal. The offline demo was verified through repeated deterministic output comparisons and engine tests.

| Area | Command | Result |
| --- | --- | --- |
| Go build | `go build -buildvcs=false ./...` | Passes |
| Go tests | `go test -count=1 ./...` | Passes for all tested packages |
| Go static checks | `go vet ./...` | Passes |
| Go formatting | `gofmt -l .` | No output |
| Application service tests | `go test -count=1 ./internal/app/` | Passes |
| CLI tests | `go test -count=1 ./internal/cli/` | Passes |
| TUI tests | `go test -count=1 ./internal/tui/` | Passes |
| Renderer tests | `go test -count=1 ./internal/render/` | Passes |
| Offline demo tests | `go test -count=1 ./internal/demo/` | Passes |
| Opt-in live smoke compile check | `go test -tags live -run '^$' ./internal/live/` | Compiles; not executed against public services |
| Live smoke run | `DEPSCOPE_LIVE_TESTS=1 go test -tags live -run TestLiveSmokeNPM ./internal/live/` | Not run; requires public network access and is intentionally opt-in |

`go build ./...` without `-buildvcs=false` fails in this environment because a broken empty `.git` directory exists in the parent home directory, causing Go VCS stamping to exit with status 128. This is an environment condition, not a code failure; the project directory is not a Git repository. Plain `go build ./...` should be used on a machine without that condition.

A pseudo-terminal smoke test confirmed that the interactive app starts, renders the menu, accepts keyboard input, opens a form, returns with `esc`, and exits with `q`. The menu, screen transitions, cancellation, progress, report views, small-terminal guard, and non-TTY help fallback are covered by automated tests. A full human interactive review in a normal terminal is still recommended before release.

### Verified in Phase 5

The following commands were executed for Phase 5. API tests use an in-memory fake service and `httptest`; they do not require a live terminal, public network, external process, or database. A local binary smoke test used only `/healthz` and invalid requests, so it made no provider calls.

| Area | Command | Result |
| --- | --- | --- |
| Go build | `go build -buildvcs=false ./...` | Passes |
| Go tests | `go test -count=1 ./...` | Passes for all tested packages |
| Go static checks | `go vet ./...` | Passes |
| Go formatting | `gofmt -l .` | No output |
| API tests | `go test -count=1 ./internal/api/` | Passes |
| API race and coverage | `go test -race -count=1 -cover ./internal/api/` | Passes; 85.8% statement coverage at the time of verification |
| API binary build | `go build -buildvcs=false -o /tmp/depscope-api ./cmd/depscope-api` | Passes |
| Local health and validation smoke | `GET /healthz`, invalid scan bodies, wrong method, unknown route | Correct status codes and stable JSON errors; no scan was executed |
| Graceful shutdown smoke | `SIGTERM` to the running binary | Logs shutdown and exits cleanly |
| Opt-in live smoke compile check | `go test -tags live -run '^$' ./internal/live/` | Compiles; not executed against public services |
| Live smoke run | `DEPSCOPE_LIVE_TESTS=1 go test -tags live -run TestLiveSmokeNPM ./internal/live/` | Not run; requires public network access and is intentionally opt-in |

No real npm, PyPI, or OSV scan was executed through the API during verification. Source connectivity and terms remain unverified. Authentication, deployment, multi-instance limits, and browser CORS integration are not implemented.

### Anticipated commands (not runnable yet)

These commands must not be reported as passing until the corresponding components exist.

| Area | Command | Becomes available |
| --- | --- | --- |
| Web install | Chosen package manager install | Phase 6 |
| Web lint | Chosen lint script | Phase 6 |
| Web type check | Chosen type-check script | Phase 6 |
| Web production build | Chosen build script | Phase 6 |
| Browser verification | Manual or chosen browser automation | Phase 6 |
| End-to-end checks | Command to be chosen | Phase 8 |
| CI | Pipeline to be defined | Phase 9 |

## 6. Risks and mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Undefined scan semantics | Results may not mean what users expect | Phase 3 limits support to npm lockfile v2/v3 and exact-pinned Python requirements and reports everything else as incomplete; resolve D8 before Phase 7 recursion |
| Unsupported or missing lockfiles | Users may overestimate coverage | Report supported formats precisely, never substitute latest versions, and force `incomplete` with explicit limitations |
| Local project scans read unintended files | Unwanted filesystem access or misleading results | Fixed filenames in the project root, bounded file count and size, include escape checks, no symlink traversal, and no recursion into dependency or build directories |
| Inventory detection fails silently | Empty results look secure | Represent detection failures as unavailable checks and limitations; return `failed` when every configured manager fails |
| Source API or rate-limit changes | Live scans break or are throttled | Isolate adapters, record fixtures, implement timeouts/backoff/caching, and surface partial failures |
| Go/TypeScript schema drift | Terminal and website disagree | Contract tests and a single schema source of truth |
| Offline demo drifts from real behavior | Demo becomes misleading | Reuse engine paths, snapshot tests, fixed timestamps, visible labelling |
| Network-dependent tests | Flaky or unavailable builds | Fixture-first testing; live tests opt-in |
| TUI test complexity | Regressions in interactive behavior | Phase 4 separates the state machine and renderers from terminal I/O and covers menu, workflows, cancellation, and report views with automated tests; manual release checks remain |
| Terminal quality outside ideal conditions | Unusable output in small or non-color terminals | Explicit small-terminal guard, non-TTY help fallback, `NO_COLOR`/`TERM=dumb` handling, text risk labels, and narrow-layout comparison tests |
| Untrusted remote text | Terminal injection or XSS | Sanitize at the render boundary on copies; test with hostile fixture strings; HTML escaping remains required for the future website |
| Public API abuse or resource exhaustion | Unbounded upstream calls, memory growth, or denial of service | Strict body and field limits, request deadlines, non-blocking concurrency cap with `503`, per-client process-local rate limiting, and no queuing beyond capacity; authentication and shared multi-instance limits remain open |
| API leaks internal detail | Secrets or infrastructure exposed to clients | Stable generic error envelope, report contract validation, health response limited to `{"status":"ok"}`, and detailed errors logged only server-side |
| Scope creep across phases | Unreviewed architecture and partial features | Phase gates, explicit approvals, and updating this plan when scope changes |
| Toolchain/version drift | Build differences across machines | Pin Go and Node toolchains and JavaScript dependencies in Phase 1 and Phase 6 |
| Missing license or canonical module path | Blocks publication and external imports | Module path is provisional; resolve D1–D3 before any publish or release work |

## 7. Definition of done for future changes

A change is complete only when:

- It stays within the approved phase scope.
- Its behavior is tested at the appropriate layer.
- Source-code comments and docstrings are written in Simplified Chinese, while identifiers, API fields, user-facing product text, and public documentation remain in English.
- Architecture, schema, or decision documentation is updated when the change alters a contract.
- Verification commands were actually run and their results are reported.
- No secrets, invented URLs, or unapproved deployment actions are included.
