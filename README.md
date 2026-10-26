# DepScope

**Dependency risk, explained.**

DepScope scans npm and PyPI package releases and presents vulnerability evidence, review heuristics, and missing coverage in a terminal, local HTTP API, and website. It reads package metadata and advisory data; it never installs or executes the packages it scans.

> **Project status:** The CLI, Go engine, local API, and website are implemented. Public releases and redistribution are blocked until a license and canonical Go module path are selected. No hosted service is provided.

![DepScope terminal demo showing a synthetic vulnerability report](docs/assets/terminal-demo.png)

*Captured from `depscope demo vulnerable`. The report is deterministic synthetic demo data, not a live package scan.*

## Try the offline demo

Requirements: Go 1.26.10 or later.

```sh
go run ./cmd/depscope demo vulnerable
```

This command makes no network requests. Other scenarios are `clean` and `install-scripts`; list them with `go run ./cmd/depscope demo list`.

## Quick start

Build and run the terminal CLI from a checkout:

```sh
go build -o bin/depscope ./cmd/depscope
./bin/depscope scan lodash@4.17.21
```

Live scans contact the npm Registry or PyPI and OSV.dev. You can also start the interactive terminal app with `./bin/depscope`, scan the current project with `./bin/depscope project`, or inspect the full command list with `./bin/depscope help`.

The local API and website can be started in separate terminals:

```sh
go run ./cmd/depscope-api
```

```sh
cd web
pnpm install --frozen-lockfile
cp .env.example .env.local
pnpm dev
```

Open <http://localhost:3000>. The API listens on `127.0.0.1:8080` by default; check it with `curl -fsS http://127.0.0.1:8080/healthz`. Website configuration is server-side and documented in [web/README.md](web/README.md). The offline web demo works without the API.

## Installation

### Build from source

Install Go 1.26.10 or later, check out this repository, then build:

```sh
go build -trimpath -o bin/depscope ./cmd/depscope
go build -trimpath -o bin/depscope-api ./cmd/depscope-api
```

For source builds, use a Go-supported platform. DepScope's verified release matrix targets Linux, macOS, and Windows on `amd64` and `arm64`; CI cross-compiles each target. Cross-compilation verifies that binaries build, not that every target has been runtime-tested.

### Release binaries

When maintainers publish a tagged release, `checksums.txt` will contain SHA-256 checksums for separate `depscope_cli_<version>_<os>_<arch>` and `depscope_api_<version>_<os>_<arch>` archives. Choose the CLI archive for everyday use and the API archive only if you need the local HTTP service. Verify your downloaded archive before extracting it:

```sh
sha256sum --check checksums.txt --ignore-missing
```

On macOS, use `shasum -a 256 --check checksums.txt`; on Windows PowerShell, compare `(Get-FileHash .\depscope_*.zip -Algorithm SHA256).Hash` with the corresponding entry in `checksums.txt`. Use only the archive for your operating system and architecture. No release is published yet; binary distribution must wait for the license and canonical module path decisions.

Extract a Linux or macOS archive with `tar -xzf <archive>.tar.gz`, then place the binary from its extracted folder on your `PATH` (for example, `install <folder>/depscope "$HOME/.local/bin/depscope"`). On Windows, use `Expand-Archive <archive>.zip <destination>` in PowerShell and add the extracted folder to `PATH`. The API is distributed separately from the CLI.

## What it scans

- **Single releases:** npm and PyPI package names, optionally pinned to one exact version.
- **Project dependencies:** npm `package.json` with `package-lock.json` version 2 or 3; Python `requirements.txt` with exact `==` or `===` pins. npm lockfiles provide a dependency graph. Python project scans cover direct pins only.
- **Global inventory:** top-level global npm packages and the selected Python interpreter's default pip environment, read-only.
- **Risk signals:** version-matched OSV advisories, npm lifecycle-script declarations, and similarity against a small versioned package-name reference dataset.
- **Surfaces:** Go CLI and interactive terminal app, a local Go HTTP API, and a Next.js website for single-package scans and comparisons.
- **Offline demo:** deterministic synthetic reports shared by the CLI and curated website examples.

Unsupported formats and unavailable checks are reported as incomplete or failed; they are never silently treated as clean. Project and inventory results are not exposed through the API or website because they do not yet have an approved wire representation.

