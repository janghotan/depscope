# AGENTS.md

## Purpose

These instructions apply to every coding agent working in this repository. Read them before making changes. They supplement, but do not replace, explicit instructions from the project owner for a specific phase.

## Required reading order

1. `README.md` — project scope and current status.
2. `docs/architecture.md` — approved architecture, boundaries, and open decisions.
3. `docs/implementation-plan.md` — phase order, dependencies, and exit criteria.
4. `docs/repository-audit.md` — verified repository and toolchain state.
5. `docs/contracts.md` — implemented domain model, report contract, status semantics, and engine interfaces.
6. `docs/risk-rules.md` — documented risk, confidence, typosquat, and failure rules.
7. `schemas/README.md` — canonical schema rules and validation command.
8. `testdata/fixtures/README.md` — demo fixture labeling and isolation rules.

If a phase instruction conflicts with these documents, stop and ask the project owner which source is authoritative. Do not silently reinterpret the architecture.

## Current repository state

As of Phase 8 verification (2026-10-26):

- A Go module exists at the module path `github.com/janghotan/depscope`, matching the public repository `origin`.
- Implemented: domain model, canonical report schema v1, schema-backed contract validation, engine interfaces, scan configuration and limiter, scan orchestrator, live npm/PyPI/OSV providers, explainable risk rules, typosquat heuristics, bounded cache and HTTP client, batch resolved-package scanning, project dependency scanning, read-only global package inventory, report rendering and comparison, shared application service, Bubble Tea interactive terminal app, non-interactive CLI and binary, deterministic offline demo runtime, production-oriented HTTP API and API binary, and tests.
- Project scan scope is limited to npm `package.json` with `package-lock.json` lockfile version 2 or 3, and Python `requirements.txt` with exact `==`/`===` pins. npm workspaces and peer dependencies, Python transitive resolution, Python lockfiles, and other npm lockfile formats are not implemented. Unsupported or unresolved input is reported as incomplete, never approximated with the latest version.
- Global inventory covers top-level npm global packages and the selected pip interpreter's default environment. It is read-only and never installs, imports, upgrades, or executes packages.
- Project and inventory results use separate Go result types assembled from shared model types; they are not yet schema-valid `ScanReport` values (D16). The terminal renders them through `internal/render.Document`; the HTTP API does not expose them.
- The CLI and TUI obtain all scan results through `app.Service`; they do not access registry, OSV, project, inventory, or risk packages directly.
- The HTTP API exposes `POST /api/v1/scan`, `POST /api/v1/compare`, and `GET /healthz` through `internal/api`, with the binary entry point in `cmd/depscope-api`. It consumes a narrow `ScanService` interface satisfied by `app.LiveService`, validates outgoing reports with `internal/contract.ValidateReport`, and does not assemble or reinterpret reports.
- The API is a local, unauthenticated service. It has no persistence, stored history, accounts, telemetry, TLS termination, or multi-instance coordination. Rate limiting and concurrency limits are process-local and keyed by TCP peer address; forwarding headers are not trusted.
- The offline demo is deterministic, offline, and isolated from live mode. It is activated by `depscope demo` or the TUI demo picker.
- The Phase 6 website lives under `web/`; it uses Next.js App Router, TypeScript, Tailwind CSS, and pnpm as recorded in `web/package.json` and `web/pnpm-lock.yaml`.
- The website consumes generated report types and the canonical schema, uses same-origin server routes to proxy the Go API, and does not call registry or vulnerability sources directly.
- Live web reports support one package at a time, with two-package comparison. Offline demo pages use validated synthetic fixtures and make no scan or comparison API requests; the shared header can check the local API health status.
- Not implemented: recursive Python dependency resolution, broader lockfile support, project/inventory wire serialization, machine-readable CLI JSON output, and public deployment.
- Git is initialized on `feature/phase-6-website`, whose history is published as `main` at `https://github.com/janghotan/depscope` (default branch `main`, public). The local branch tracks `origin/main`.
- `LICENSE` contains the Apache License 2.0 text with only the copyright line localized to `Copyright 2026 Jangho Tan`; `NOTICE` records third-party licenses. The maintainer selected Apache-2.0 and that attribution.
- Phase 7 added CI, cross-platform build verification, and gated GoReleaser configuration. The license and copyright notices are now settled; tagged releases still require an explicit maintainer decision on the version and publication. No release or deployment has occurred.
- Direct runtime dependencies: `github.com/santhosh-tekuri/jsonschema/v6`, `github.com/charmbracelet/bubbletea`, and `github.com/charmbracelet/lipgloss`; their pinned indirect dependencies are recorded in `go.mod` and `go.sum`. The HTTP API uses only the Go standard library and existing internal packages.
- The Go and website build/test commands are listed in `README.md` and `web/README.md`; report the exact commands actually run.
- Do not assume planned directories, transports, or commands beyond those listed in `README.md` exist.

