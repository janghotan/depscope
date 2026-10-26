import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import {
  demoSourceDir,
  demoTargetDir,
  readGeneratedTypes,
  renderTypes,
  schemaSource,
  schemaTarget,
  syncContract,
} from "./contract.mjs";

const failures = [];

async function assertSame(label, source, target) {
  const expected = await readFile(source, "utf8");
  const actual = await readFile(target, "utf8").catch(() => null);
  if (actual !== expected) {
    failures.push(`${label} is out of date.`);
  }
}

await syncContract();

await assertSame("src/generated/scan-report.v1.schema.json", schemaSource, schemaTarget);

const demoFiles = await readdir(demoSourceDir);
for (const name of demoFiles.filter((entry) => entry.endsWith(".json")).sort()) {
  await assertSame(
    `src/generated/demo/${name}`,
    path.join(demoSourceDir, name),
    path.join(demoTargetDir, name),
  );
}

const expectedTypes = await renderTypes();
const actualTypes = await readGeneratedTypes().catch(() => null);
if (actualTypes !== expectedTypes) {
  failures.push("src/generated/report-types.ts is out of date.");
}

if (failures.length > 0) {
  console.error("Contract drift detected:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  console.error("Run `pnpm run sync:contract && pnpm run generate:types` and retry.");
  process.exit(1);
}

console.log("Generated contract, demo fixtures, and TypeScript types are current.");
