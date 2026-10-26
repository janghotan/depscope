# DepScope website

The Next.js App Router website presents the shared Go scan report contract. Live requests pass through same-origin server routes to the Go HTTP API. Browser code does not call npm, PyPI, or OSV and contains no security rules.

## Requirements

- Node.js 24.x
- pnpm 12.x
- The Go API from the repository root for live scans and comparisons
- Playwright-managed Chromium for browser tests; set `CHROMIUM_PATH` only when using a custom browser executable

Versions are pinned in `package.json`, `pnpm-lock.yaml`, and the repository `.nvmrc`.

## Start locally

In one terminal, start the API from the repository root:

```sh
go run ./cmd/depscope-api
```

In a second terminal:

```sh
cd web
pnpm install --frozen-lockfile
cp .env.example .env.local
pnpm dev
```

Visit `http://localhost:3000`. The demo pages work offline. Live requests need the Go API at `http://127.0.0.1:8080` by default. You can configure a different address using the server-only `DEPSCOPE_API_BASE_URL` value in `.env.local`.

pnpm 12 keeps its store operation lock under `$XDG_RUNTIME_DIR` (falling back to `/run/user/<uid>`). If that directory exists but is not writable, `pnpm install --frozen-lockfile` fails with `ERR_PNPM_STORE_DIR_OPEN_OPERATION_LOCK … Read-only file system` even when `--store-dir` points elsewhere. Redirect the runtime directory to a writable, Git-ignored location:

```sh
cd web
XDG_RUNTIME_DIR="$PWD/../.scratch-prep/xdg-runtime" \
  pnpm install --frozen-lockfile --store-dir "$PWD/../.scratch-prep/pnpm-store"
```

A successful frozen install must leave `pnpm-lock.yaml` unchanged; check with `git status --short web/pnpm-lock.yaml`.

For a production build, run `pnpm build` and then `pnpm start` from `web/`. Set the public site origin in `DEPSCOPE_SITE_URL` at build time so absolute Open Graph image metadata uses the deployed origin. The development default is `http://localhost:3000`.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `DEPSCOPE_API_BASE_URL` | `http://127.0.0.1:8080` | Go API origin used only by server-side proxy routes. Must be an origin without credentials, query, or fragment. |
| `DEPSCOPE_API_TIMEOUT_MS` | `100000` | Per-request proxy timeout; accepted range is 1000–120000 ms. |
| `DEPSCOPE_SITE_URL` | `http://localhost:3000` | Public site origin used as the metadata base; set this to the deployed HTTPS origin at build time. |
| `DEPSCOPE_REPOSITORY_URL` | unset | Optional public HTTPS source repository link. The site shows its contribution guide when unset or invalid. |

These values are read server-side. Do not rename them with a `NEXT_PUBLIC_` prefix. The site has no authentication, stored scan history, analytics, or telemetry. Same-origin proxy requests do not require enabling Go API CORS for the visitor's browser.

## Checks

Run from `web/`:

```sh
pnpm lint
pnpm typecheck
pnpm test
pnpm check:contract
pnpm build
pnpm exec playwright install chromium
pnpm test:e2e
```

Vitest tests use deterministic fixtures and mocked fetches. Playwright starts the built Next.js production server and uses the Chromium executable installed by Playwright; its live scan journey stubs only the browser-to-site response with a schema-valid report. The demo browser test verifies that it sends no scan or comparison request; the shared header may still check the same-origin health route. Go API provider behavior remains covered by the repository's offline Go tests.

## Scope and limitations

- The site exposes single-package npm/PyPI scans and two-package comparisons only. It does not display project scan or inventory results because those are not part of the v1 API report contract.
- Dependency resolution and risk assessment remain in the Go engine. Scan coverage depends on the registry and OSV checks that actually complete; missing or failed checks stay incomplete.
- The browser API is a same-origin server proxy. Its request and response bodies are bounded, reports are schema-validated, responses are not cached, and upstream failure is shown as an error rather than demo data.
- Demo reports are deterministic synthetic fixtures, labelled as demo data, and isolated from live requests.
- The Go API defaults to loopback and is unauthenticated. Public hosting and deployment security are not configured by this project.
- No remote repository is configured in this checkout and no public site has been deployed. Repository links appear only when `DEPSCOPE_REPOSITORY_URL` is set.
- The project is distributed under the Apache License 2.0 (see the repository root `LICENSE`); the copyright notice names 2026 Jangho Tan.
- Brand and documentation assets shared with the repository READMEs live in `assets/` at the repository root; see `assets/README.md`.
