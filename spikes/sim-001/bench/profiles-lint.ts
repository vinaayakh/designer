// Prototype of the Profile lint test from sim ticket 004: parse every Profile, check that ranges are
// ordered, sources are complete, and that each library Component marked `full` has a Profile.
// Run: node bench/profiles-lint.ts
import { readdirSync, readFileSync } from "node:fs";
import { parse } from "yaml";

const root = new URL("../../../internal/catalog/", import.meta.url);
const dir = new URL("profiles/", root);
const errors: string[] = [];
const warn: string[] = [];
const kinds = new Set<string>();
let ranges = 0, verify = 0, nullPrices = 0;

function walk(v: any, path: string, file: string) {
  if (v && typeof v === "object" && !Array.isArray(v)) {
    if ("low" in v && "typical" in v && "high" in v) {
      ranges++;
      if (!(v.low <= v.typical && v.typical <= v.high)) errors.push(`${file} ${path}: range not ordered (${v.low}, ${v.typical}, ${v.high})`);
      return;
    }
    if (path.endsWith(".price")) {
      if (v.verify) verify++;
      if (!v.as_of) errors.push(`${file} ${path}: price without as_of`);
    }
    for (const [k, x] of Object.entries(v)) walk(x, `${path}.${k}`, file);
  } else if (v === null && path.endsWith(".price")) nullPrices++;
}

for (const f of readdirSync(dir).filter((f) => f.endsWith(".yaml"))) {
  let p: any;
  try {
    p = parse(readFileSync(new URL(f, dir), "utf8"));
  } catch (e) {
    errors.push(`${f}: ${e}`);
    continue;
  }
  if (`${p.kind}.yaml` !== f) errors.push(`${f}: kind ${p.kind} doesn't match the file name`);
  kinds.add(p.kind);
  for (const k of ["as_of", "unit", "capacity", "latency"]) if (!p[k]) errors.push(`${f}: missing ${k}`);
  if (!p.capacity?.model) errors.push(`${f}: capacity.model missing`);
  if (!p.capacity?.confidence) warn.push(`${f}: capacity.confidence missing`);
  for (const s of p.sources ?? []) if (!s.url || !s.copy || !s.used_for) errors.push(`${f}: source ${s.id} incomplete`);
  walk(p, "", f);
}

const lib = parse(readFileSync(new URL("library/aws.yaml", root), "utf8"));
const cat = parse(readFileSync(new URL("data/aws.yaml", root), "utf8"));
const svcKind = new Map(cat.services.map((s: any) => [s.id, s.kind]));
for (const c of lib.components) {
  if (c.sim === "off-path" || c.place === "group" || c.place === "source") continue; // full and hop need a Profile; sources make traffic
  const k = c.kind ?? svcKind.get(c.service);
  if (!k) warn.push(`library ${c.id}: service ${c.service} not in the catalog yet`);
  else if (!kinds.has(k)) errors.push(`library ${c.id}: marked ${c.sim} but no Profile for ${k}`);
}

console.log(`${kinds.size} Profiles, ${ranges} ranges, ${verify} prices to verify, ${nullPrices} Services without a price`);
for (const w of warn) console.log("warn:", w);
for (const e of errors) console.log("ERROR:", e);
process.exit(errors.length ? 1 : 0);
