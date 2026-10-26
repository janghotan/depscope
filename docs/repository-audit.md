# Repository Audit

- **Audit date:** 2026-10-25
- **Audited path:** `/home/jangho/Projects/DepScope`
- **Audit type:** read-only inspection, followed by documentation-only Phase 0 scaffolding

## 1. Summary

At the start of Phase 0, the target directory was completely empty. It contained no files, hidden files, Git metadata, source code, documentation, tests, CI workflows, dependency manifests, or installed dependencies.

No existing DepScope work was found to preserve. The only pre-existing configuration relevant to future work was the machine-level toolchain and global Git configuration.

Phase 0 added documentation only. No application component, dependency, build manifest, CI pipeline, or deployment configuration was created.

## 1.1 Phase 1 update (2026-10-25)

Phase 1 added the first application code and build manifest. The Phase 0 findings below remain an accurate record of the state before Phase 1.

Current state:

- `go.mod` exists with module path `depscope` (provisional) and `go 1.26.8`.
- Runtime dependency: `github.com/santhosh-tekuri/jsonschema/v6 v6.0.3`; indirect: `golang.org/x/text v0.14.0`.
- Packages: `internal/model`, `internal/engine`, `internal/contract`, and `schemas` (embedded schema).
- Canonical contract: `schemas/scan-report.v1.schema.json`, schema version `1.0.0`.
- Demo fixtures: three labeled scenario reports under `testdata/fixtures/demo/`.
- Tests: unit and contract tests exist and pass.
- No live network request to npm, PyPI, or OSV was made.
- No frontend manifest, dependency, or build exists; `web/README.md` is a placeholder.
- Git is still not initialized. No license was added.
- No CI, release, deployment, database, authentication, or telemetry was added.

Phase 1 executable verification:

| Command | Result |
| --- | --- |
| `go build ./...` | Passes |
| `go test ./...` | Passes |
| `go vet ./...` | Passes |
| `gofmt -l .` | No output |

The frontend type-check command could not be run because no frontend exists. This is expected and is not a passing result.

## 1.2 Phase 2 update (2026-10-25)

Phase 2 added the real security engine and live source adapters. The Phase 0 and Phase 1 records above remain accurate for their dates.

Current state:

- Go module path remains provisional `depscope`; dependencies are unchanged from Phase 1.
- New packages: `internal/cache` (bounded TTL memory cache), `internal/fetch` (shared JSON HTTP client), `internal/sources/npm`, `internal/sources/pypi`, `internal/sources/osv`, and `internal/live` (production wiring).
- Extended packages: `internal/engine` now contains the scan orchestrator, explainable risk rules, typosquat detection, and a versioned reference dataset at `internal/engine/reference/packages-v1.json`.
- `internal/engine/interfaces.go` gained internal-only version metadata types; the wire schema remains `1.0.0` and unchanged.
- No CLI, TUI, HTTP API, or website code exists. `web/README.md` remains a placeholder.
- Git is still not initialized. No license was added.
- No CI, release, deployment, database, authentication, or telemetry was added.

## 1.3 Phase 3 update (2026-10-25)

Phase 3 added project dependency scanning and read-only global package inventory. The Phase 0–2 records above remain accurate for their dates.

Current state:

- Go module path remains provisional `depscope`; the Go dependency set is unchanged from Phase 2.
- New packages: `internal/project` (manifest/lockfile detection, npm lockfile v2/v3 graph construction, exact-pinned Python requirements parsing, project result assembly) and `internal/inventory` (read-only npm and pip listing, installed metadata parsing, inventory result assembly).
- Extended package: `internal/engine` exposes `ScanResolvedBatch` for bounded batch scanning with per-package partial failures.
- New synthetic fixtures: `testdata/fixtures/projects/` for project trees and `testdata/fixtures/inventory/` for installed metadata.
- The canonical report schema remains `1.0.0` and unchanged. Project and inventory results use separate exported Go result types assembled from shared model types; they are not yet wire reports.
- No CLI, TUI, HTTP API, or website code exists. `web/README.md` remains a placeholder.
- Git is still not initialized. No license was added.
- No CI, release, deployment, database, authentication, or telemetry was added.
- No package code, lifecycle script, installation command, or real global inventory was executed during verification.

## 1.4 Phase 4 update (2026-10-26)

Phase 4 added the application service, report renderer, interactive terminal app, non-interactive CLI, binary entry point, and offline demo runtime. The Phase 0–3 records above remain accurate for their dates.

Current state:

- Go module path remains provisional `depscope`.
- New packages: `internal/app` (shared terminal service contract and live wiring), `internal/render` (report documents, comparison, sanitization, color policy), `internal/tui` (Bubble Tea state machine, views, styles, input), `internal/cli` (commands, flags, exit codes, terminal detection), `internal/demo` (deterministic offline scenarios), and `cmd/depscope` (binary and signal handling).
- New direct dependencies: `github.com/charmbracelet/bubbletea v1.3.10` and `github.com/charmbracelet/lipgloss v1.1.0`, with their pinned indirect dependencies recorded in `go.mod` and `go.sum`. `go mod tidy` was run so the two direct imports are classified correctly.
- The canonical report schema remains `1.0.0` and unchanged. The renderer uses an internal `Document` model for display; it is not a new wire contract.
- Project and inventory results are still separate exported Go result types assembled from shared model types (D16 remains open).
- The CLI and TUI both consume `app.Service`. Neither duplicates engine, project, inventory, or risk logic.
- The offline demo is code-defined in `internal/demo`, uses demo providers and a fixed clock, and is activated by `depscope demo` or the TUI. It makes no network requests.
- No HTTP API or website code exists. `web/README.md` remains a placeholder.
- Git is still not initialized. No license was added.
- No CI, release, deployment, database, authentication, or telemetry was added.
- No package code, lifecycle script, installation command, or real global inventory was executed during verification.
- `go build ./...` fails in this environment because a broken empty `.git` directory exists at `/home/jangho/.git`, causing Go VCS stamping to run a failing `git status --porcelain` from the parent directory. `go build -buildvcs=false ./...` is the verified build command here. The project directory itself is not a Git repository; the parent directory was not modified.

## 1.5 Phase 5 update (2026-10-26)

Phase 5 added the production-oriented Go HTTP API and its binary. The Phase 0–4 records above remain accurate for their dates.

Current state:

- Go module path remains provisional `depscope`; the Go dependency set is unchanged from Phase 4.
- New packages: `internal/api` (configuration, request validation, error envelope, rate limiting, routing, middleware, handlers) and `cmd/depscope-api` (server entry point with JSON logging and graceful shutdown).
- Endpoints: `POST /api/v1/scan`, `POST /api/v1/compare`, and `GET /healthz`. The API is a transport adapter over `app.Service`; it does not resolve dependencies, match vulnerabilities, apply risk rules, or assemble reports itself.
- Responses for successful scans are the canonical `ScanReport` JSON validated through `internal/contract.ValidateReport`; no second report model or API-only wire schema was added. The schema remains `1.0.0`.
- The API exposes only single-package scans and comparisons. Project scans, global inventory, and the offline demo are not exposed.
- Process-local protections: request body limit, request timeout and context cancellation, non-blocking concurrency cap, per-client-IP token bucket, exact-origin CORS allowlist disabled by default, panic recovery, `Cache-Control: no-store`, and `X-Content-Type-Options: nosniff`.
- No authentication, persistence, database, stored history, telemetry, or multi-instance coordination exists. Rate limiting and concurrency are per process and keyed by the TCP peer address; `X-Forwarded-For` is not trusted.
- No provider or upstream scan behavior changed. Provider failures remain report checks/limitations with HTTP 200; HTTP errors are reserved for transport, validation, cancellation, timeouts, and internal failures.
- No website code exists. `web/README.md` remains a placeholder.
- Git is still not initialized. No license was added.
- No CI, release, deployment, database, authentication, or telemetry was added.
- No package code, lifecycle script, installation command, or real global inventory was executed during verification.
- `go build -buildvcs=false ./...` remains the verified build command because of the broken parent `.git` directory described in Phase 4.

## 2. Initial repository state

| Check | Result |
| --- | --- |
| Directory contents (`ls -la`) | Only `.` and `..` |
| Hidden or nested files (`find . -maxdepth 2 -mindepth 1`) | None |
| Git repository | Not initialized |
| Git branch | None |
| Git history | None |
| README or design documents | None |
| Source code | None |
| Tests | None |
| CI or release workflows | None |
| Dependency manifests or lockfiles | None |
| Installed project dependencies | None |

The parent directory contains unrelated projects. No neighboring directory was modified.

## 3. Git state

| Check | Result |
| --- | --- |
| `git status --short --branch` | `fatal: not a git repository` |
| `git log --oneline -15` | `fatal: not a git repository` |
| Global Git identity | Configured (values intentionally not recorded) |
| Global `init.defaultBranch` | Not configured |

No Git repository was initialized, and no commit, branch, remote, or tag was created during Phase 0.

## 4. Verified toolchain

The following versions were obtained by running the listed commands in the target directory.

