import type { CheckStatus } from "@/generated/report-types";
import {
  AlertIcon,
  CheckIcon,
  CloseIcon,
  DashIcon,
} from "@/components/icons";
import { CHECK_STATUS_LABEL } from "@/lib/format";

const CHECK_STYLE: Record<
  CheckStatus,
  { className: string; Icon: typeof CheckIcon }
> = {
  succeeded: {
    className: "text-inspection-deep border-inspection-deep",
    Icon: CheckIcon,
  },
  unavailable: {
    className: "text-amber-deep border-amber-deep border-dashed",
    Icon: AlertIcon,
  },
  skipped: {
    className: "text-ink-soft border-ink-faint border-dashed",
    Icon: DashIcon,
  },
  failed: {
    className: "text-paper bg-oxide-deep border-oxide-deep",
    Icon: CloseIcon,
  },
};

export function CheckStatusTag({ status }: { status: CheckStatus }) {
  const { className, Icon } = CHECK_STYLE[status];
  return (
    <span className={`tag ${className}`}>
      <Icon className="h-3.5 w-3.5" />
      {CHECK_STATUS_LABEL[status]}
    </span>
  );
}

export function CheckStatusGlyph({ status }: { status: CheckStatus }) {
  const { className, Icon } = CHECK_STYLE[status];
  return (
    <span
      className={`inline-flex h-7 w-7 shrink-0 items-center justify-center border ${className}`}
    >
      <Icon className="h-4 w-4" />
    </span>
  );
}
