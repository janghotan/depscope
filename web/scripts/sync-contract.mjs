import { syncContract } from "./contract.mjs";

const { demoNames } = await syncContract();
console.log(`Synced canonical schema and ${demoNames.length} demo fixtures into src/generated/.`);