| Tool | Command | Verified result |
| --- | --- | --- |
| Go | `go version` | `go1.26.8 linux/amd64` |
| Go environment | `go env GOTOOLCHAIN GOOS GOARCH GOPATH GOMODCACHE` | `auto`, `linux`, `amd64`, `/home/jangho/go`, `/home/jangho/go/pkg/mod` |
| Node.js | `node --version` | `v24.22.0` |
| npm | `npm --version` | `11.19.1` |
| pnpm | `pnpm --version` | `12.14.0` |
| Yarn | `yarn --version` | `1.22.22` |
| Bun | `bun --version` | `1.4.0` |
| Git | `git --version` | `2.53.0` |
| GNU Make | `make --version` | `4.4.1` |
| Docker | `docker --version` | `29.9.2` |
| npx | `npx --version` | `11.19.1` |
| TypeScript compiler | `tsc --version` | Not installed globally |
| npm registry configuration | `npm config get registry` | `https://registry.npmjs.org/` |
| Operating system | `uname -srm` | `Linux 7.0.0-38-generic x86_64` |

Notes:

- Go does not accept `--version`; the verified value comes from `go version`.
- Multiple JavaScript package managers are installed. The project has not chosen one.
- A globally installed `tsc` is not required for the future website, because a project-local TypeScript version is expected once the website is scaffolded.
- Docker availability does not imply that Docker is part of the approved architecture.

## 5. Dependency state

At Phase 0:

- No `go.mod`, `go.sum`, `package.json`, lockfile, or project-local dependency directory existed.
- No dependency was installed during Phase 0.
- `node_modules` did not exist inside the project. A `node_modules` directory exists in the parent directory but belongs to unrelated work and was not inspected or modified.
- No Go modules were downloaded or added for this project.
- The global npm registry is configured to the public npm registry; this is a configuration read, not proof of network reachability.

After Phase 1:

- `go.mod` and `go.sum` exist for module `depscope`.
- Direct dependency: `github.com/santhosh-tekuri/jsonschema/v6 v6.0.3`.
- Indirect dependency: `golang.org/x/text v0.14.0`.
- No Node package manifest, package manager selection, lockfile, or installed JavaScript dependency exists.
- No database, authentication, HTTP framework, TUI, or logging dependency was added.

After Phase 2:

- The Go dependency set is unchanged. The HTTP client, caching, retries, risk rules, and source adapters use only the Go standard library and the existing dependencies.
- No JavaScript dependency, manifest, or lockfile was added.
- No database, authentication, HTTP server framework, TUI, or logging dependency was added.
- The typosquat reference dataset is a tracked JSON file inside the repository, not a downloaded live list.

After Phase 3:

- The Go dependency set is unchanged. Project parsing and inventory use only the Go standard library, the shared engine, and existing dependencies.
- No JavaScript dependency, manifest, or lockfile was added.
- No database, authentication, HTTP server framework, TUI, or logging dependency was added.
- Project and inventory fixtures are tracked synthetic files; no real global package data, live scan data, or command output was recorded into them.

After Phase 4:

- Direct Go dependencies: `github.com/santhosh-tekuri/jsonschema/v6 v6.0.3`, `github.com/charmbracelet/bubbletea v1.3.10`, and `github.com/charmbracelet/lipgloss v1.1.0`.
- Indirect dependencies come from Bubble Tea and Lip Gloss (terminal rendering, input handling, color profiles, runewidth, `golang.org/x/sys`, and `golang.org/x/text`).
- No JavaScript dependency, manifest, or lockfile was added.
- No database, authentication, HTTP server framework, or logging dependency was added.
- Demo scenarios are code-defined and synthetic; no live response or real machine inventory was recorded into the repository.
- `go mod tidy` downloaded `golang.org/x/exp` into the module cache while resolving the dependency graph. It was not added to `go.mod`, and no DepScope command queried npm, PyPI, or OSV.

After Phase 5:

- The Go dependency set is unchanged. The HTTP API uses only the Go standard library (`net/http`, `log/slog`, `encoding/json`, `context`) and existing internal packages.
- No HTTP framework, router, rate-limiting library, authentication library, database driver, or observability dependency was added.
- No JavaScript dependency, manifest, or lockfile was added.

## 6. Network verification

No live requests were made to OSV.dev, the npm Registry, or PyPI in Phase 0, Phase 1, Phase 2, Phase 3, Phase 4, or Phase 5. Phase 2 and Phase 3 tests use Go `httptest` servers, sanitized in-process fixtures, synthetic project trees, and injected command runners. Phase 4 tests use injected services, in-memory terminal I/O, and deterministic demo providers. Phase 5 tests use `httptest` against the API handler with a fake `ScanService`; no real upstream provider is contacted. Therefore:

