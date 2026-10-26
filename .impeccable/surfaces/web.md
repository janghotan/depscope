---
version: 1
slug: "web"
primary_target: "web"
related_targets: []
---

# Website surface brief

- **Scope:** Phase 6 web surface for single-package scans, package comparison, report detail, and the offline demo. No project or inventory views.
- **Visitor mode:** Operate.
- **Audience and job:** A developer checking one npm or PyPI package before adding or upgrading it, often comparing two candidates.
- **Action:** Run a scan, read the condition, inspect evidence and gaps, and decide what to do next.
- **Proof and content:** Canonical `ScanReport` data, deterministic demo fixtures, successful and failed checks, sources, limitations, and dependency paths. No fabricated adoption, benchmarks, or certifications.
- **Constraints:** npm and PyPI only; demo must be labelled `DEMO DATA`; severity, status, finding kind, and demo mode must never rely on color alone; render remote text safely; responsive and keyboard accessible.

## Chosen direction

Inspection Bay: a working structural-condition inspection desk. The memorable moment is tracing the load path to a flagged dependency while the affected checkpoint holds a visible marker until its evidence is opened.

## Unresolved decisions

Hosting and deployment (D13), API authentication and trusted proxies (D10), and project/inventory wire exposure (D16).

## Direction contract

THESIS: A working inspection desk, not a marketing page: every scan is a condition report whose evidence and gaps are inspected in place; refuse the generic security dashboard and the fake terminal.

OWN-WORLD: Slate-blue shell (#16232B), warm survey-paper field (#F4EFE4), ink (#1D2328), safety orange (#D95B2B), signal amber (#D9A441), inspection green (#3E7A63); hairline rules, coordinate ticks, stamped verdict boxes; technical grotesk UI with monospace data; no glow, gradient, or rounded card grid.

STORY: The visitor enters a package, sees the report stamped with a condition, traces the load path to the flagged node, and knows what was checked, what was not, and what to do next.

FIRST VIEWPORT: A top instrument bar with DepScope, live API status, and Scan/Compare/Demo navigation. Left: a large inspection request panel with an ecosystem segmented control, package name, optional exact version, and Scan button. Right: a labelled DEMO specimen sheet with a stamped condition, three check strips, one finding excerpt, and a small load-path diagram with one orange checkpoint. At 1440x900 both panels fill the viewport without scrolling; at 390px they stack, form first.

FORM: Inspection report / engineering condition assessment; position 4 of 7 grounded directions; seed:2d97babd3e444d59.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.
