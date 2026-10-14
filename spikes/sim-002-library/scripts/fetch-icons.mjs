// Stand-in for the app's first-use icon download (sim ticket 015): fetch the pinned AWS icon
// package, check its sha256, and extract only the files the manifest lists, as <key>.svg.
// Output goes to public/icons/aws/ (gitignored: the icons are never committed or bundled).
// Usage: node scripts/fetch-icons.mjs [path-to-already-downloaded.zip]
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, copyFileSync, rmSync, writeFileSync, existsSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { parse } from "yaml";

const root = fileURLToPath(new URL("..", import.meta.url));
const manifest = parse(readFileSync(join(root, "../../internal/catalog/icons/aws.yaml"), "utf8"));
const work = mkdtempSync(join(tmpdir(), "aws-icons-"));
const zip = process.argv[2] ?? join(work, "pkg.zip");

if (!process.argv[2]) {
  const res = await fetch(manifest.url);
  if (!res.ok) throw new Error(`download failed: ${res.status}`);
  writeFileSync(zip, Buffer.from(await res.arrayBuffer()));
}
const sha = createHash("sha256").update(readFileSync(zip)).digest("hex");
if (sha !== manifest.sha256) throw new Error(`checksum mismatch: ${sha} (want ${manifest.sha256})`);

const out = join(root, "public/icons/aws");
mkdirSync(out, { recursive: true });
const paths = Object.values(manifest.files);
// Extract only the listed files (the package has 3,620 SVGs and 4,262 PNGs).
execFileSync("unzip", ["-q", "-o", zip, ...paths, "-d", work]);
let n = 0;
for (const [key, p] of Object.entries(manifest.files)) {
  const src = join(work, p);
  if (!existsSync(src)) throw new Error(`missing in package: ${p}`);
  copyFileSync(src, join(out, `${key}.svg`));
  n++;
}
rmSync(work, { recursive: true, force: true });
console.log(`verified ${manifest.version} (${sha.slice(0, 12)}…), extracted ${n} icons to public/icons/aws/`);
