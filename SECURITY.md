# Security policy

## Reporting a vulnerability in DepScope

Please **do not** open a public issue for a vulnerability in DepScope itself. Avoid posting actionable exploit details in public channels, including issue trackers, discussions, and social media.

### Current reporting path

The repository `janghotan/depscope` was created privately on 2026-10-26. Its GitHub security features were probed on the same day; the results below are what the API actually returned, not what is intended:

| Feature | Probe result | Meaning |
| --- | --- | --- |
| Private vulnerability reporting | `PUT`/`GET repos/{owner}/{repo}/private-vulnerability-reporting` → `404 Not Found` | Not enabled through the API. Re-check after the first push. |
| Secret scanning | `PATCH` with `secret_scanning` → `422 Secret scanning is not available for this repository.` | Unavailable for this repository; likely a private-repository plan limitation. |
| Dependabot alerts | `PUT …/vulnerability-alerts` → `204 No Content`; `GET` → `204` | Enabling succeeded. Alert routing only starts once a dependency manifest is on the default branch, so expect activity after the first push. |
| Code scanning | `GET …/code-scanning/alerts` → `403 Code scanning is not enabled` | Not enabled; the CI workflow performs `govulncheck` instead. |
| Branch protection | `GET …/branches/main/protection` → `403 Upgrade to GitHub Pro or make this repository public` | Not available on a private repository under this plan. |

**A private reporting channel is therefore not verified as enabled yet**, and the maintainer must confirm one before public launch. Until then:

1. Use GitHub's **Report a vulnerability** button under the repository's **Security** tab if it is shown. This uses GitHub private vulnerability reporting and keeps the report private.
2. If that button is not available, contact the maintainer through the private channel where this repository was provided to you and ask for an acknowledged disclosure path before sending details.

Do not invent or rely on an unverified email address for security reports. When a private channel is confirmed, it will be documented in this file and in the repository's Security settings.

### Manual steps to finish the GitHub configuration

These settings cannot be changed through the currently authenticated API token and must be completed in the GitHub web interface:

1. **Private vulnerability reporting:** repository **Settings → Security** (the security and analysis area) → enable **Private vulnerability reporting**. The `PUT` API above returned `404`, so confirm the setting in the UI after the first push.
2. **Secret scanning:** the API reported it unavailable. If it is also absent in **Settings → Code security and analysis**, it requires a GitHub plan or public visibility; record the outcome in this file when it changes.
3. **Branch protection:** available only for public repositories or paid plans. Options are to make the repository public (a separate, explicit decision), upgrade the plan, or pin the repository's protection expectations in a maintainer note.

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