- Connectivity to any source remains unverified.
- Authentication requirements remain unverified, although the approved initial scope assumes public, unauthenticated access.
- Rate limits, terms, response formats, and required client identification remain to be confirmed before live operation.
- The opt-in live smoke test was compiled but not executed.
- No real global npm or pip inventory was executed during verification.
- The offline demo was executed through tests and the CLI without network access.
- `go mod tidy` fetched `golang.org/x/exp` from the Go module proxy while resolving build dependencies. This was a toolchain operation, not a DepScope scan request or a source query.
- No credentials were provided, requested, or stored.

## 7. Documentation and code added

Phase 0 documentation:

| File | Purpose |
| --- | --- |
| `README.md` | Project overview, approved stack, initial scope, and current status |
| `docs/architecture.md` | Components, boundaries, data flow, report concept, and open decisions |
| `docs/implementation-plan.md` | Provisional phase sequence, dependencies, exit criteria, and risks |
| `docs/repository-audit.md` | This verified state and toolchain record |
| `AGENTS.md` | Persistent instructions for future coding agents |

Phase 1 additions:

| Path | Purpose |
| --- | --- |
| `go.mod`, `go.sum` | Go module and pinned dependencies |
| `internal/model/` | Domain model, identity normalization, status derivation, semantic validation |
| `internal/engine/` | Provider and orchestration interfaces, configuration, limiter, cache |
| `internal/contract/` | Embedded schema compilation and combined schema/semantic validation |
| `schemas/` | Canonical report schema, embedded loader, and schema rules |
| `testdata/fixtures/` | Deterministic demo fixtures and fixture rules |
| `web/README.md` | Placeholder recording that the website is not implemented |
| `docs/contracts.md` | Implemented contract reference |
| `.gitignore` | Go, Node, environment, and editor ignores |

Phase 2 additions and changes:

| Path | Purpose |
| --- | --- |
| `internal/sources/npm/` | npm Registry adapter and provider tests |
| `internal/sources/pypi/` | PyPI JSON API adapter and provider tests |
| `internal/sources/osv/` | OSV.dev query adapter and provider tests |
| `internal/fetch/` | Shared JSON HTTP client with timeouts, body limits, bounded retries, and TTL cache integration |
| `internal/cache/` | Bounded in-memory TTL cache |
| `internal/engine/scanner.go` | Scan orchestrator and report assembly |
| `internal/engine/scanner_test.go` | Orchestrator tests with fake providers |
| `internal/engine/integration_test.go` | Full-path engine tests using real adapters against `httptest` servers |
| `internal/engine/risk.go` | Explainable severity and risk rules |
| `internal/engine/typosquat.go` | Damerau-Levenshtein typosquat detection |
| `internal/engine/reference/packages-v1.json` | Versioned embedded typosquat reference dataset |
| `internal/engine/interfaces.go` | Internal version-metadata and lifecycle-script types added; no wire schema change |
| `internal/live/` | Production scanner factory, wiring tests, and opt-in live smoke test |
| `docs/risk-rules.md` | Documented risk, confidence, typosquat, and failure rules |

Phase 3 additions and changes:

| Path | Purpose |
| --- | --- |
| `internal/project/project.go` | Project root detection, format and limit types, result model, orchestration |
| `internal/project/npm.go` | `package.json` and `package-lock.json` v2/v3 parsing, graph construction, unresolved-node handling |
| `internal/project/python.go` | `requirements.txt` parsing, includes, exact-pin resolution, unsupported-syntax detection |
| `internal/project/scan.go` | Shared batch scanning and project result assembly |
| `internal/project/project_test.go` | npm project tests including healthy, vulnerable, nested, missing, malformed, unsupported, and limit cases |
| `internal/project/python_test.go` | Python project tests including pinned, unpinned, conflicting, unsupported, include, and limit cases |
| `internal/inventory/inventory.go` | Installed package model, npm/pip listing, result assembly, limits, checks, and limitations |
| `internal/inventory/system.go` | Read-only `npm root -g` and `pip list` command execution with bounded output and injected runner |
| `internal/inventory/inventory_test.go` | Inventory scan tests with synthetic metadata and injected runners |
| `internal/inventory/system_test.go` | Command runner, output parsing, fallback, malformed output, and output-limit tests |
| `internal/engine/scanner.go` | Batch resolved-package scanning with bounded workers and per-package partial failures |
| `testdata/fixtures/projects/` | Synthetic npm and Python project trees |
| `testdata/fixtures/inventory/` | Synthetic installed npm and pip metadata |
| `README.md`, `docs/*.md`, `AGENTS.md` | Updated status, contracts, coverage, verification, and phase plan |

