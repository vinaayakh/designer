// Drives headed Microsoft Edge (system install, via playwright-core) against the built spike.
// Usage: node bench/run.mjs   (after `npx vite build`)
import { chromium } from "playwright-core";
import { mkdtempSync, rmSync, writeFileSync, mkdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { serve } from "./serve.mjs";

const EDGE = "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe";
const root = fileURLToPath(new URL("..", import.meta.url));
const OUT = `${root}bench/out`;
mkdirSync(`${OUT}/profiles`, { recursive: true });
const PORT = 5191;
const server = await serve(`${root}dist`, PORT);
const r1 = (x) => (x == null ? null : Math.round(x * 10) / 10);

async function open(params, { dpr = 1 } = {}) {
  const dir = mkdtempSync(`${OUT}/profiles/p-`);
  const ctx = await chromium.launchPersistentContext(dir, {
    executablePath: EDGE,
    headless: false, // real GPU raster + compositor
    viewport: { width: 1600, height: 900 },
    deviceScaleFactor: dpr,
    args: ["--enable-precise-memory-info", "--no-first-run", "--no-default-browser-check", "--window-position=0,0"],
  });
  const page = ctx.pages()[0] || (await ctx.newPage());
  await page.goto(`http://127.0.0.1:${PORT}/?${new URLSearchParams(params)}`);
  await page.waitForFunction(() => window.__ready === true, null, { timeout: 60000 });
  if (params.dashboard !== "0") await page.waitForFunction(() => window.__dashReady === true, null, { timeout: 30000 });
  await page.waitForTimeout(4000); // let the sim warm up and fonts land
  return { page, close: async () => { await ctx.close(); rmSync(dir, { recursive: true, force: true }); } };
}

const measure = (page, ms = 6000) => page.evaluate((ms) => window.__bench.measure(ms), ms);
const fmt = (m) => ({
  fps: r1(m.fps), frameP95: r1(m.frameP95), frameP99: r1(m.frameP99), over33: m.framesOver33, longTasks: m.longTasks, longTaskMax: r1(m.longTaskMaxMs),
  dots: m.dots, heapMB: r1(m.heapMB), achievedX: r1(m.worker.achieved), lagging: m.worker.lagging, tickMean: r1(m.worker.tickMean * 100) / 100, tickP99: r1(m.worker.tickP99), probes: Math.round(m.worker.probesPerSec),
});
// Fit the whole diagram on screen: the worst case for dots and heat.
const fitAll = (page) => page.evaluate(() => { window.__bench.setView({ scrollX: 20, scrollY: 20, zoom: window.__bench.nodes > 100 ? 0.12 : 0.35 }); });

const results = {};

// A: size × charts at 1×, default view and zoomed out to fit everything.
results.size = [];
for (const domains of [5, 20, 50]) {
  for (const charts of [0, 30]) {
    const b = await open({ domains, charts, probes: 5000, dashboard: charts ? "1" : "0" });
    const normal = fmt(await measure(b.page));
    await fitAll(b.page);
    await b.page.waitForTimeout(500);
    const fit = fmt(await measure(b.page));
    const align = await b.page.evaluate(() => window.__bench.alignment());
    if (domains === 50 && charts === 30) await b.page.screenshot({ path: `${root}results/fit-500.png` });
    if (domains === 5 && charts === 30) await b.page.screenshot({ path: `${root}results/sim-mode-50.png` });
    results.size.push({ nodes: domains * 10, charts, normal, fit, align });
    console.log(JSON.stringify(results.size.at(-1)));
    await b.close();
  }
}

// B: speed. Fixed 5k probes per simulated second vs a 100k probes-per-real-second budget.
results.speed = [];
for (const domains of [20, 50]) {
  for (const budget of [0, 100000]) {
    const b = await open({ domains, charts: 30, probes: 5000, budget });
    for (const speed of [1, 10, 100]) {
      await b.page.evaluate((s) => window.__bench.set({ type: "speed", speed: s }), speed);
      await b.page.waitForTimeout(1500);
      results.speed.push({ nodes: domains * 10, budget, speed, ...fmt(await measure(b.page, 5000)) });
      console.log(JSON.stringify(results.speed.at(-1)));
    }
    await b.close();
  }
}

// C: pan and zoom with the wheel while running; overlay alignment checked after each gesture.
results.pan = [];
for (const dpr of [1, 2]) {
  const b = await open({ domains: 50, charts: 30, probes: 5000 }, { dpr });
  await b.page.mouse.move(700, 350);
  let worst = 0;
  const panning = (async () => {
    for (let i = 0; i < 40; i++) {
      if (i % 10 < 5) await b.page.mouse.wheel(i % 2 ? 60 : -45, 35);
      else {
        await b.page.keyboard.down("Control");
        await b.page.mouse.wheel(0, i % 2 ? 40 : -40);
        await b.page.keyboard.up("Control");
      }
      await b.page.waitForTimeout(120);
      const a = await b.page.evaluate(() => window.__bench.alignment());
      worst = Math.max(worst, a.worstPx);
    }
  })();
  const m = fmt(await measure(b.page, 5000));
  await panning;
  if (dpr === 2) await b.page.screenshot({ path: `${root}results/pan-dpr2.png` });
  results.pan.push({ dpr, worstAlignPx: r1(worst), ...m });
  console.log(JSON.stringify(results.pan.at(-1)));
  await b.close();
}

writeFileSync(`${root}results/browser.json`, JSON.stringify(results, null, 1));
server.close();
console.log("done");