## Phase discipline

- Work only on the phase explicitly requested by the project owner.
- Do not begin the next phase, implement "obvious" follow-up features, or create placeholder application code unless the current instruction assigns that work.
- Do not spawn agents to implement features unless explicitly authorized for that phase.
- Stop and report when the requested phase is complete.
- A plan, an empty checklist, or available time is not permission to continue.

## Language rules

- All newly written source-code comments and docstrings must be in **Simplified Chinese (简体中文)**. This includes Go, TypeScript, JavaScript, shell, YAML, Dockerfiles, and tests.
- Keep code identifiers, API field names, user-facing product text, `README.md`, and public documentation in **English** unless explicitly requested otherwise.
- Preserve existing comments, license notices, and user-authored content unless a change is necessary.

## Architecture guardrails

- The shared Go security engine is the only place for dependency resolution, vulnerability matching, risk assessment, and report assembly.
- The CLI/TUI and HTTP API are adapters over the engine; they must not duplicate its logic.
- The CLI and TUI must use `app.Service`; do not call `internal/engine`, `internal/project`, `internal/inventory`, or `internal/sources` directly from terminal surfaces.
- The HTTP API must consume `app.Service` (through a handler-level interface), never call providers or the engine directly, and must validate every outgoing report with `internal/contract.ValidateReport` before responding.
- API errors must use the stable documented error envelope. Do not leak internal errors, stack traces, environment values, or secrets in responses.
- API resource limits (body size, request timeout, concurrency, rate limiting) are process-local. Do not add distributed rate limiting, persistence, authentication, or telemetry without explicit approval and documentation.
- Renderers consume reports and must not mutate findings or recalculate risk. Sanitize untrusted text on copies at the render boundary.
- Never make color the only way to communicate severity, status, finding kind, check outcome, or demo mode.
- Interactive progress must reflect real engine work. Do not fabricate progress or add artificial delays.
- Cancelling a scan must cancel its context, and results or progress from cancelled or superseded scans must not be displayed.
- The offline demo must go through the shared engine with demo providers; live and demo modes must never share a scanner or data path.
- The website consumes the shared report contract; it must not call npm, PyPI, or OSV directly in the initial scope.
- npm and PyPI are the only approved ecosystems for the initial scope.
- OSV.dev, the npm Registry, and PyPI are the approved initial sources.
- Keep external sources behind replaceable interfaces so deterministic fixtures can be used.
- The offline demo must be deterministic, network-free, reproducible, and clearly labelled in every surface.
- Treat the report schema as a versioned cross-language contract. Do not change it without updating both consumers and contract checks.
- `schemas/scan-report.v1.schema.json` is the canonical source of truth for the wire contract. Go types in `internal/model` must stay aligned with it, and any future TypeScript types must be generated from it rather than hand-copied.
- Schema version changes require a new schema file and an explicit version bump; do not silently mutate `1.0.0`.
- A missing, unavailable, skipped, or failed check must never produce a clean result. `clean` is valid only when every check succeeded, no findings exist, and at least one source is recorded.
- Demo reports must use `dataMode: "demo"`, a non-empty `demoNotice`, `provider: "demo"` sources, synthetic package names, `DEMO-` identifiers, fixed timestamps, and no fabricated URLs.
- Live reports must use `dataMode: "live"`, a null `demoNotice`, and only `osv`, `npm`, or `pypi` sources.
- Validate reports through `internal/contract.ValidateReport` (schema plus semantic checks), not by ad hoc assertions.

## Security and privacy rules