Phase 4 additions and changes:

| Path | Purpose |
| --- | --- |
| `cmd/depscope/main.go` | Binary entry point, terminal detection, signal handling, and exit code |
| `internal/app/app.go` | `Service` contract, package/project/compare requests, progress, comparison, demo scenario types |
| `internal/app/live.go` | Live service wiring and progress mapping for package, project, inventory, comparison, and demo scans |
| `internal/app/spec.go` | Package specification parsing for npm and PyPI |
| `internal/app/app_test.go` | Service tests for parsing, mode separation, cancellation, comparison preservation, and demo determinism |
| `internal/render/document.go` | Report documents and views for reports, project results, and inventory results |
| `internal/render/compare.go` | Side-by-side and stacked package comparison |
| `internal/render/sanitize.go` | Control and escape sequence removal for untrusted terminal text |
| `internal/render/normalize.go` | Sanitization of copied documents without mutating source reports |
| `internal/render/color.go` | Color mode parsing and `NO_COLOR`/`TERM=dumb` policy |
| `internal/render/theme.go` | Shared labels, badges, wrapping, and width helpers |
| `internal/render/render_test.go` | Formatting, status distinction, comparison, sanitization, and color tests |
| `internal/tui/model.go` | Bubble Tea model, screens, keyboard handling, cancellation, report views |
| `internal/tui/commands.go` | Scan commands, progress channel, and result messages |
| `internal/tui/views.go` | Menu, forms, loading, reports, comparison, error, help, and small-terminal views |
| `internal/tui/input.go` | Rune-based text field and plain-text layout helpers |
| `internal/tui/styles.go` | Optional Lip Gloss styling that becomes a no-op without color |
| `internal/tui/run.go` | Bubble Tea program construction |
| `internal/tui/model_test.go` | Menu, workflow, cancellation, progress, report view, comparison, demo, settings, and terminal-size tests |
| `internal/cli/cli.go` | Commands, global flags, exit codes, help, terminal detection |
| `internal/cli/cli_test.go` | Non-TTY, flag, command, color, demo, comparison, project, and inventory tests |
| `internal/demo/demo.go` | Three deterministic offline scenarios using the shared engine |
| `internal/demo/demo_test.go` | Demo contract validity, determinism, offline behavior, and scenario tests |
| `go.mod`, `go.sum` | Bubble Tea and Lip Gloss direct dependencies plus pinned indirect dependencies |
| `README.md`, `docs/*.md`, `AGENTS.md`, `testdata/fixtures/README.md` | Updated terminal scope, contracts, plan, audit, and fixture rules |

Phase 5 additions and changes:

| Path | Purpose |
| --- | --- |
| `internal/api/config.go` | Environment-driven configuration, defaults, bounds, and CORS origin normalization |
| `internal/api/errors.go` | Stable error codes, JSON error envelope, status mapping, JSON writer |
| `internal/api/request.go` | Strict JSON decoding, content-type enforcement, body limits, field validation, length caps |
| `internal/api/ratelimit.go` | Process-local token bucket keyed by TCP peer address with idle sweep and map cap |
| `internal/api/server.go` | Routes, middleware chain, handlers, CORS, concurrency gate, timeout, panic recovery, response types |
| `internal/api/config_test.go` | Defaults, environment overrides, invalid values, origin rules, bounds |
| `internal/api/server_test.go` | Handler tests for scans, comparisons, validation, limits, timeouts, cancellation, partial results, CORS, rate limiting, concurrency, panics, and health |
| `cmd/depscope-api/main.go` | API binary: JSON logging, server timeouts, signal handling, graceful shutdown |
| `docs/api.md` | Launch, endpoints, request fields, examples, errors, environment variables, limitations |
| `README.md`, `docs/architecture.md`, `docs/contracts.md`, `docs/implementation-plan.md`, `docs/repository-audit.md`, `AGENTS.md` | Updated API status, contracts, phase plan, verification, and audit |

The Next.js website does not exist yet.

## 8. Verification commands

### Executed for the Phase 0 audit

```text
pwd
ls -la
find . -maxdepth 2 -mindepth 1 -print
git status --short --branch
git log --oneline -15
git config --global user.name
git config --global user.email
git config --global init.defaultBranch
go version
go env GOTOOLCHAIN GOOS GOARCH GOPATH GOMODCACHE
node --version
npm --version
pnpm --version
yarn --version
bun --version
git --version
make --version
docker --version
npx --version
tsc --version
npm config get registry
uname -srm
find . -maxdepth 3 -type f
wc -l README.md AGENTS.md docs/*.md
```

