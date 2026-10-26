# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

Existing Phase 6 scaffold: pnpm 12.14.0, Next.js 16.4.3 (App Router), React 19.3.0, TypeScript 5.9.3, Tailwind CSS 4.3.3, Node 24. Exact versions are pinned in the manifest and lockfile.

## Users

Primary: developers checking one npm or PyPI package before adding or upgrading it, often while deciding between two candidate packages. Secondary: security reviewers evaluating whether DepScope's evidence and stated limitations are trustworthy.

## Product Purpose

DepScope correlates package identity and version with vulnerability data from OSV.dev, npm, and PyPI, then presents explainable findings, successful and failed checks, sources, and limitations. Success means a developer can see what is known, what is heuristic, what could not be checked, and what to do next without mistaking missing coverage for safety.

## Positioning

One shared Go engine and one versioned report contract serve the terminal and the website. The website presents the same canonical `ScanReport` the terminal renders; it does not re-resolve versions, re-match advisories, or invent a risk score. Confirmed advisories, heuristic signals, successful checks, and incomplete coverage remain visibly distinct.

## Operating Context

The Phase 6 website runs against the local, unauthenticated HTTP API. Live scans require the API and its upstream npm/PyPI/OSV connectivity. The offline demo must work without credentials or network access and must be visibly labelled `DEMO DATA`. Project scans, global inventory, accounts, and stored history are not part of this surface.

## Capabilities and Constraints

- Supported ecosystems: npm and PyPI only.
- Supported workflows: scan one package, compare two packages, and view deterministic offline demo reports.
- Report fields and enums come from `schemas/scan-report.v1.schema.json` version `1.0.0`; TypeScript types must be generated from or checked against it.
- The API returns HTTP `200` with report statuses such as `findings`, `incomplete`, and `failed`; these are scan outcomes, not transport success.
- A missing, failed, or unavailable check must never read as clean.
- No project scanning, global inventory, authentication, persistence, telemetry, or deployment is part of this phase.
- Demo reports use synthetic package names, `DEMO-` advisory identifiers, and fixed timestamps. They must remain unmistakable.
- Untrusted advisory text and URLs must be rendered safely; the site must not use raw HTML injection.

## Brand Commitments

The name is DepScope. The voice is precise, calm, and explicit: it reports evidence and uncertainty rather than promising safety. Severity, finding kind, status, and demo mode must never rely on color alone. Avoid a generic SaaS landing page, a fake terminal skin, dashboard overload, and a playful or gamified tone.

## Evidence on Hand

- Canonical schema: `schemas/scan-report.v1.schema.json`.
- Deterministic demo reports: `testdata/fixtures/demo/`.
- Implemented API contract: `docs/api.md`.
- No testimonials, customers, benchmarks, pricing, security certifications, or adoption claims exist. Do not fabricate them.

## Product Principles

1. Show the evidence, not a verdict alone.
2. Never let missing coverage look like safety.
3. Keep terminal and web results consistent by consuming the shared contract.
4. Make the next action obvious without hiding uncertainty.
5. Treat every remote string as untrusted.

## Accessibility & Inclusion

Use semantic HTML, keyboard-operable controls, visible focus, readable contrast, and text labels for severity, status, finding kind, and demo mode. Layout must remain usable on narrow screens and at 200% zoom.
