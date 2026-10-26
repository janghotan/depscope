# Security policy

## Reporting a vulnerability in DepScope

Please **do not** open a public issue for a vulnerability in DepScope itself. Avoid posting actionable exploit details in public channels, including issue trackers, discussions, and social media.

### Current reporting path

The repository `janghotan/depscope` was created privately on 2026-10-26. During preparation, the private vulnerability reporting endpoint (`repos/{owner}/{repo}/private-vulnerability-reporting`) returned `404 Not Found` for it, and secret scanning was reported as unavailable. So **a private reporting channel is not verified as enabled yet**; the maintainer must confirm it before public launch. Until then:

1. Use GitHub's **Report a vulnerability** button under the repository's **Security** tab if it is shown. This uses GitHub private vulnerability reporting and keeps the report private.
2. If that button is not available, contact the maintainer through the private channel where this repository was provided to you and ask for an acknowledged disclosure path before sending details.

Do not invent or rely on an unverified email address for security reports. When a private channel is confirmed, it will be documented in this file and in the repository's Security settings.

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
