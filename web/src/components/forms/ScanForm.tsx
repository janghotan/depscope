"use client";

import { useRouter } from "next/navigation";
import { useId, useState, type FormEvent } from "react";

import { ArrowRightIcon, AlertIcon, CheckIcon } from "@/components/icons";
import {
  PackageFieldset,
  draftFromInput,
  draftToInput,
  emptyDraft,
  type PackageDraft,
  type PackageDraftErrors,
} from "@/components/forms/PackageFieldset";
import type { PackageInput } from "@/lib/api-types";
import { parsePackageInput } from "@/lib/request";

function fieldForMessage(
  message: string,
): keyof PackageDraftErrors | null {
  if (message.startsWith("ecosystem")) {
    return "ecosystem";
  }
  if (message.startsWith("name")) {
    return "name";
  }
  if (message.startsWith("version")) {
    return "version";
  }
  return null;
}

export function ScanForm({
  initial,
  compact = false,
}: {
  initial?: PackageInput;
  compact?: boolean;
}) {
  const router = useRouter();
  const idPrefix = useId().replace(/:/g, "");
  const [draft, setDraft] = useState<PackageDraft>(
    initial ? draftFromInput(initial) : emptyDraft(),
  );
  const [errors, setErrors] = useState<PackageDraftErrors>({});
  const [generalError, setGeneralError] = useState<string | null>(null);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setGeneralError(null);
    const input = draftToInput(draft);
    const parsed = parsePackageInput(input);
    if (!parsed.ok) {
      const field = fieldForMessage(parsed.issue.message);
      if (field) {
        setErrors({ [field]: parsed.issue.message });
      } else {
        setErrors({});
        setGeneralError(parsed.issue.message);
      }
      return;
    }
    setErrors({});
    const params = new URLSearchParams();
    params.set("ecosystem", parsed.value.ecosystem);
    params.set("name", parsed.value.name);
    if (parsed.value.version) {
      params.set("version", parsed.value.version);
    }
    router.push(`/scan?${params.toString()}`);
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-5">
      <PackageFieldset
        idPrefix={idPrefix}
        legend="Package under inspection"
        value={draft}
        errors={errors}
        onChange={(next) => {
          setDraft(next);
          setErrors({});
        }}
      />

      {generalError ? (
        <p className="field-error" role="alert">
          <AlertIcon className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          {generalError}
        </p>
      ) : null}

      <button type="submit" className="button-primary w-full">
        <span>Inspect package</span>
        <ArrowRightIcon className="h-4 w-4" />
      </button>

      {!compact ? (
        <ul className="grid gap-2 border-t border-rule pt-3 text-xs text-ink-soft sm:grid-cols-3">
          {["No code execution", "No installs", "Read-only checks"].map(
            (item) => (
              <li key={item} className="flex items-center gap-1.5">
                <CheckIcon className="h-3.5 w-3.5 shrink-0 text-inspection-deep" />
                {item}
              </li>
            ),
          )}
        </ul>
      ) : null}
    </form>
  );
}