Manifest and configuration existence checks were also run at that point; `go.mod`, `package.json`, common JavaScript lockfiles, `tsconfig.json`, Next.js and Tailwind configuration files, `.gitignore`, and `LICENSE` were all absent.

### Executed for Phase 1 verification

```text
go build ./...
go test ./...
go vet ./...
gofmt -l .
```

All passed; `gofmt -l .` produced no output. The contract tests validate the embedded schema, all demo fixtures, and negative cases including missing evidence, unknown severity, incomplete checks, and invalid status transitions.

Not run: frontend install, lint, type check, production build, and browser verification. No frontend exists. No live source request was made.

### Executed for Phase 2 verification

```text
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
go test -tags live -run '^$' ./internal/live/
```

All passed; `gofmt -l .` produced no output. The test run covered `internal/cache`, `internal/contract`, `internal/engine`, `internal/fetch`, `internal/live`, `internal/model`, `internal/sources/npm`, `internal/sources/osv`, and `internal/sources/pypi`. Default tests use `httptest` servers only and made no public network request.

Not run:

- `DEPSCOPE_LIVE_TESTS=1 go test -tags live -run TestLiveSmokeNPM ./internal/live/` was not executed because it requires public network access.
- No CLI, TUI, API, or website command exists to run.
- Frontend install, lint, type check, production build, and browser verification remain unavailable until Phase 6.

### Executed for Phase 3 verification

```text
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
go test -count=1 ./internal/project/
go test -count=1 ./internal/inventory/
go test -tags live -run '^$' ./internal/live/
```

All passed; `gofmt -l .` produced no output. The test run covered `internal/cache`, `internal/contract`, `internal/engine`, `internal/fetch`, `internal/inventory`, `internal/live`, `internal/model`, `internal/project`, `internal/sources/npm`, `internal/sources/osv`, and `internal/sources/pypi`. Project and inventory tests used synthetic fixtures and injected command runners only.

Not run:

- `DEPSCOPE_LIVE_TESTS=1 go test -tags live -run TestLiveSmokeNPM ./internal/live/` was not executed because it requires public network access.
- No real global npm or pip inventory command was executed. The inventory code path is covered through the injectable runner and synthetic metadata.
- No CLI, TUI, API, or website command exists to run.
- Frontend install, lint, type check, production build, and browser verification remain unavailable until Phase 6.

### Executed for Phase 4 verification

```text
gofmt -l .
go build -buildvcs=false ./...
go vet ./...
go test -count=1 ./...
go test -count=1 ./internal/app/
go test -count=1 ./internal/cli/
go test -count=1 ./internal/tui/
go test -count=1 ./internal/render/
go test -count=1 ./internal/demo/
```

All passed; `gofmt -l .` produced no output. The full test run covered `internal/app`, `internal/cache`, `internal/cli`, `internal/contract`, `internal/demo`, `internal/engine`, `internal/fetch`, `internal/inventory`, `internal/live`, `internal/model`, `internal/project`, `internal/render`, `internal/sources/npm`, `internal/sources/osv`, `internal/sources/pypi`, and `internal/tui`; `cmd/depscope` and `schemas` have no test files. Terminal tests used in-memory input/output and injected services.

The offline demo was additionally exercised through the built binary with output redirected, and its output was compared across repeated runs. A pseudo-terminal smoke test confirmed that the interactive app starts, renders the menu, accepts keyboard input, opens a form, returns with `esc`, and exits with `q`. Full human review of every interactive view is still recommended; automated state-machine and view tests cover the menu, workflows, cancellation, progress, report views, small-terminal guard, and non-TTY fallback.

Not run:

- `DEPSCOPE_LIVE_TESTS=1 go test -tags live -run TestLiveSmokeNPM ./internal/live/` was not executed because it requires public network access.
- No live npm, PyPI, OSV, or real global inventory command was executed.
- No browser, HTTP API, or website verification exists because those components do not exist.
- Plain `go build ./...` was not used as the verified command; it fails VCS stamping in this environment. `go build -buildvcs=false ./...` passed.

### Executed for Phase 5 verification

```text
gofmt -l .
go build -buildvcs=false ./...
go vet ./...
go test -count=1 ./...
go test -count=1 ./internal/api/
go test -race -count=1 -cover ./internal/api/
```

All passed; `gofmt -l .` produced no output. The full test run covered `internal/api`, `internal/app`, `internal/cache`, `internal/cli`, `internal/contract`, `internal/demo`, `internal/engine`, `internal/fetch`, `internal/inventory`, `internal/live`, `internal/model`, `internal/project`, `internal/render`, `internal/sources/npm`, `internal/sources/osv`, `internal/sources/pypi`, and `internal/tui`; `cmd/depscope`, `cmd/depscope-api`, and `schemas` have no test files. The race-enabled API run reported 85.8% statement coverage.

