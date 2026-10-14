// CPU-profiles the main thread for 5 s at 500 Nodes and prints the top functions by self time.
import { chromium } from "playwright-core";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { serve } from "./serve.mjs";

const root = fileURLToPath(new URL("..", import.meta.url));
const server = await serve(`${root}dist`, 5194);
const params = process.argv[2] || "domains=50&charts=0&dashboard=0&memo=1";
const dir = mkdtempSync(`${root}bench/out/profiles/c-`);
const ctx = await chromium.launchPersistentContext(dir, { executablePath: "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe", headless: false, viewport: { width: 1600, height: 900 }, args: ["--no-first-run", "--window-position=0,0"] });
const page = ctx.pages()[0];
await page.goto(`http://127.0.0.1:5194/?${params}`);
await page.waitForFunction(() => window.__ready === true, null, { timeout: 60000 });
await page.waitForTimeout(4000);
const cdp = await ctx.newCDPSession(page);
await cdp.send("Profiler.enable");
await cdp.send("Profiler.setSamplingInterval", { interval: 200 });
await cdp.send("Profiler.start");
await page.waitForTimeout(5000);
const { profile } = await cdp.send("Profiler.stop");
const byId = new Map(profile.nodes.map((n) => [n.id, n]));
const self = new Map();
const dt = profile.timeDeltas;
profile.samples.forEach((id, i) => {
  const n = byId.get(id);
  const f = n.callFrame;
  const key = `${f.functionName || "(anon)"} ${f.url.split("/").pop()}:${f.lineNumber}`;
  self.set(key, (self.get(key) || 0) + (dt[i] || 0) / 1000);
});
const top = [...self.entries()].sort((a, b) => b[1] - a[1]).slice(0, 25);
for (const [k, ms] of top) console.log(`${ms.toFixed(1).padStart(8)} ms  ${k}`);
writeFileSync(`${root}bench/out/profile.json`, JSON.stringify(profile));
await ctx.close();
rmSync(dir, { recursive: true, force: true });
server.close();
