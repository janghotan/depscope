"use client";

import { useRouter } from "next/navigation";
import { useId, useState, type FormEvent } from "react";

import { AlertIcon, ArrowRightIcon } from "@/components/icons";
import {
  PackageFieldset,
  draftFromInput,
  draftToInput,
  emptyDraft,
  type PackageDraft,
  type PackageDraftErrors,
} from "@/components/forms/PackageFieldset";
import type { CompareInput, PackageInput } from "@/lib/api-types";
import { parseCompareInput } from "@/lib/request";

function fieldForMessage(message: string): keyof PackageDraftErrors | null {
  const body = message.replace(/^(left|right):\s*/, "");
  if (body.startsWith("ecosystem")) {
    return "ecosystem";
  }
  if (body.startsWith("name")) {
    return "name";
  }
  if (body.startsWith("version")) {
    return "version";
  }
  return null;
}

export function CompareForm({
  initialLeft,
  initialRight,
}: {
  initialLeft?: PackageInput;
  initialRight?: PackageInput;
}) {
  const router = useRouter();
  const idBase = useId().replace(/:/g, "");
  const [left, setLeft] = useState<PackageDraft>(
    initialLeft ? draftFromInput(initialLeft) : emptyDraft(),
  );
  const [right, setRight] = useState<PackageDraft>(
    initialRight ? draftFromInput(initialRight) : emptyDraft(),
  );
  const [leftErrors, setLeftErrors] = useState<PackageDraftErrors>({});
  const [rightErrors, setRightErrors] = useState<PackageDraftErrors>({});
  const [generalError, setGeneralError] = useState<string | null>(null);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setGeneralError(null);
    const input: CompareInput = {
      left: draftToInput(left),
      right: draftToInput(right),
    };
    const parsed = parseCompareInput(input);
    if (!parsed.ok) {
      const field = fieldForMessage(parsed.issue.message);
      if (field && parsed.issue.side === "left") {
        setLeftErrors({ [field]: parsed.issue.message });
        setRightErrors({});
      } else if (field && parsed.issue.side === "right") {
        setRightErrors({ [field]: parsed.issue.message });
        setLeftErrors({});
      } else {
        setLeftErrors({});
        setRightErrors({});
        setGeneralError(parsed.issue.message);
      }
      return;
    }

    setLeftErrors({});
    setRightErrors({});
    const params = new URLSearchParams();
    params.set("leftEcosystem", parsed.value.left.ecosystem);
    params.set("leftName", parsed.value.left.name);
    if (parsed.value.left.version) {
      params.set("leftVersion", parsed.value.left.version);
    }
    params.set("rightEcosystem", parsed.value.right.ecosystem);
    params.set("rightName", parsed.value.right.name);
    if (parsed.value.right.version) {
      params.set("rightVersion", parsed.value.right.version);
    }
    router.push(`/compare?${params.toString()}`);
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-6">
      <div className="border-t border-rule pt-5">
        <PackageFieldset
          idPrefix={`${idBase}-left`}
          legend="Side A"
          value={left}
          errors={leftErrors}
          onChange={(next) => {
            setLeft(next);
            setLeftErrors({});
          }}
        />
      </div>

      <div className="border-t border-rule pt-5">
        <PackageFieldset
          idPrefix={`${idBase}-right`}
          legend="Side B"
          value={right}
          errors={rightErrors}
          onChange={(next) => {
            setRight(next);
            setRightErrors({});
          }}
        />
      </div>

      {generalError ? (
        <p className="field-error" role="alert">
          <AlertIcon className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          {generalError}
        </p>
      ) : null}

      <button type="submit" className="button-primary w-full">
        <span>Compare conditions</span>
        <ArrowRightIcon className="h-4 w-4" />
      </button>

      <p className="border-t border-rule pt-3 text-xs leading-5 text-ink-soft">
        Each side is scanned independently. If one side fails, the other side is
        still returned with its record and error state intact.
      </p>
    </form>
  );
}
