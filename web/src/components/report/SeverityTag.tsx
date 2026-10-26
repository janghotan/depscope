import type { Severity } from "@/generated/report-types";
import { SEVERITY_LABEL } from "@/lib/format";

const SEVERITY_CLASS: Record<Severity, string> = {
  critical: "tag-critical",
  high: "tag-high",
  medium: "tag-medium",
  low: "tag-low",
  unknown: "tag-unknown",
};

const SEVERITY_BARS: Record<Severity, number> = {
  critical: 4,
  high: 3,
  medium: 2,
  low: 1,
  unknown: 0,
};

export function SeverityTag({ severity }: { severity: Severity }) {
  return (
    <span className={`tag ${SEVERITY_CLASS[severity]}`}>
      <span className="flex items-end gap-[1px]" aria-hidden="true">
        {[0, 1, 2, 3].map((index) => (
          <span
            key={index}
            className={`w-[2px] ${
              index < SEVERITY_BARS[severity]
                ? "bg-current"
                : "border border-current opacity-40"
            }`}
            style={{ height: `${4 + index * 2}px` }}
          />
        ))}
      </span>
      {SEVERITY_LABEL[severity]}
    </span>
  );
}
