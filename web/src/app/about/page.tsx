import type { Metadata } from "next";
import Link from "next/link";

import { CheckStatusTag } from "@/components/report/CheckStatusTag";
import { SeverityTag } from "@/components/report/SeverityTag";
import { StatusStamp } from "@/components/report/StatusStamp";
import { PageMasthead } from "@/components/site/PageMasthead";
import type { CheckStatus, ScanStatus, Severity } from "@/generated/report-types";
import { CONDITION_COPY } from "@/lib/format";

export const metadata: Metadata = {
  title: "How to read a record",
  description:
    "DepScope's condition vocabulary, checks, limitations, comparison semantics, API access notes, and accessibility commitments.",
};

const STATUSES: ScanStatus[] = [
  "clean",
  "findings",
  "incomplete",
  "failed",
  "cancelled",
];

const CHECK_STATUSES: CheckStatus[] = [
  "succeeded",
  "unavailable",
  "skipped",
  "failed",
];

const SEVERITIES: Severity[] = [
  "critical",
  "high",
  "medium",
  "low",
  "unknown",
];

const CHECKS = [
  {
    name: "Registry metadata",
    body: "Reads package and version metadata from the npm registry or PyPI, including whether a requested exact version exists.",
  },
  {
    name: "Dependency metadata",
    body: "Records declared direct dependencies. It does not install or execute them.",
  },
  {
    name: "Dependency graph",
    body: "Records the graph the engine can establish. For a single package, declared direct dependencies are not resolved transitively; when that check is skipped the record becomes incomplete.",
  },
  {
    name: "Lifecycle scripts",
    body: "Reports npm lifecycle script declarations as inspection signals. Scripts are never executed.",
  },
  {
    name: "Typosquat heuristic",
    body: "Compares the requested name with popular package names and records a heuristic signal, never a confirmed advisory.",
  },
  {
    name: "Vulnerability lookup",
    body: "Matches the resolved version against OSV.dev advisories. A missing or failed lookup is reported as missing coverage.",
  },
  {
    name: "Risk evaluation",
    body: "Records the engine's risk rationale for confirmed findings. The website renders that rationale; it does not invent or recalculate a score.",
  },
];

const SECTIONS = [
  { id: "record", label: "Read a record" },
  { id: "checks", label: "Checks and limits" },
  { id: "comparison", label: "Comparison" },
  { id: "demo", label: "Demo data" },
  { id: "api", label: "API and access" },
  { id: "accessibility", label: "Accessibility" },
  { id: "installation", label: "CLI installation" },
  { id: "sources", label: "Sources and methodology" },
  { id: "contributing", label: "Contributing" },
];

