// Re-measures 500 Nodes with Excalidraw memoised (memo=1) vs re-rendered by every frame message (memo=0).
import { chromium } from "playwright-core";
import { mkdtempSync, rmSync, writeFileSync, mkdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { serve } from "./serve.mjs";

const EDGE = "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe";
const root = fileURLToPath(new URL("..", import.meta.url));
mkdirSync(`${root}bench/out/profiles`, { recursive: true });
const server = await serve(`${root}dist`, 5193);
const r1 = (x) => (x == null ? null : Math.round(x * 10) / 10);

async function open(params, dpr = 1) {
  const dir = mkdtempSync(`${root}bench/out/profiles/m-`);
  const ctx = await chromium.launchPersistentContext(dir, { executablePath: EDGE, headless: false, viewport: { width: 1600, height: 900 }, deviceScaleFactor: dpr, args: ["--no-first-run", "--no-default-browser-check", "--window-position=0,0", "--enable-precise-memory-info"] });
  const page = ctx.pages()[0];
  await page.goto(`http://127.0.0.1:5193/?${new URLSearchParams(params)}`);
  await page.waitForFunction(() => window.__ready === true, null, { timeout: 60000 });
  if (params.charts !== "0") await page.waitForFunction(() => window.__dashReady === true, null, { timeout: 30000 });
  await page.waitForTimeout(4000);
  return { page, close: async () => { await ctx.close(); rmSync(dir, { recursive: true, force: true }); } };
}
const fmt = (m) => ({ fps: r1(m.fps), frameP95: r1(m.frameP95), frameP99: r1(m.frameP99), over33: m.framesOver33, longTasks: m.longTasks, longTaskMax: r1(m.longTaskMaxMs), dots: m.dots, achievedX: r1(m.worker.achieved) });
const measure = async (page, ms = 6000) => fmt(await page.evaluate((ms) => window.__bench.measure(ms), ms));

const rows = [];
for (const memo of ["0", "1"]) {
  for (const charts of ["0", "30"]) {
    const b = await open({ domains: 50, charts, probes: 5000, budget: 100000, memo, dashboard: charts === "0" ? "0" : "1" });
    const normal = await measure(b.page);
    await b.page.evaluate(() => window.__bench.setView({ scrollX: 20, scrollY: 20, zoom: 0.12 }));
    await b.page.waitForTimeout(500);
    const fit = await measure(b.page);
    await b.page.evaluate(() => window.__bench.set({ type: "speed", speed: 100 }));
    await b.page.waitForTimeout(1500);
    const fit100 = await measure(b.page, 5000);
    rows.push({ memo, charts, normal, fit, fit100 });
    console.log(JSON.stringify(rows.at(-1)));
    await b.close();
  }
}
// Pan/zoom at DPR 2, memoised.
{
  const b = await open({ domains: 50, charts: "30", probes: 5000, budget: 100000, memo: "1" }, 2);
  await b.page.mouse.move(700, 350);
  let worst = 0;
  const pan = (async () => {
    for (let i = 0; i < 40; i++) {
      if (i % 10 < 5) await b.page.mouse.wheel(i % 2 ? 60 : -45, 35);
      else { await b.page.keyboard.down("Control"); await b.page.mouse.wheel(0, i % 2 ? 40 : -40); await b.page.keyboard.up("Control"); }
      await b.page.waitForTimeout(120);
      worst = Math.max(worst, (await b.page.evaluate(() => window.__bench.alignment())).worstPx);
    }
  })();
  const m = await measure(b.page, 5000);
  await pan;
  await b.page.screenshot({ path: `${root}results/pan-dpr2-memo.png` });
  rows.push({ memo: "1", dpr: 2, pan: true, worstAlignPx: worst, ...m });
  console.log(JSON.stringify(rows.at(-1)));
  await b.close();
}
writeFileSync(`${root}results/memo.json`, JSON.stringify(rows, null, 1));
server.close();
