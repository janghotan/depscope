# Test fixtures

All files under `testdata/fixtures/demo/` are synthetic demonstration fixtures. They are not real scan results, do not describe real vulnerabilities, and must never be presented as live security data.

Every demo fixture:

- uses `dataMode: "demo"` and a non-empty `DEMO DATA` notice;
- uses only `provider: "demo"` source references;
- uses clearly synthetic package names containing `depscope-demo`;
- uses `DEMO-` identifiers instead of real advisory IDs;
- omits URLs rather than inventing source links;
- uses fixed timestamps for deterministic output.

The fixtures exercise clean, findings, and incomplete report states. Live scan data must never be written into this directory. If later phases add recorded provider responses, they must use a separate, explicitly named directory and retain provenance; they must not be mixed with synthetic demo data.

## Demo runtime scenarios

The offline demo activated by `depscope demo [scenario]`, `depscope demo list`, and the TUI demo picker is defined as deterministic code in `internal/demo`, not as recorded responses in this directory. The scenarios are:

| ID | Purpose |
| --- | --- |
| `vulnerable` | Confirmed critical and medium synthetic advisories with fixed versions and risk scores |
| `install-scripts` | Declared `preinstall`/`postinstall` commands and a deprecation signal; commands are never executed |
| `clean` | No advisories, no install scripts, and complete successful checks |

Every runtime scenario uses `dataMode: "demo"`, the shared demo notice, demo-only sources, synthetic `@depscope-demo/...` package names, `DEMO-` advisory identifiers, a fixed clock (`2026-01-01T00:00:00Z`), and the shared engine and risk rules. The demo scanner rejects live requests and the live scanner rejects demo requests, so scenario data cannot enter a live scan.

Changes to runtime scenario data must preserve the contract rules above and the determinism tests. Runtime scenarios must not be produced by recording live npm, PyPI, or OSV responses.

## Project fixtures

Files under `testdata/fixtures/projects/` are synthetic project trees used by `internal/project` tests. They are not copies of real projects and do not describe real vulnerabilities.

Every project fixture:

- uses clearly synthetic package names such as `safe-pkg`, `vulnerable-pkg`, `hidden-pkg`, and `@scope/scoped-pkg`;
- uses fixed versions and, where applicable, fixed lockfile content;
- covers one scenario per directory, including healthy trees, vulnerable transitive dependencies, missing lockfiles, malformed lockfiles, unsupported lockfile versions, nested dependencies, conflicts, unpinned requirements, and unsupported requirement syntax;
- is read-only test input; tests must not execute project code, install dependencies, or modify the fixture tree.

These fixtures must not be mixed with real project files, live lockfiles, or recorded provider responses.

## Inventory fixtures

Files under `testdata/fixtures/inventory/` are synthetic installed-metadata fixtures used by `internal/inventory` tests.

Every inventory fixture:

- uses clearly synthetic package names such as `safe-pkg`, `vulnerable-pkg`, `hidden-pkg`, and `@scope/scoped-pkg`, plus `Safe_Pkg` to exercise normalization;
- uses fixed versions and fixed metadata;
- simulates top-level npm global packages and a pip `list --format=json` document;
- must never be produced by running a real global inventory command into the fixture directory.

Tests use injected command runners and these fixtures. They must not modify installed global packages, invoke installed package code, or write real machine inventory data into the repository.
