import { readFile, writeFile, mkdir, readdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { compile } from "json-schema-to-typescript";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
export const webRoot = path.resolve(scriptDir, "..");
export const repoRoot = path.resolve(webRoot, "..");

export const schemaSource = path.join(repoRoot, "schemas", "scan-report.v1.schema.json");
export const schemaTarget = path.join(webRoot, "src", "generated", "scan-report.v1.schema.json");
export const demoSourceDir = path.join(repoRoot, "testdata", "fixtures", "demo");
export const demoTargetDir = path.join(webRoot, "src", "generated", "demo");
export const typesTarget = path.join(webRoot, "src", "generated", "report-types.ts");

const banner = `/**
 * 此文件由 schemas/scan-report.v1.schema.json 自动生成。
 * 请勿手动编辑；运行 pnpm run generate:types 重新生成。
 */`;

async function ensureDir(dir) {
  await mkdir(dir, { recursive: true });
}

export async function syncContract() {
  await ensureDir(path.dirname(schemaTarget));
  await ensureDir(demoTargetDir);

  const schema = await readFile(schemaSource, "utf8");
  await writeFile(schemaTarget, schema, "utf8");

  const files = (await readdir(demoSourceDir)).filter((name) => name.endsWith(".json")).sort();
  for (const name of files) {
    const source = await readFile(path.join(demoSourceDir, name), "utf8");
    await writeFile(path.join(demoTargetDir, name), source, "utf8");
  }

  return { schema, demoNames: files };
}

export async function renderTypes() {
  const schema = JSON.parse(await readFile(schemaSource, "utf8"));
  // 移除 title 与 $id，避免生成器改用 schema 标识名而非显式类型名。
  delete schema.title;
  delete schema.$id;
  return compile(schema, "ScanReport", {
    bannerComment: banner,
    additionalProperties: false,
    declareExternallyReferenced: true,
    enableConstEnums: true,
    format: true,
    style: {
      singleQuote: false,
      semi: true,
    },
  });
}

export async function readGeneratedTypes() {
  return readFile(typesTarget, "utf8");
}
