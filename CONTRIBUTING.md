# Contributing to DepScope

Thanks for helping improve DepScope. Issues, pull requests, additional test fixtures, and clearer explanations of findings are all welcome.

DepScope is a dependency security tool. Contributions must keep its central promise intact: **a result only looks clean when every check actually succeeded**, and confirmed evidence stays separate from heuristic signals. Read [risk and evidence rules](docs/risk-rules.md) and [data contracts](docs/contracts.md) before changing scan behavior.

## Before you start

- For a security problem **in DepScope itself**, follow [SECURITY.md](SECURITY.md) instead of opening a public issue.
- For a large feature or an architectural change, open an issue first. Several scope decisions are recorded as open items in [architecture decisions](docs/architecture.md) (D1–D16); do not silently resolve one in a pull request.
- For a bug, include the DepScope version (`depscope version`), the command or API request you ran, the affected ecosystem and package identity, and the report status you saw.

## Development setup

Requirements: Go 1.26.10, Node.js 24.22.0, pnpm 12.14.0, and a Chromium browser for the website's browser tests.

Run the Go checks from the repository root:

```sh
gofmt -l .              # must print nothing
go vet ./...
go build -trimpath ./...
go test -count=1 ./...
go mod verify
```

Run the website checks:

```sh
cd web
pnpm install --frozen-lockfile
pnpm check:contract
pnpm lint
pnpm typegen
pnpm typecheck
pnpm test
pnpm build
pnpm exec playwright install chromium
pnpm test:e2e
pnpm audit --prod --audit-level high
```

`pnpm install --frozen-lockfile` must not modify `pnpm-lock.yaml`; verify with `git status --short web/pnpm-lock.yaml`.

`pnpm typegen` generates the route types that declare `PageProps` and `LayoutProps` globally. Without them, `pnpm typecheck` fails with `Cannot find name 'PageProps'` on a clean checkout; `pnpm typecheck` also runs generation automatically through its `pretypecheck` hook.

pnpm 12 takes its operation lock under `$XDG_RUNTIME_DIR` (falling back to `/run/user/<uid>`) and writes its store there too. In a restricted environment where that directory exists but is not writable, the install fails with `ERR_PNPM_STORE_DIR_OPEN_OPERATION_LOCK … Read-only file system`, which is an environment problem and not a lockfile problem. Point both the runtime directory and the store at a writable location:

```sh
cd web
XDG_RUNTIME_DIR="$PWD/../.scratch-prep/xdg-runtime" \
  pnpm install --frozen-lockfile --store-dir "$PWD/../.scratch-prep/pnpm-store"
```

Use a directory that is ignored by Git (`.scratch-prep/` is already in `.gitignore`) so the store is never committed.

Run the local stack. Start the API with `go run ./cmd/depscope-api` (it listens on `127.0.0.1:8080`; liveness is `http://127.0.0.1:8080/healthz`). In another terminal run `cd web`, `cp .env.example .env.local`, and `pnpm dev`, then open `http://localhost:3000`. The site calls the API at `DEPSCOPE_API_BASE_URL`, which defaults to `http://127.0.0.1:8080`.

The offline demo needs no network access and is useful while developing:

```sh
go run ./cmd/depscope demo vulnerable
go run ./cmd/depscope demo list
```

## Change expectations

- Keep dependency resolution, vulnerability matching, risk decisions, and report assembly in the shared Go engine (`internal/engine`, `internal/project`, `internal/inventory`). The CLI, TUI, and HTTP API are adapters; they must not duplicate that logic.
- The CLI and TUI must obtain results through `app.Service`. The HTTP API consumes a narrow handler-level interface over the same service and validates every outgoing report with `internal/contract.ValidateReport`.
- Add deterministic tests with synthetic fixtures. The default test suite must not contact npm, PyPI, OSV, or any other public service.
- Preserve incomplete results and evidence. A failed, skipped, or unavailable check must never be represented as clean.
- Never install, import, or execute a package being scanned, and never run lifecycle scripts. Lifecycle scripts are inspection signals only.
- `schemas/scan-report.v1.schema.json` is canonical. Go types mirror it, and website types are generated from it. Run `go test ./internal/contract/` and `pnpm check:contract` after any contract-related change. Schema version changes require a new schema file and an explicit version bump.
- Escape untrusted text at the render boundary: terminal output must not pass through escape sequences, and website output must be HTML-safe.
- Never make color the only way to convey severity, status, finding kind, check status, or demo mode.
- Write all new source-code comments and docstrings in **Simplified Chinese**. Keep identifiers, API fields, UI copy, and public documentation in English (the two README files are intentionally bilingual).
- Do not add credentials, local environment files (`.env`, `.env.local`), generated build outputs, or machine-specific artifacts.
- Do not add new runtime dependencies, CI jobs, or deployment configuration without prior agreement in an issue.

## Commit and pull-request expectations

Describe the user-facing change, its effect on risk and coverage semantics, and the exact commands you ran. Include screenshots or a short recording for user-visible website or terminal changes. Do not claim that a scan is comprehensive or that a package is safe because data was missing.

Keep commits focused. Do not force-push shared branches, rewrite published history, or mix unrelated cleanups into a functional change.

## Third-party services and data

DepScope queries the npm Registry, PyPI, and OSV.dev. Respect their terms, rate limits, and identification requirements. Do not add fixtures containing real personal data, private package names, credentials, or vendor-specific internal information; synthetic data under `testdata/fixtures/` is the expected form.

## Licensing of contributions

This project is distributed under the [Apache License 2.0](LICENSE). By submitting a pull request you agree that your contribution is licensed under the same terms, unless it is clearly marked otherwise by the maintainer in advance. Do not paste in code from projects with incompatible licenses; if you adapt third-party code, keep its license notice and add it to [NOTICE](NOTICE).
