# DepScope HTTP API

The Phase 5 API exposes the shared DepScope scan service over HTTP. It is a transport adapter: package identity validation, provider requests, vulnerability matching, risk rules, and report assembly all remain in the shared Go engine. Scan responses are canonical `ScanReport` values defined by [`../schemas/scan-report.v1.schema.json`](../schemas/scan-report.v1.schema.json) and validated through `internal/contract` before they are returned.

The API serves live scans only. It does not expose demo data, project scans, or global inventory results.

## Launch

From the repository root:

```sh
go build -o depscope-api ./cmd/depscope-api
DEPSCOPE_API_ADDR=127.0.0.1:8080 ./depscope-api
```

The binary listens on `127.0.0.1:8080` by default and logs JSON to stderr. Construction does not contact npm, PyPI, or OSV; network access happens only when a scan request is served. `SIGTERM` and interrupt trigger graceful shutdown, waiting up to `DEPSCOPE_API_SHUTDOWN_TIMEOUT` for in-flight requests before forcing the listener closed.

In this development environment, Go VCS stamping fails because of a broken `.git` directory above the repository. Use `go build -buildvcs=false -o depscope-api ./cmd/depscope-api` here. A normal checkout can use plain `go build`.

## Endpoints

| Method | Path | Request body | Success response |
| --- | --- | --- | --- |
| `GET` | `/healthz` | none | `200` with `{"status":"ok"}` |
| `POST` | `/api/v1/scan` | one package object | `200` with a canonical `ScanReport` object |
| `POST` | `/api/v1/compare` | `left` and `right` package objects | `200` with a comparison envelope |

All request and response bodies use JSON. Success responses carry `Content-Type: application/json; charset=utf-8`. All responses carry `Cache-Control: no-store` and `X-Content-Type-Options: nosniff`.

Query parameters are not accepted by the scan or compare endpoints. `/healthz` ignores query parameters because health probes commonly append them.

### `GET /healthz`

Returns only the process status. It does not call npm, PyPI, or OSV, and it does not disclose configuration, dependency versions, environment variables, or infrastructure details.

```json
{"status":"ok"}
```

A non-`GET` method returns `405`.

### `POST /api/v1/scan`

Request fields:

| Field | Type | Required | Rules |
| --- | --- | --- | --- |
| `ecosystem` | string | yes | Exact `npm` or `pypi` |
| `name` | string | yes | Canonical syntax for the ecosystem; at most 214 characters; no leading or trailing whitespace |
| `version` | string | no | Exact published version; at most 256 characters; only ASCII letters, digits, `.`, `_`, `+`, and `-` |

Omit `version` to resolve the ecosystem's latest published version. Version ranges, URLs, paths, and arbitrary expressions are not accepted.

Example:

```sh
curl -sS http://127.0.0.1:8080/api/v1/scan \
  -H 'Content-Type: application/json' \
  -d '{"ecosystem":"npm","name":"lodash","version":"4.17.21"}'
```

The response is the `ScanReport` object itself, not an envelope. It includes the schema version, data mode (`live`), scan status, target and resolved version, findings, per-check coverage, dependency graph, source references, and limitations. Partial coverage is returned as `status: "incomplete"` with the failed or unavailable checks and limitations intact. A report is never rewritten into a clean success and is never served from a response cache.

An `npm` name is lowercased and may use an `@scope/name` form. A PyPI name is lowercased and normalized to PEP 503 hyphen form. The response target reflects the normalized identity.

### `POST /api/v1/compare`

Request body:

```json
{
  "left":  {"ecosystem": "npm",  "name": "lodash",   "version": "4.17.21"},
  "right": {"ecosystem": "pypi", "name": "requests"}
}
```

Each side accepts the same fields and rules as the scan endpoint. `left` and `right` are required objects.

The comparison response is an API envelope that composes two canonical reports; it is not a second report schema:

```json
{
  "left": {
    "request": {"ecosystem": "npm", "name": "lodash", "version": "4.17.21"},
    "report": { "...canonical ScanReport..." }
  },
  "right": {
    "request": {"ecosystem": "pypi", "name": "requests"},
    "report": { "...canonical ScanReport..." }
  }
}
```

`request.version` is omitted when no version was supplied. If one side fails before producing a report, that side contains a generic `error` object instead of `report`, and the successful side is still returned with `200`. If both sides fail, the response is `502` with the standard error envelope. Internal failure text is logged by the server but never returned.

## Validation behavior

The API is strict by design:

- The body must contain exactly one JSON object. Empty bodies, arrays, scalars, malformed JSON, truncated JSON, and trailing JSON values are rejected.
- `Content-Type` must be `application/json` (parameters such as `charset=utf-8` are accepted).
- Only the documented fields are accepted. Unknown fields, including a `url` field, are rejected at both the top level and inside comparison sides.
- Field keys are case-sensitive. Duplicate keys are resolved by the standard library decoder, which keeps the last value; clients should not send duplicates.
- `null` is not accepted for required string fields.
- Package names must pass the shared model's ecosystem-specific validation. Names containing schemes, slashes outside npm scopes, spaces, control characters, or unsupported punctuation are rejected.
- Versions must be a single identifier. Slashes, colons, query characters, whitespace, and URL-shaped values are rejected before any provider request is built.
- Provider requests are constructed only from validated package identities and versions. The API never accepts an arbitrary fetch URL.

## Errors

Errors use one stable envelope:

```json
{"error":{"code":"invalid_request","message":"name is required"}}
```

Clients should branch on `code`, not on `message`.

