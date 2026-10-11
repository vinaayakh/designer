// CI check (ticket 017): every Component Kind and group kind names a Tabler icon that exists.
import { readFileSync, existsSync } from "node:fs";
const yaml = readFileSync(new URL("../../internal/catalog/data/kinds.yaml", import.meta.url), "utf8");
const names = [...yaml.matchAll(/icon: ([a-z0-9-]+)/g)].map((m) => m[1]);
const missing = names.filter((n) => !existsSync(new URL(`../node_modules/@tabler/icons/icons/outline/${n}.svg`, import.meta.url)));
if (missing.length) {
  console.error("Unknown Tabler icons:", missing.join(", "));
  process.exit(1);
}
console.log(`${new Set(names).size} Tabler icons OK`);