- Never commit credentials, tokens, private keys, or secrets.
- Never invent repository URLs, API keys, deployment configuration, or toolchain capabilities.
- Treat registry and OSV responses as untrusted input.
- Escape untrusted text for terminal output to prevent escape-sequence injection.
- Escape untrusted text for HTML output to prevent cross-site scripting.
- Never execute package code or run package lifecycle scripts. Lifecycle scripts are inspection signals only.
- Do not upload source code or local file contents to external services unless explicitly approved and documented.
- Respect each source's terms, rate limits, and client-identification requirements.
- Do not add telemetry, analytics, or crash reporting without explicit approval.

## Dependency and tooling rules

- Do not install dependencies merely to prepare for future work.
- Install dependencies only when the current approved phase requires them.
- Use the package manager chosen for the project; do not introduce a second package manager.
- Keep the validated toolchain baseline aligned with `go.mod` (`go 1.26.10`) unless a phase explicitly updates it.
- Pin versions and commit lockfiles once manifests exist.
- Do not add CI pipelines, release workflows, deployment infrastructure, or production integrations unless the phase explicitly authorizes them.
- Verify a tool exists before documenting or using it. Distinguish commands actually run from anticipated commands.
- Run `gofmt -w` on changed Go files; `gofmt -l .` must print nothing.
- The current Go commands are `go build ./...`, `go test ./...`, `go vet ./...`, and `gofmt -l .`. Website commands are documented in `web/README.md`.

## Testing and verification rules

- Add tests in the same phase that introduces behavior.
- Prefer deterministic fixture-based tests; live network tests must be opt-in.
- Add a failing test first, verify that it fails for the expected reason, then implement.
- Run `go test -count=1 ./...` before reporting completion; report exact commands and results.
- Use Go `httptest` and sanitized fixtures for all default provider and integration tests. Never make public network requests in the default test suite.
- Keep live smoke tests behind the `live` build tag and an explicit environment variable, and never report them as passing unless they were executed.
- Validate changed or added fixtures through `internal/contract` on every test run.
- Never claim a build, test, or scan passed without running it and checking the result.
- For terminal UI changes, separate render/state logic from terminal I/O and perform manual verification when possible.
- For website changes, build the site and verify main flows and error states in a browser when tooling permits. If browser verification is impossible, state that explicitly.
- Confirm that offline demo output is stable across repeated runs.

## Git and change rules

- Do not initialize Git, commit, push, publish, or deploy unless the project owner explicitly asks.
- Do not use destructive Git operations without explicit approval.
- Before any operation that could discard work, inspect the working tree and preserve existing changes.
- Keep changes scoped to the requested phase.

## Decision handling

- Do not silently choose among unresolved decisions listed in `docs/architecture.md` (D1–D16).
- If a decision blocks the current phase, report it and ask for direction.
- Record any newly resolved decision in the architecture document and update the implementation plan when sequencing changes.

## Documentation rules

- Keep `README.md` aligned with the actual repository state.
- Update `docs/architecture.md` when component boundaries or contracts change.
- Update `docs/implementation-plan.md` when phase scope or dependencies change.
- Update `docs/repository-audit.md` when the verified toolchain or repository state changes materially.
- Do not create planning, decision, or analysis documents unless requested.

## Phase boundaries

Phase 1 established contracts only. Phase 2 implemented the engine and live source adapters for a single resolved target package. Phase 3 implemented project scanning for npm lockfile v2/v3 graphs and exact-pinned Python requirements, plus read-only global npm and pip inventory. Phase 4 implemented the report renderer, shared application service, Bubble Tea interactive terminal app, non-interactive CLI and binary, and deterministic offline demo runtime. Phase 5 implemented the production-oriented HTTP API and API binary.

Phase 5 does not authorize implementing the website, recursive Python resolution, additional lockfile or manifest formats, npm workspace or peer dependency scanning, project/inventory wire serialization, machine-readable CLI JSON output, authentication, persistence, telemetry, CI, release workflows, deployment, or live verification runs. Each later phase requires a separate, explicit instruction.

Phase 6 (the Next.js website) is complete. Phase 7 is the owner-approved open-source launch and release-engineering scope. Recursive dependency resolution and live verification remain deferred and require agreed D8 semantics and a separate explicit instruction before code is written.
