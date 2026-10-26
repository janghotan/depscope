import Link from "next/link";

import { ScopeMark } from "@/components/icons";

export function SiteFooter() {
  return (
    <footer
      data-site-footer
      className="mt-auto border-t border-shell-line bg-shell text-[#c6cfd2]"
    >
      <div className="record-container grid gap-8 py-10 md:grid-cols-[1.2fr_1fr_1fr]">
        <div>
          <div className="flex items-center gap-3 text-paper">
            <ScopeMark className="h-6 w-6 text-safety" />
            <span className="text-sm font-bold tracking-[0.16em]">DEPSCOPE</span>
          </div>
          <p className="mt-4 max-w-sm text-sm leading-6">
            A condition record shows what was checked, what was found, and what
            could not be checked. It is evidence, not a guarantee of safety.
          </p>
        </div>

        <nav aria-label="Footer" className="text-sm">
          <p className="rule-label text-[#9dabb0]">Inspect</p>
          <ul className="mt-3 space-y-2">
            <li>
              <Link href="/scan" className="link-quiet hover:text-paper">
                Scan a package
              </Link>
            </li>
            <li>
              <Link href="/compare" className="link-quiet hover:text-paper">
                Compare two packages
              </Link>
            </li>
            <li>
              <Link href="/demo" className="link-quiet hover:text-paper">
                Offline demo
              </Link>
            </li>
          </ul>
        </nav>

        <div className="text-sm">
          <p className="rule-label text-[#9dabb0]">Record</p>
          <ul className="mt-3 space-y-2">
            <li>
              <Link href="/about" className="link-quiet hover:text-paper">
                How to read a record
              </Link>
            </li>
            <li>
              <Link href="/about#api" className="link-quiet hover:text-paper">
                API and access
              </Link>
            </li>
            <li>
              <Link href="/about#accessibility" className="link-quiet hover:text-paper">
                Accessibility
              </Link>
            </li>
          </ul>
        </div>
      </div>
      <div className="border-t border-shell-line">
        <div className="record-container flex flex-col gap-2 py-4 font-mono text-[0.625rem] tracking-[0.08em] text-[#9dabb0] uppercase sm:flex-row sm:items-center sm:justify-between">
          <p>Schema 1.0.0 · npm and PyPI only</p>
          <p>Live scans via the local HTTP API · Demo data generated offline</p>
        </div>
      </div>
    </footer>
  );
}