The built API binary was also smoke-tested on loopback:

- `GET /healthz` returned `200` with `{"status":"ok"}`, `Cache-Control: no-store`, and `X-Content-Type-Options: nosniff`.
- An invalid scan body returned `400` with the stable `invalid_request` code.
- An unknown field returned `400` with `unknown_field`.
- `GET /api/v1/scan` returned `405` with an `Allow` header.
- An unknown route returned `404` with `not_found`.
- `SIGTERM` triggered `shutdown requested` and `DepScope API stopped`, and the process exited cleanly.

Not run:

- `DEPSCOPE_LIVE_TESTS=1 go test -tags live -run TestLiveSmokeNPM ./internal/live/` was not executed because it requires public network access.
- No live npm, PyPI, OSV, or real global inventory request was executed through the API.
- No website install, lint, type check, production build, or browser verification exists because the website does not exist.
- No load, penetration, authentication, TLS-termination, or multi-instance test was performed. Rate limiting and concurrency limits were verified only in-process with `httptest`.

## 9. Existing work discovered

At Phase 0, no existing DepScope implementation was discovered; the workspace was greenfield and contained only the Phase 0 documentation. Phase 1 added the contracts and tests described above. Phase 2 added the engine and source adapters. Phase 3 added project scanning, global inventory, and batch engine scanning. Phase 4 added the application service, renderer, interactive terminal app, CLI, binary, and offline demo. Phase 5 added the HTTP API package, API binary, API tests, and API documentation. No third-party source code was imported, no package code was executed, no installed package was modified, and no external service was queried. The external Go modules are limited to the schema validator, Bubble Tea, Lip Gloss, and their pinned transitive dependencies.

## 10. Blockers and unresolved decisions

The following items block or constrain future phases. Full context is in `docs/architecture.md` Section 11.

| Blocker or decision | Why it matters |
| --- | --- |
| Git is not initialized; no remote URL or default branch is defined (D1) | Required before history, review, CI, or release work |
| No license has been selected (D2) | Required before public distribution |
| Go module path is provisional `depscope` (D3) | A canonical remote path is required before publishing or external imports |
| No JavaScript package manager or Next.js/Tailwind/Node version policy has been chosen (D4) | Website scaffolding would otherwise encode an unapproved choice |
| Report schema source of truth is resolved (D7) | Implemented and enforced; TypeScript generation is still needed in Phase 6 |
| Scan semantics are limited (D8) | Phase 3 covers npm lockfile v2/v3 graphs and exact-pinned Python requirements; Python transitives, workspaces, peer dependencies, and other formats remain open |
| Risk rules are defined but conventional and unvalidated (D9) | Scores are prioritization values, not probabilities; calibrating them is still open |
| API contract, CORS, and request limits are implemented locally; authentication and public deployment are still undefined (D10) | The API is a local, unauthenticated service; safe public operation still requires an authentication and deployment decision |
| Hosting and deployment targets are undefined (D13) | Required before launch |
| Project and inventory result integration is undefined (D16) | Results use separate Go types and are not yet schema-valid wire reports; the API deliberately does not expose them, and the website must not expose hand-built JSON for them |
| Machine-readable CLI output is undefined (D14) | The CLI is human-readable text only; scripting against JSON requires a future format decision |
| Live source connectivity, terms, and rate limits are unverified | The opt-in smoke test was not executed; live operation cannot be claimed |
| Typosquat reference coverage is a small curated seed | It is not a live top-N dataset; expanding or replacing it requires provenance review |

## 11. Phase boundaries

Phase 0 did not implement the engine, sources, scanning, CLI/TUI, API, website, demo runtime, CI, release, or deployment, and did not install dependencies or initialize Git.

Phase 1 did not:

- Implement dependency resolution or version selection.
- Implement npm, PyPI, or OSV clients or make any live request.
- Implement risk scoring or scan orchestration behavior.
- Build CLI, TUI, HTTP API, or website code.
- Implement dependency graph resolution, comparison features, or demo runtime.
- Add CI, release automation, deployment infrastructure, a database, or authentication.
- Initialize Git, commit, push, publish, or deploy.

Phase 2 did not:

- Resolve transitive dependencies or scan their vulnerability status; declared dependencies remain unresolved direct nodes.
- Read lockfiles, manifests, or installed environments.
- Execute package code or lifecycle scripts.
- Implement a CLI, TUI, HTTP API, website, or demo runtime.
- Make a live request to npm, PyPI, or OSV, or execute the opt-in live smoke test.
- Change the wire schema, install a JavaScript dependency, or add CI, release, deployment, database, or authentication.
- Initialize Git, commit, push, publish, or deploy.

