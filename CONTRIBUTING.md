# Contributing to DepScope

Thank you for helping improve DepScope. The project is under active development; its current scan limits and unresolved license status are described in the [README](README.md).

## Development setup

Install Go 1.26.10 or later, Node.js 24.22.0, and pnpm 12.14.0. From the repository root:

```sh
go test -count=1 ./...
go vet ./...
go build -trimpath ./...
```

For website work:

```sh
cd web
pnpm install --frozen-lockfile
pnpm check:contract
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm exec playwright install chromium
pnpm test:e2e
```

Start the API with `go run ./cmd/depscope-api`. It listens on `127.0.0.1:8080`; its liveness endpoint is `http://127.0.0.1:8080/healthz`. In another terminal, run `cd web`, `cp .env.example .env.local`, and `pnpm dev`; open `http://localhost:3000`. The site uses the API at `DEPSCOPE_API_BASE_URL`, defaulting to `http://127.0.0.1:8080`.

## Change expectations

- Keep dependency analysis, risk decisions, and report assembly in the shared Go engine.
- Add deterministic tests with synthetic fixtures. The default test suite must not contact public package or vulnerability services.
- Preserve incomplete results and evidence; missing checks must not be represented as clean.
- Never install, import, or execute packages being scanned, or run lifecycle scripts.
- Keep `schemas/scan-report.v1.schema.json` canonical. Generate website types from the schema and run both Go contract tests and `pnpm check:contract` after contract-related changes.
- Write all new source-code comments and docstrings in Simplified Chinese. Keep identifiers, API fields, UI copy, and public documentation in English.
- Do not add credentials, local environment files, generated build outputs, or machine-specific artifacts.

## Pull requests

Describe the user-facing change, its risk and coverage impact, and the exact checks you ran. Include screenshots for user-visible website changes. Avoid claiming that a scan is comprehensive or that a package is safe based on missing data. A repository remote and issue tracker have not been configured yet, so use the hosting project's contribution channel once maintainers establish it.
