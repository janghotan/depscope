"use client";

import { useState, type ReactNode } from "react";

/** 折叠区域首次展开后才渲染内容，避免隐藏内容产生重复 id 与额外渲染开销。 */
export function DeferredDetails({
  summary,
  children,
  className,
}: {
  summary: string;
  children: ReactNode;
  className?: string;
}) {
  const [opened, setOpened] = useState(false);

  return (
    <details
      className={className}
      onToggle={(event) => {
        if (event.currentTarget.open) {
          setOpened(true);
        }
      }}
    >
      <summary className="rule-label w-fit cursor-pointer text-ink-soft hover:text-ink">
        {summary}
      </summary>
      {opened ? <div className="mt-4">{children}</div> : null}
    </details>
  );
}
