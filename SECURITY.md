# Security policy

## Reporting a vulnerability in DepScope

Please **do not** open a public issue for a vulnerability in DepScope itself. Avoid posting actionable exploit details in public channels, including issue trackers, discussions, and social media.

### Current reporting path

The repository `https://github.com/janghotan/depscope` was published publicly on 2026-10-26, and its security configuration was re-probed immediately afterwards. The table records what the GitHub API actually returned:

| Feature | Verified state | Evidence |
| --- | --- | --- |
| Private vulnerability reporting | **Enabled** | `PUT repos/{owner}/{repo}/private-vulnerability-reporting` → `204`; `GET` → `{"enabled":true}` |
| Secret scanning and push protection | **Enabled** | Repository `security_and_analysis` reports `secret_scanning: enabled` and `secret_scanning_push_protection: enabled` |
| Dependabot alerts | **Enabled** | `PUT …/vulnerability-alerts` → `204`; `GET` → `204` |
| Code scanning | Not enabled | `GET …/code-scanning/alerts` → `404 no analysis found`; the CI workflow runs `govulncheck` instead |
| Branch protection on `main` | Not configured | `GET …/branches/main/protection` → `404 Branch not protected` |

To report a vulnerability in DepScope itself:

1. Use GitHub's **Report a vulnerability** button under the repository's **Security** tab. This opens a private GitHub Security Advisory and keeps the report private.
2. Do not open a public issue, and do not post exploit details in any public channel.

Do not include secrets, private source code, personal data, or third-party inventories. When you are ready to discuss a fix publicly, coordinate disclosure in the advisory.

### Manual steps still pending

These items are not automated by this repository and remain with the maintainer:

1. **Branch protection** on `main` (required reviewers, required status checks including the `CI` workflow, signed commits) — configure under **Settings → Branches → Add branch protection rule**.
2. **Repository social preview** — upload `assets/social/depscope-social-preview.png` under **Settings → General → Social preview**. The GitHub API does not expose this setting (`PUT …/social-preview` → `404`).
3. **Code scanning** — optionally enable GitHub's default CodeQL setup under **Settings → Code security and analysis**; the Go job already runs `govulncheck@v1.9.0`.

Please include:

- the affected version or commit;
- the command, API request, or website path involved;
- a description of the impact and the conditions required to trigger it;
- the minimum reproducible steps.

Do not include secrets, private source code, personal data, or scanned customers' package inventories. Redact tokens and internal hostnames.

The maintainer will try to acknowledge a report promptly, agree on a disclosure timeline, and credit reporters who request it. No response-time guarantee is claimed.

## Scope

In scope:

- the shared Go engine (`internal/engine`, `internal/project`, `internal/inventory`, `internal/sources`, `internal/fetch`, `internal/cache`);
- the terminal application and CLI (`internal/tui`, `internal/cli`, `cmd/depscope`);
- the local HTTP API (`internal/api`, `cmd/depscope-api`);
- the website (`web/`);
- the report contract and its validation (`schemas/`, `internal/contract`, `internal/model`);
- release configuration and CI workflows.

Out of scope for this repository:

- vulnerabilities in packages that DepScope scans — report those through that package's own advisory channel;
- inaccuracies or delays in npm, PyPI, or OSV.dev data — report those to the relevant upstream project;
- findings produced by DepScope's own heuristics — those are review signals, not defects in the scanned package.

## What DepScope does not do

DepScope is not comprehensive malware detection, exploitability analysis, or a guarantee that a dependency is safe. Its results depend on supported manifests, the metadata returned by npm and PyPI, OSV.dev coverage, and a small heuristic reference dataset. It does not execute packages, inspect installed package code, or detect maintainer account takeover. Sources, heuristics, and coverage limits are documented in [risk and evidence rules](docs/risk-rules.md).

## Operating the API safely

The HTTP API is local and unauthenticated. It is intended for loopback use or deployment behind a separately secured network boundary. Rate limiting and concurrency limits are process-local and keyed by TCP peer address; forwarding headers are not trusted. Do not expose the API directly to the public internet, and do not run it as a multi-tenant service without adding authentication, isolation, and resource accounting.
