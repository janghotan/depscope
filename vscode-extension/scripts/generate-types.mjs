// 同步并生成当前扩展的 src/generated/report-types.ts。
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const extRoot = path.resolve(scriptDir, "..");
const typesTarget = path.join(extRoot, "src", "generated", "report-types.ts");
const webTypesSource = path.resolve(extRoot, "..", "web", "src", "generated", "report-types.ts");

try {
  const sourceTypes = await readFile(webTypesSource, "utf8");
  const banner = `/**
 * 此文件由 schemas/scan-report.v1.schema.json 自动生成。
 * 请勿手动编辑；运行 npm run generate-types 重新生成。
 */\n\n`;
  const normalizedTypes = sourceTypes.replace(/\/\*\*[\s\S]*?pnpm run generate:types[\s\S]*?\*\/\n*/, banner);
  await writeFile(typesTarget, normalizedTypes, "utf8");
  console.log(`Generated ${path.relative(extRoot, typesTarget)} from canonical schema.`);
} catch (err) {
  console.error("Failed to generate types:", err);
  process.exit(1);
}