## Example report

The excerpt below is from the `vulnerable` **demo fixture**. Package names, advisory identifiers, findings, and timestamps are synthetic.

```text
PACKAGE REPORT
npm:@depscope-demo/vulnerable-package
Resolved version 2.0.0

  Data mode            DEMO
  Scanned at           2026-01-01T00:00:00Z

ASSESSMENT
  Status               [FINDINGS]
  Completed checks     6 of 6 succeeded
  Confirmed findings   2
  Heuristic warnings   0
  Highest severity     critical
```

Run `go run ./cmd/depscope demo vulnerable` for the full synthetic output. No real scan response is included in this README.

## Risk methodology

DepScope separates **confirmed** vulnerability findings from **heuristic** review signals. Confirmed findings require a version match from OSV.dev plus advisory evidence and a reference. Heuristics include declared npm lifecycle scripts and close package-name matches against a small, versioned seed list. A heuristic is a reason to review, not proof of malicious behavior.

Severity is normalized from advisory metadata and CVSS values where available. `unknown` is not safe. Numeric risk values are deterministic prioritization scores, not probabilities of exploitation or maliciousness. Findings retain evidence, source references, and remediation details when provided by the source.

Every report includes per-check outcomes, sources, and limitations. A missing, skipped, unavailable, or failed check prevents a `clean` status. See [risk and evidence rules](docs/risk-rules.md) for the exact rules and coverage boundaries.

## API

The unauthenticated API is intended for local use or a separately secured network boundary. It serves live reports from the shared Go engine and does not store scan history.

```sh
curl -sS http://127.0.0.1:8080/api/v1/scan \
  -H 'Content-Type: application/json' \
  -d '{"ecosystem":"npm","name":"lodash","version":"4.17.21"}'
```

Endpoints are `GET /healthz`, `POST /api/v1/scan`, and `POST /api/v1/compare`. The API applies request and body limits, timeouts, bounded concurrency, process-local rate limiting, and optional exact-origin CORS. The website calls it through same-origin server routes. See [API configuration and response semantics](docs/api.md).

## Architecture

```text
CLI / TUI ─┐
           ├── Go application service ── shared scan engine ── npm / PyPI / OSV
HTTP API ──┘                                  │
                                              └── canonical ScanReport v1
                                                       │
Website ── same-origin server proxy ──────────────────┘
```

The Go engine owns package resolution, source matching, risk evaluation, and report assembly. The API and terminal are adapters; the website consumes the canonical JSON Schema and generated TypeScript types. Demo mode uses a separate deterministic engine path and cannot fall back from a failed live scan. See [architecture](docs/architecture.md) and [contracts](docs/contracts.md).

## Local development and checks

Requirements: Go 1.26.10, Node.js 24.22.0, pnpm 12.14.0, and a Chromium browser for browser tests.

```sh
gofmt -l .
go test -count=1 ./...
go vet ./...
go build -trimpath ./...
go test -race -count=1 ./...
```

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

See [contribution guidance](CONTRIBUTING.md) for the full local workflow and [release procedure](docs/release.md) for packaging and maintainer gates.

## Security limitations and reporting

DepScope is not comprehensive malware detection, exploitability analysis, or a guarantee that a package is safe. Its results depend on supported manifests, metadata returned by npm/PyPI, OSV coverage, and a small heuristic reference dataset. It does not execute packages, inspect installed package code, or detect maintainer account takeover. The API has no authentication, and no hosted API or website deployment exists.

Do not submit secrets or private source code. For suspected vulnerabilities in DepScope itself, avoid posting actionable exploit details publicly. The repository has no configured private security contact yet; see [SECURITY.md](SECURITY.md) for the maintainer action required before public launch.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md), [architecture](docs/architecture.md), and [risk rules](docs/risk-rules.md) before changing behavior. Tests use synthetic fixtures and must not make public network requests by default. All newly written source-code comments and docstrings must be in Simplified Chinese; identifiers, UI text, and public documentation remain in English.

## License

No license has been selected or added. The project is not yet licensed for redistribution. A maintainer must choose and approve a license before publishing binaries or describing the project as legally open source. Do not add a license without that decision.