Phase 3 did not:

- Implement recursive Python resolution or Python lockfile formats.
- Implement npm workspaces or peer dependency scanning.
- Support npm lockfile version 1, yarn, pnpm, bun, or shrinkwrap lockfiles.
- Substitute the latest registry version for an unresolved project version.
- Recursively walk `node_modules`, vendor directories, build output, or arbitrary filesystem paths, or follow symlinks outside the project root.
- Execute project code, package code, lifecycle scripts, package-manager installation commands, or installed packages.
- Produce a schema-valid wire report for project or inventory results (D16).
- Make a live request to npm, PyPI, or OSV, or execute the opt-in live smoke test.
- Implement a CLI, TUI, HTTP API, website, or demo runtime.
- Change the wire schema, install a JavaScript dependency, or add CI, release, deployment, database, or authentication.
- Initialize Git, commit, push, publish, or deploy.

Phase 4 did not:

- Add dependency resolution, vulnerability matching, risk rules, project parsing, or inventory detection to the CLI or TUI; all of these remain in the shared engine and application service.
- Execute package code, lifecycle scripts, installation commands, or installed packages. Lifecycle warnings remain inspection signals only.
- Mix demo data into live scans or fall back from a failed live scan to demo data.
- Add machine-readable JSON output (D14).
- Recursively resolve Python dependencies, support additional lockfiles, or implement npm workspaces or peer dependencies.
- Produce a schema-valid wire report for project or inventory results (D16).
- Make a live request to npm, PyPI, or OSV, or execute the opt-in live smoke test.
- Implement an HTTP API, website, database, authentication, telemetry, CI, release, or deployment.
- Change the wire schema or initialize Git, commit, push, publish, or deploy.

Phase 5 did not:

- Add dependency resolution, vulnerability matching, risk rules, project parsing, inventory detection, or report assembly to the API; all of these remain in the shared engine and application service.
- Introduce an API-specific report model or change the wire schema. Successful responses are canonical `ScanReport` values validated through `internal/contract`.
- Expose project scans, global inventory, the offline demo, or a machine-readable terminal mode (D14).
- Add authentication, authorization, persistence, stored history, telemetry, TLS termination, or multi-instance rate limiting.
- Implement request-level response caching, an upstream readiness check, or a public deployment configuration.
- Execute package code, lifecycle scripts, installation commands, or installed packages.
- Make a live request to npm, PyPI, or OSV, or execute the opt-in live smoke test.
- Implement a website, database, authentication, telemetry, CI, release, or deployment.
- Change the wire schema or initialize Git, commit, push, publish, or deploy.

Phases 2, 3, 4, and 5 are complete. The website is the next phase. Recursive dependency resolution and live verification remain deferred and have not begun.

## Phase 6 update (2026-10-26)

The website now exists under `web/`. The earlier audit entries above are historical snapshots and do not describe the current checkout.

- Stack: Node `v24.22.0`, pnpm `12.14.0`, Next.js `16.4.3`, TypeScript `5.9.3`, and Tailwind CSS `4.3.3`; exact JavaScript dependencies are pinned in `web/pnpm-lock.yaml`.
- Browser tests use Playwright `1.64.1` and the installed `/usr/bin/google-chrome`. Set `CHROMIUM_PATH` when the browser executable is elsewhere.
- The website uses generated TypeScript report types and schema validation, same-origin proxy routes to the Go API, live scan and comparison forms, offline synthetic demos, report exports, and a keyboard-accessible dependency path view.
- `DEPSCOPE_API_BASE_URL`, `DEPSCOPE_API_TIMEOUT_MS`, `DEPSCOPE_SITE_URL`, and optional `DEPSCOPE_REPOSITORY_URL` are documented in `web/README.md`. No repository remote URL is configured.
- `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, `go test -race ./...`, and `gofmt -l .` completed successfully. The `gofmt` command printed no files.
- `web/` lint, typecheck, 106 Vitest tests, contract check, production build, and 3 Playwright tests completed successfully. The browser scan test used a schema-valid intercepted API report; no live registry or OSV scan was run.
- A local request through the website's `/api/health` route returned `{"status":"ok"}` from the running Go API. Both local servers were stopped after this check.
- Git is initialized on `feature/phase-6-website`. The initial repository state is committed; no remote or default branch is configured. The Phase 6 changes are recorded in the current branch history.

Phase 7 recursive resolution and public-source live verification remain deferred. No CI, deployment, authentication, persistence, or telemetry was added.
