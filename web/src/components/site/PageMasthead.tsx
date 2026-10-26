import type { ReactNode } from "react";

export function PageMasthead({
  index,
  title,
  intro,
  children,
}: {
  index: string;
  title: string;
  intro: string;
  children?: ReactNode;
}) {
  return (
    <header className="border-b border-rule pb-6">
      <p className="rule-label text-ink-soft">{index}</p>
      <div className="mt-2 flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between lg:gap-12">
        <h1 className="max-w-2xl text-3xl leading-[1.05] font-bold tracking-[-0.02em] sm:text-4xl">
          {title}
        </h1>
        <p className="max-w-md text-sm leading-6 text-ink-soft">{intro}</p>
      </div>
      {children}
    </header>
  );
}
