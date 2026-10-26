import { InfoIcon } from "@/components/icons";

export function DemoBanner({ notice }: { notice: string }) {
  return (
    <aside
      aria-label="Demo data notice"
      className="border border-amber-deep bg-amber text-night"
    >
      <div className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:gap-5">
        <span className="w-fit shrink-0 border-2 border-night px-2 py-1 font-mono text-xs font-bold tracking-[0.16em] uppercase">
          Demo data
        </span>
        <div className="flex items-start gap-2 text-sm leading-5">
          <InfoIcon className="mt-0.5 hidden h-4 w-4 shrink-0 sm:block" />
          <p>
            <span className="font-semibold">{notice}</span>{" "}
            <span>
              This page renders a local fixture with no network request. It is
              not a live scan and must not be used as a security result.
            </span>
          </p>
        </div>
      </div>
    </aside>
  );
}
