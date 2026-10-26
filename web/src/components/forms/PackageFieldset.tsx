"use client";

import { AlertIcon } from "@/components/icons";

export type PackageDraft = {
  ecosystem: "npm" | "pypi";
  name: string;
  version: string;
};

export type PackageDraftErrors = {
  ecosystem?: string;
  name?: string;
  version?: string;
};

export function emptyDraft(): PackageDraft {
  return { ecosystem: "npm", name: "", version: "" };
}

export function draftFromInput(input: {
  ecosystem: "npm" | "pypi";
  name: string;
  version?: string;
}): PackageDraft {
  return {
    ecosystem: input.ecosystem,
    name: input.name,
    version: input.version ?? "",
  };
}

export function draftToInput(draft: PackageDraft): {
  ecosystem: "npm" | "pypi";
  name: string;
  version?: string;
} {
  return {
    ecosystem: draft.ecosystem,
    name: draft.name.trim(),
    ...(draft.version.trim() === "" ? {} : { version: draft.version.trim() }),
  };
}

export function PackageFieldset({
  idPrefix,
  legend,
  value,
  errors,
  onChange,
}: {
  idPrefix: string;
  legend: string;
  value: PackageDraft;
  errors: PackageDraftErrors;
  onChange: (next: PackageDraft) => void;
}) {
  const nameId = `${idPrefix}-name`;
  const versionId = `${idPrefix}-version`;
  const nameErrorId = `${nameId}-error`;
  const versionErrorId = `${versionId}-error`;
  const ecosystemErrorId = `${idPrefix}-ecosystem-error`;

  return (
    <fieldset className="space-y-4">
      <legend className="rule-label mb-3 text-ink">{legend}</legend>

      <div>
        <span className="field-label" id={`${idPrefix}-ecosystem-label`}>
          Ecosystem
        </span>
        <div
          className="segmented"
          role="radiogroup"
          aria-labelledby={`${idPrefix}-ecosystem-label`}
          aria-describedby={
            errors.ecosystem ? ecosystemErrorId : undefined
          }
        >
          <label>
            <input
              type="radio"
              name={`${idPrefix}-ecosystem`}
              value="npm"
              checked={value.ecosystem === "npm"}
              onChange={() => onChange({ ...value, ecosystem: "npm" })}
            />
            <span>npm</span>
          </label>
          <label>
            <input
              type="radio"
              name={`${idPrefix}-ecosystem`}
              value="pypi"
              checked={value.ecosystem === "pypi"}
              onChange={() => onChange({ ...value, ecosystem: "pypi" })}
            />
            <span>PyPI</span>
          </label>
        </div>
        {errors.ecosystem ? (
          <p className="field-error" id={ecosystemErrorId} role="alert">
            <AlertIcon className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            {errors.ecosystem}
          </p>
        ) : null}
      </div>

      <div>
        <label className="field-label" htmlFor={nameId}>
          Package name
        </label>
        <input
          id={nameId}
          className="field-input"
          type="text"
          value={value.name}
          onChange={(event) => onChange({ ...value, name: event.target.value })}
          placeholder={value.ecosystem === "npm" ? "lodash" : "requests"}
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          aria-invalid={errors.name ? true : undefined}
          aria-describedby={`${nameId}-hint${errors.name ? ` ${nameErrorId}` : ""}`}
        />
        <p className="field-hint" id={`${nameId}-hint`}>
          {value.ecosystem === "npm"
            ? "npm name, including @scope/name when scoped. Case is normalized."
            : "PyPI project name. Case and separators are normalized."}
        </p>
        {errors.name ? (
          <p className="field-error" id={nameErrorId} role="alert">
            <AlertIcon className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            {errors.name}
          </p>
        ) : null}
      </div>

      <div>
        <label className="field-label" htmlFor={versionId}>
          Exact version <span className="font-normal lowercase">(optional)</span>
        </label>
        <input
          id={versionId}
          className="field-input"
          type="text"
          value={value.version}
          onChange={(event) =>
            onChange({ ...value, version: event.target.value })
          }
          placeholder="4.17.21"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          aria-invalid={errors.version ? true : undefined}
          aria-describedby={`${versionId}-hint${errors.version ? ` ${versionErrorId}` : ""}`}
        />
        <p className="field-hint" id={`${versionId}-hint`}>
          Leave blank to resolve the latest published release. Range operators
          and URL-like values are not accepted.
        </p>
        {errors.version ? (
          <p className="field-error" id={versionErrorId} role="alert">
            <AlertIcon className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            {errors.version}
          </p>
        ) : null}
      </div>
    </fieldset>
  );
}
