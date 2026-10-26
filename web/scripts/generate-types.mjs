import { writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderTypes, syncContract, typesTarget } from "./contract.mjs";

await syncContract();
const types = await renderTypes();
await writeFile(typesTarget, types, "utf8");
console.log(`Generated ${path.relative(path.dirname(fileURLToPath(import.meta.url)), typesTarget)}.`);