export default function AboutPage() {
  return (
    <div className="record-container py-8 lg:py-10">
      <PageMasthead
        index="Reference manual"
        title="How to read a record"
        intro="The short version: a condition record is evidence. It names what was found, what was checked, and what could not be checked."
      />

      <div className="mt-6 grid gap-10 lg:grid-cols-[14rem_minmax(0,1fr)]">
        <nav aria-label="On this page" className="hidden lg:block">
          <div className="sticky top-24 border-t border-rule pt-3">
            <p className="rule-label text-ink-soft">On this page</p>
            <ul className="mt-3 space-y-2">
              {SECTIONS.map((section, index) => (
                <li key={section.id}>
                  <a
                    href={`#${section.id}`}
                    className="flex items-baseline gap-2 text-sm text-ink-soft hover:text-ink"
                  >
                    <span className="data-value text-[0.625rem] text-safety-deep">
                      {String(index + 1).padStart(2, "0")}
                    </span>
                    <span className="link-quiet">{section.label}</span>
                  </a>
                </li>
              ))}
            </ul>
          </div>
        </nav>

        <div className="prose-record max-w-3xl">
          <section aria-labelledby="record">
            <h2 id="record">1. How to read a record</h2>
            <p>
              Every record starts with a target package and a condition stamp.
              The stamp is a summary of the record, not a substitute for it. Read
              the findings, checks, sources, and limitations before acting.
            </p>

            <div className="my-5 grid gap-3 sm:grid-cols-2">
              {STATUSES.map((status) => (
                <div key={status} className="border border-rule bg-sheet p-3">
                  <StatusStamp status={status} size="small" />
                  <p className="mt-2 text-sm leading-5 text-ink-soft">
                    {CONDITION_COPY[status].description}
                  </p>
                </div>
              ))}
            </div>

            <p>
              <strong>Confirmed</strong> findings are backed by a matched
              advisory or equivalent corroborated evidence.{" "}
              <strong>Heuristic</strong> findings are signals such as a
              typosquat resemblance; they are leads to inspect, not vulnerability
              confirmations. Severity labels come from the recorded evidence and
              are always shown as text, never colour alone.
            </p>

            <div className="my-4 flex flex-wrap gap-2">
              {SEVERITIES.map((severity) => (
                <SeverityTag key={severity} severity={severity} />
              ))}
            </div>

            <p>
              Check coverage is part of the condition. A check can be{" "}
              <strong>succeeded</strong>, <strong>unavailable</strong>,{" "}
              <strong>skipped</strong>, or <strong>failed</strong>. Only a record
              where every recorded check succeeded, no findings exist, and at
              least one source is present can read as clean.
            </p>

            <div className="flex flex-wrap gap-2">
              {CHECK_STATUSES.map((status) => (
                <CheckStatusTag key={status} status={status} />
              ))}
            </div>
          </section>

          <section aria-labelledby="checks">
            <h2 id="checks">2. What a scan checks, and what it does not</h2>
            <p>
              Supported ecosystems are npm and PyPI. A request names one package
              and may pin an exact published version; range operators and
              URL-like values are rejected rather than approximated.
            </p>

            <div className="my-4 divide-y divide-rule border-y border-rule">
              {CHECKS.map((check) => (
                <div key={check.name} className="py-3">
                  <h3 className="text-sm font-bold">{check.name}</h3>
                  <p className="mt-1 text-sm leading-6 text-ink-soft">
                    {check.body}
                  </p>
                </div>
              ))}
            </div>

            <p>
              A scan <strong>never</strong> installs a package, runs an install
              script, imports package code, or executes a lifecycle script.
              DepScope reads registry and OSV data; it does not run the thing it
              inspects.
            </p>
            <p>
              Transitive resolution for a single package is not implemented. The
              record reports direct dependencies it could not resolve and marks
              the condition incomplete. It never substitutes the latest version
              for a pinned one.
            </p>
            <p>
              This website does not scan projects, installed environments, or
              lockfiles. Those surfaces are not part of this build.
            </p>
          </section>

          <section aria-labelledby="comparison">
            <h2 id="comparison">3. Comparison</h2>
            <p>
              Each side is scanned independently. The API returns both sides in
              one envelope; each side contains either a validated report or an
              error. A failed side is shown as a failed side, and the other side
              is not hidden.
            </p>
            <p>
              The comparison marks measures that differ and links each recorded
              condition in full. It does not choose a package, merge the sides
              into a single score, or treat a missing check on one side as a
              reason to discount the other. Mixed demo and live sides are warned
              about rather than compared as equivalent.
            </p>
          </section>

          <section aria-labelledby="demo">
            <h2 id="demo">4. Demo data</h2>
            <p>
              Demo pages read fixed JSON fixtures committed in{" "}
              <code>testdata/fixtures/demo</code> and validate them against
              schema 1.0.0 before display. Package names are synthetic, advisory
              identifiers begin with <code>DEMO-</code>, timestamps are fixed,
              and every demo surface is labelled <strong>DEMO DATA</strong>.
            </p>
            <p>
              Demo pages make no scan or comparison request; the shared header
              may check the local API health status. Fixture records remain
              available when the API is offline. A demo record is not a live
              scan and must never be used as a security result.{" "}
              <Link href="/demo" className="link-quiet">
                Open the demo specimens
              </Link>
              .
            </p>
          </section>

          <section aria-labelledby="api">
            <h2 id="api">5. API and access</h2>
            <p>
              The browser talks only to this site&apos;s same-origin routes.
              Those routes proxy scan, comparison, and health requests to the
              shared Go HTTP API. The site does not call npm, PyPI, or OSV
              directly, and it does not re-implement resolution or risk rules.
            </p>
            <ul>
              <li>
                Scan and comparison requests are capped at 16 KiB and must be
                JSON objects with exactly the documented fields.
              </li>
              <li>
                Upstream responses are capped at 8 MiB, validated against the
                canonical schema, and required to be live reports for live
                requests.
              </li>
              <li>
                Responses use <code>Cache-Control: no-store</code> and{" "}
                <code>X-Content-Type-Options: nosniff</code>.
              </li>
              <li>
                The proxy reads <code>DEPSCOPE_API_BASE_URL</code> (default{" "}
                <code>http://127.0.0.1:8080</code>) and{" "}
                <code>DEPSCOPE_API_TIMEOUT_MS</code> (default 100000, allowed
                1000–120000).
              </li>
              <li>
                The API itself is documented in the repository&apos;s{" "}
                <code>docs/api.md</code>. The canonical report schema is{" "}
                <code>schemas/scan-report.v1.schema.json</code> version 1.0.0.
              </li>
            </ul>
            <p>
              This build has no accounts, authentication, stored history,
              telemetry, or analytics. Deployment, authentication, and trusted
              proxy handling are not part of this phase. If the API is not
              running, live scans fail visibly; they do not fall back to a demo
              result.
            </p>
          </section>

          <section aria-labelledby="accessibility">
            <h2 id="accessibility">6. Accessibility</h2>
            <p>
              The site uses semantic landmarks, real form labels, native
              controls, keyboard-operable disclosure controls, and visible
              focus. Status, severity, finding kind, and demo mode always carry a
              text label; colour reinforces the label but never replaces it.
            </p>
            <p>
              Layouts reflow for narrow screens and remain usable at 200% zoom.
              Motion is limited and respects <code>prefers-reduced-motion</code>.
              The record can be printed or copied as plain text. No formal
              conformance certification is claimed.
            </p>
          </section>

          <section aria-labelledby="installation">
            <h2 id="installation">7. Install the CLI</h2>
            <p>
              From a checkout at the repository root, install and run the Go CLI:
            </p>
            <pre className="overflow-x-auto border border-rule bg-shell p-4 text-sm text-paper">
              <code>{"go install ./cmd/depscope\ndepscope scan lodash"}</code>
            </pre>
            <p>
              Use <code>depscope help</code> for available commands. The CLI
              and website both use the same Go security engine. The scan checks
              a package record and does not install or execute the package.
            </p>
          </section>

          <section aria-labelledby="sources">
            <h2 id="sources">8. Sources and methodology</h2>
            <p>
              Live records use the npm Registry or PyPI for package metadata and
              OSV.dev for version-specific vulnerability advisories. Each
              record lists the sources and checks it actually completed. The
              API and engine documentation in the repository describe request
              limits, matching semantics, and provider failure behavior.
            </p>
            <p>
              Confirmed findings come from matched advisory evidence. Typosquat
              similarity and inspected npm lifecycle scripts are heuristic
              signals. Maintenance metadata is context, not a security finding.
              Missing, failed, or unavailable checks remain visible and make
              coverage incomplete; they are never interpreted as clean.
            </p>
            <p>
              The site currently scans a single npm or PyPI package and
              compares two package records. It does not scan local projects,
              lockfiles, or installed environments. Project scan support is
              limited to npm lockfile v2/v3 and exact-pinned Python
              <code>requirements.txt</code> in the CLI; Python transitives,
              workspaces, and peer dependencies are not resolved.
            </p>
          </section>

          <section aria-labelledby="contributing">
            <h2 id="contributing">9. Contributing</h2>
            <p>
              Contributions should preserve the canonical report schema and
              shared Go engine boundary. The repository includes architecture,
              contract, risk-rule, and API documentation, plus deterministic
              fixtures and offline tests. Start with <code>AGENTS.md</code> and
              run the Go and web checks described in the repository README.
            </p>
            <p>
              Do not add public network calls to default tests, execute package
              code, or present unavailable evidence as safe. Report new source
              behavior with sanitized fixtures and contract validation.
            </p>
          </section>

          <p className="border-t border-rule pt-5">
            <Link href="/scan" className="link-arrow text-sm">
              Scan a package
            </Link>{" "}
            ·{" "}
            <Link href="/compare" className="link-arrow text-sm">
              Compare two packages
            </Link>
          </p>
        </div>
      </div>
    </div>
  );
}
