import type { ScanStatus } from "@/generated/report-types";
import {
  AlertIcon,
  CheckIcon,
  CloseIcon,
  HalfIcon,
  SlashIcon,
} from "@/components/icons";
import { CONDITION_COPY } from "@/lib/format";

const STATUS_STYLE: Record<
  ScanStatus,
  { color: string; Icon: typeof CheckIcon }
> = {
  clean: { color: "text-inspection-deep", Icon: CheckIcon },
  findings: { color: "text-safety-deep", Icon: AlertIcon },
  incomplete: { color: "text-amber-deep", Icon: HalfIcon },
  failed: { color: "text-oxide-deep", Icon: CloseIcon },
  cancelled: { color: "text-ink-soft", Icon: SlashIcon },
};

export function StatusStamp({
  status,
  size = "large",
  animate = false,
}: {
  status: ScanStatus;
  size?: "large" | "small";
  animate?: boolean;
}) {
  const { color, Icon } = STATUS_STYLE[status];
  const copy = CONDITION_COPY[status];

  return (
    <span
      className={`stamp ${size === "small" ? "stamp-small" : ""} ${color} ${
        animate ? "stamp-enter" : ""
      }`}
      role="img"
      aria-label={`Condition: ${copy.label}`}
    >
      <span className="flex items-center gap-1.5">
        <Icon className={size === "small" ? "h-3.5 w-3.5" : "h-5 w-5"} />
        {copy.label}
      </span>
      {size === "large" ? <span className="stamp-label">Condition</span> : null}
    </span>
  );
}
