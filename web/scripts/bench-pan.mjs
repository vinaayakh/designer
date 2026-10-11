// Pan and zoom benchmark for a running Designer (Microsoft Edge via playwright-core).
//
//   node scripts/bench-pan.mjs [url=http://127.0.0.1:4717/] [--throttle 4] [--dpr 2] [--rough] [--hz 120] [--no-snapshot]
//
// Opens the page, fits the diagram, then measures every animation frame while it
//   1. pans with the wheel at a trackpad's event rate (--hz per second, 2 per frame at 120),
//   2. zooms with ctrl+wheel (pinch),
//   3. drags with the hand tool,
//   4. pans at 1x zoom (reading distance).
// --throttle slows the CPU (4 ≈ a mid-range laptop). Saving is blocked, so any project is safe.
// For readable profiles, build with `npx vite build --minify false` first.
import { chromium } from "playwright-core";

const args = process.argv.slice(2);
const opt = (name, def) => { const i = args.indexOf(`--${name}`); return i >= 0 ? args[i + 1] : def; };
const url = args.find((a) => a.startsWith("http")) ?? "http://127.0.0.1:4717/";
const throttle = +opt("throttle", 1), dpr = +opt("dpr", 1), hz = +opt("hz", 120);

const browser = await chromium.launch({ channel: "msedge", headless: true });
const ctx = await browser.newContext({ viewport: { width: 1500, height: 900 }, deviceScaleFactor: dpr });
if (args.includes("--no-snapshot")) await ctx.addInitScript(() => localStorage.setItem("designer.snapshotPan", "off"));
const page = await ctx.newPage();
await page.route("**/api/versions", (r) => (r.request().method() === "POST" ? r.fulfill({ status: 409, body: "{}" }) : r.continue()));
await page.route("**/api/rough", (r) => (r.request().method() === "PUT" ? r.fulfill({ status: 409, body: "{}" }) : r.continue()));
await page.goto(url);
await page.waitForFunction(() => !!window.__designerCanvas, null, { timeout: 30000 });
await page.waitForTimeout(3000);
if (args.includes("--rough")) { await page.click("button:has-text('Rough Draft')"); await page.waitForTimeout(2000); }

const cdp = await ctx.newCDPSession(page);
await cdp.send("Performance.enable");
if (throttle > 1) await cdp.send("Emulation.setCPUThrottlingRate", { rate: throttle });
const scene = await page.evaluate(() => {
  const a = window.__designerCanvas;
  const els = a.getSceneElements().filter((e) => !e.isDeleted);
  a.setViewport({ target: els, fit: "scale-down", offsets: { ui: true }, animation: false });
  return { elements: els.length, zoom: +a.getAppState().zoom.value.toFixed(2) };
});
await page.waitForTimeout(800);
const box = (await page.locator("canvas.excalidraw__canvas.interactive").boundingBox()) ?? { x: 0, y: 60, width: 1100, height: 800 };
const cx = box.x + box.width / 2, cy = box.y + box.height / 2;
await page.mouse.move(cx, cy);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const metric = async (k) => (await cdp.send("Performance.getMetrics")).metrics.find((x) => x.name === k)?.value ?? 0;
// Windows timers tick every ~16 ms, so events are sent in batches of hz/60 per tick.
const perTick = Math.max(1, Math.round(hz / 60));
const wheel = (dx, dy, ctrl = false) => cdp.send("Input.dispatchMouseEvent", { type: "mouseWheel", x: cx, y: cy, deltaX: dx, deltaY: dy, modifiers: ctrl ? 2 : 0 });

async function phase(name, run) {
  await page.evaluate(() => {
    const gen = (window.__gen = (window.__gen ?? 0) + 1); // one frame loop per phase
    window.__f = []; window.__long = [];
    const tick = (t) => { if (window.__gen !== gen) return; window.__f.push(t); requestAnimationFrame(tick); };
    requestAnimationFrame(tick);
    try { window.__po?.disconnect(); window.__po = new PerformanceObserver((l) => { for (const e of l.getEntries()) window.__long.push(e.duration); }); window.__po.observe({ type: "longtask" }); } catch {}
  });
  const s0 = await metric("ScriptDuration"), t0 = Date.now();
  await run();
  await sleep(300);
  const wall = Date.now() - t0, s1 = await metric("ScriptDuration");
  const { f, long } = await page.evaluate(() => { window.__gen++; return { f: window.__f, long: window.__long }; });
  const gaps = f.slice(1).map((t, i) => t - f[i]).sort((a, b) => a - b);
  const pct = (p) => (gaps[Math.min(gaps.length - 1, Math.floor(p * gaps.length))] ?? 0).toFixed(1);
  const dropped = gaps.reduce((s, g) => s + (g > 25 ? Math.round(g / 16.7) - 1 : 0), 0);
  console.log(`  ${name.padEnd(14)} fps ${(gaps.length / (wall / 1000)).toFixed(1).padStart(5)}  p50 ${pct(0.5)}  p95 ${pct(0.95)}  p99 ${pct(0.99)}  max ${(gaps.at(-1) ?? 0).toFixed(0).padStart(4)}ms  dropped ${String(dropped).padStart(3)}  long tasks ${long.length} (${Math.round(long.reduce((a, b) => a + b, 0))}ms)  script ${Math.round((s1 - s0) * 1000)}ms (${(((s1 - s0) * 1000) / wall * 100).toFixed(0)}% of main thread)`);
}

console.log(`${url}  snapshot ${args.includes("--no-snapshot") ? "off" : "on"}  elements ${scene.elements}  fit zoom ${scene.zoom}  throttle ${throttle}x  dpr ${dpr}  wheel ${hz}/s`);
await phase("wheel pan", async () => {
  for (let i = 0; i < 240; i++) {
    const dir = Math.floor(i / 60) % 4;
    for (let k = 0; k < perTick; k++) void wheel([24, 0, -24, 0][dir] / perTick, [0, 20, 0, -20][dir] / perTick);
    await sleep(16);
  }
});
await phase("pinch zoom", async () => {
  for (let i = 0; i < 160; i++) {
    for (let k = 0; k < perTick; k++) void wheel(0, i < 80 ? -3 : 3, true);
    await sleep(16);
  }
});
await page.evaluate(() => window.__designerCanvas.setActiveTool({ type: "hand" }));
await phase("hand drag", async () => {
  await page.mouse.down();
  for (let i = 0; i < 180; i++) {
    const a = (i / 180) * Math.PI * 4;
    await page.mouse.move(cx + Math.cos(a) * 250, cy + Math.sin(a) * 150);
  }
  await page.mouse.up();
});
await page.evaluate(() => window.__designerCanvas.setActiveTool({ type: "selection" }));
await page.evaluate(() => window.__designerCanvas.updateScene({ appState: { zoom: { value: 1 }, scrollX: -400, scrollY: -300 } }));
await sleep(500);
await phase("pan at 1x", async () => {
  for (let i = 0; i < 180; i++) {
    for (let k = 0; k < perTick; k++) void wheel((i % 90 < 45 ? 24 : -24) / perTick, 0);
    await sleep(16);
  }
});
await browser.close();