| HTTP status | `code` | Meaning |
| --- | --- | --- |
| `400` | `invalid_json` | Body is empty, malformed, not an object, or contains more than one JSON value |
| `400` | `unknown_field` | An undocumented field is present |
| `400` | `invalid_request` | A documented field is missing or fails validation, or query parameters are present |
| `403` | `origin_not_allowed` | A CORS preflight comes from an origin outside the allowlist |
| `404` | `not_found` | No endpoint matches the path |
| `405` | `method_not_allowed` | The path exists but does not accept the method; `Allow` is set |
| `413` | `request_too_large` | The body exceeds `DEPSCOPE_API_MAX_BODY_BYTES` |
| `415` | `unsupported_media_type` | The content type is not `application/json` |
| `429` | `rate_limited` | The per-client token bucket is empty; `Retry-After` is set |
| `499` | `request_cancelled` | The request context was cancelled before a result was available (non-standard status used for observability) |
| `500` | `internal_error` | The scan service failed, panicked, or returned a report that failed contract validation; details are logged, not returned |
| `502` | `scan_failed` | Both comparison sides failed without a report |
| `503` | `server_busy` | The process is at its concurrency cap; `Retry-After` is set |
| `504` | `scan_timeout` | The scan or comparison did not finish within `DEPSCOPE_API_REQUEST_TIMEOUT` |

A successfully completed scan returns `200` even when its embedded status is `findings`, `incomplete`, or `failed`. Those statuses are part of the report contract, not HTTP transport failures.

## Configuration

Configuration is read from the environment at startup. Invalid values fail startup rather than falling back silently.

| Variable | Default | Rules |
| --- | --- | --- |
| `DEPSCOPE_API_ADDR` | `127.0.0.1:8080` | `host:port`; use loopback unless the API is intentionally exposed behind a network boundary |
| `DEPSCOPE_API_REQUEST_TIMEOUT` | `90s` | Positive Go duration; bounds each API request context |
| `DEPSCOPE_API_MAX_BODY_BYTES` | `16384` | 1024–1048576 bytes |
| `DEPSCOPE_API_MAX_CONCURRENCY` | `8` | 1–1024 simultaneous scan and compare requests |
| `DEPSCOPE_API_RATE_PER_MINUTE` | `60` | 1–1000000; token refill rate per client |
| `DEPSCOPE_API_RATE_BURST` | `10` | 1–100000 burst capacity per client |
| `DEPSCOPE_API_CORS_ORIGINS` | empty (disabled) | Comma-separated exact `http://` or `https://` origins; wildcards, credentials, paths, queries, and fragments are rejected |
| `DEPSCOPE_API_SHUTDOWN_TIMEOUT` | `15s` | Positive Go duration; graceful shutdown wait |

The shared engine uses its own bounded provider and scan timeouts (15 seconds per provider request and 2 minutes per scan by default). The API request deadline cancels the scan context, including in-flight provider requests. Keep the API request timeout below the engine scan timeout so the transport deadline determines the client-visible outcome.

The HTTP server also enforces a 5-second read-header timeout, 15-second request read timeout, 60-second idle timeout, and a write timeout of the request timeout plus 15 seconds.

## CORS

CORS is disabled by default. Only origins listed in `DEPSCOPE_API_CORS_ORIGINS` receive `Access-Control-Allow-Origin`; the exact request origin is echoed and `Vary: Origin` is set. Preflight responses allow `GET`, `POST`, and `OPTIONS` with the `Content-Type` header and cache the preflight for 600 seconds.

An `OPTIONS` preflight from an unlisted origin returns `403 origin_not_allowed`. A non-preflight request from an unlisted origin is served without CORS headers, so a browser cannot read the response. Wildcard origins are not supported. The API does not use cookies or credentialed CORS.

## Operational limitations

- **No authentication or authorization.** The API has no accounts, API keys, or per-user controls. Deploy it behind network isolation, a gateway, or another access-control layer; do not expose it directly to the public internet.
- **Process-local limits.** Rate limiting, concurrency limits, and the engine cache are in-memory and per process. Multiple instances do not share counters or cached upstream responses. A restart resets rate-limit state.
- **Client identity.** Rate limiting uses the TCP peer address. `X-Forwarded-For` and similar headers are deliberately not trusted. Behind a reverse proxy, configure the proxy to preserve client identity at the connection level or accept that clients share a bucket.
- **No response caching.** Every successful scan is freshly assembled. The shared engine may reuse bounded upstream metadata within its TTL, but the API does not cache or replay whole reports, so an unavailable check is never represented as a cached success.
- **No readiness dependency check.** `/healthz` reports process liveness only. It intentionally avoids upstream calls so probes do not consume provider rate limits; it cannot prove npm, PyPI, or OSV are reachable.
- **No project or inventory transport.** Phase 4 project and inventory results remain separate Go result types. They are not part of the v1 wire schema (D16) and are not exposed by this API.
- **No persistence or telemetry.** The API does not store requests, reports, or scan history, and it does not send telemetry. Server logs contain operational errors and panic stacks; they do not contain request bodies or report content.
- **No demo endpoint.** Demo scenarios are available only through the terminal surfaces. The API rejects non-live reports if a service implementation ever returns one.
- **Report size is not separately capped.** Only request bodies are bounded. Response size depends on upstream metadata and can grow with advisory and dependency counts.
- **Version semantics.** Passing `version` requests that exact release; it is not a range, tag, or constraint solver. Omit it to let the provider choose the latest release.
