import Link from "next/link";

import { ScopeMark } from "@/components/icons";
import { EngineStatus } from "@/components/site/EngineStatus";
import { SiteNav } from "@/components/site/SiteNav";

export function SiteHeader() {
  return (
    <header
      data-site-header
      className="sticky top-0 z-40 border-b border-shell-line bg-shell text-paper"
    >
      <div className="record-container flex flex-col gap-2 py-2 sm:flex-row sm:items-center sm:gap-8 sm:py-0">
        <div className="flex items-center justify-between gap-6 sm:py-3">
          <Link
            href="/"
            className="flex items-center gap-3 text-paper"
            aria-label="DepScope home"
          >
            <ScopeMark className="h-7 w-7 text-safety" />
            <span className="flex flex-col leading-none">
              <span className="text-[1.05rem] font-bold tracking-[0.16em]">
                DEPSCOPE
              </span>
              <span className="mt-1 hidden font-mono text-[0.5rem] font-medium tracking-[0.2em] text-[#c6cfd2] uppercase sm:block">
                Dependency condition records
              </span>
            </span>
          </Link>
          <div className="sm:hidden">
            <EngineStatus />
          </div>
        </div>
        <div className="flex items-center justify-between gap-4 border-t border-shell-line pt-2 sm:ml-auto sm:border-0 sm:pt-0">
          <SiteNav />
          <div className="hidden sm:block">
            <EngineStatus />
          </div>
        </div>
      </div>
    </header>
  );
}
