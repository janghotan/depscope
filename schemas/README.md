# Scan report schema

`scan-report.v1.schema.json` is the canonical, versioned contract for a DepScope scan report and for the public API response body. It is shared by the Go backend and the future TypeScript frontend.

- Schema ID: `urn:depscope:schema:scan-report:1.0.0`
- Report version: `1.0.0`
- JSON field style: camelCase
- Draft: JSON Schema 2020-12

## Rules

1. This file is the source of truth. Do not hand-copy field definitions into Go or TypeScript.
2. Go types live in `internal/model/report.go` and are validated against this schema by `internal/contract`.
3. The TypeScript frontend does not exist yet. When it is scaffolded, its types must be generated from this schema, or generated types must be checked against it in CI.
4. Any field addition, removal, or semantic change requires a schema version decision. `additionalProperties` is false, so schema changes are breaking by default.
5. A `clean` status is valid only when at least one check exists, every check succeeded, there are no findings, and at least one source is referenced. Missing checks or unavailable, skipped, or failed checks require `incomplete` or another non-clean status.
6. Reports with `dataMode: demo` require a non-empty `demoNotice` and may reference only `provider: demo` sources.
7. Confirmed findings require a resolved version and at least one advisory evidence item.

Project scans and global inventory results are not yet represented by this schema. They expose separate Go result types assembled from shared model types; see D16 in `../docs/architecture.md` and Section 6.2–6.3 of `../docs/contracts.md`. Do not serialize them as ad hoc JSON or silently extend `1.0.0`.

## Validation

The schema is embedded into the Go binary through `schemas/embed.go`. Run:

```sh
go test ./internal/contract/
```

The contract tests validate all demo fixtures, reject malformed documents, and verify the schema ID and version constants.
