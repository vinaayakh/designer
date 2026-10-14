// Spike page: Sim Mode over a generated Production-like diagram.
// URL params: domains (×10 Nodes), charts (number of dashboard charts), probes
// (per simulated second at 1×), budget (probe budget per real second; 0 = fixed),
// traffic (rps), dashboard=0 to leave charts out.
import React, { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { Excalidraw, convertToExcalidrawElements, sceneCoordsToViewportCoords, viewportCoordsToSceneCoords } from "@excalidraw/excalidraw";
import "@excalidraw/excalidraw/index.css";
import { buildSystem, type System } from "./system.ts";
import { FN } from "./engine.ts";
import { Overlay, pathFromArrow, type View } from "./overlay.ts";
import type { ChartSpec, Marker } from "./dashboard.ts";

const qp = new URLSearchParams(location.search);
const num = (k: string, d: number) => (qp.has(k) ? Number(qp.get(k)) : d);
const DOMAINS = num("domains", 20);
const CHARTS = num("charts", 12);
const PROBES = num("probes", 5000);
const BUDGET = num("budget", 0);
const TRAFFIC = num("traffic", 200 * DOMAINS);
const DASH = qp.get("dashboard") !== "0";

const KIND_BG: Record<string, string> = {
  network: "#e7f5ff", compute: "#fff4e6", data: "#ebfbee", messaging: "#f3f0ff", external: "#f1f3f5",
};

function sceneFor(sys: System) {
  const skel: any[] = [];
  for (const n of sys.nodes) {
    skel.push({
      type: "rectangle", id: n.id, x: n.x, y: n.y, width: n.w, height: n.h, roughness: 0, fillStyle: "solid",
      backgroundColor: KIND_BG[n.kind.split(".")[0]] ?? "#fff", strokeColor: "#495057", roundness: { type: 3 },
      customData: { kind: n.kind }, label: { text: `${n.label}\n${n.kind}`, fontSize: 14 },
    });
  }
  for (const e of sys.edges) {
    const a = sys.nodes[e.from], b = sys.nodes[e.to];
    const sx = a.x + a.w / 2, sy = a.y + a.h / 2, ex = b.x + b.w / 2, ey = b.y + b.h / 2;
    skel.push({
      type: "arrow", id: e.id, x: sx, y: sy, width: ex - sx, height: ey - sy, roughness: 0, strokeColor: "#868e96",
      strokeStyle: e.async ? "dashed" : "solid", start: { id: a.id }, end: { id: b.id },
    });
  }
  return convertToExcalidrawElements(skel, { regenerateIds: false }) as any[];
}

const fmtMs = (ms: number) => (!Number.isFinite(ms) ? "–" : ms >= 1000 ? `${(ms / 1000).toFixed(1)} s` : `${Math.round(ms)} ms`);
const fmtRps = (r: number) => (r >= 1000 ? `${(r / 1000).toFixed(1)}k` : `${Math.round(r)}`);

// memo=0 reproduces the first measurement: Excalidraw re-rendered by every 10 Hz frame message.
const MEMO = qp.get("memo") !== "0";

/** Excalidraw with props that never change, so live sim state never re-renders it. */
const Canvas = React.memo(function Canvas({ elements, api, overlay }: { elements: any[]; api: React.MutableRefObject<any>; overlay: React.MutableRefObject<Overlay | null> }) {
  const initialData = React.useMemo(() => ({ elements, appState: { viewModeEnabled: true, zoom: { value: 0.6 as any }, scrollX: 40, scrollY: 40 } }), [elements]);
  const onApi = React.useCallback((a: any) => (api.current = a), [api]);
  const onScroll = React.useCallback((scrollX: number, scrollY: number, zoom: { value: number }) => {
    if (overlay.current) overlay.current.view = { scrollX, scrollY, zoom: zoom.value };
  }, [overlay]);
  return <Excalidraw initialData={initialData} viewModeEnabled onExcalidrawAPI={onApi} onScrollChange={onScroll} />;
});

type Totals ={ t: number; traffic: number; cost: number; rate: number; err: number; p50: number; p95: number; p99: number; warming: number };

function App() {
  // Lazy initialisers: useRef(expr) would rebuild the scene on every render (10 Hz) and throw it away.
  const [sys] = useState<System>(() => buildSystem(DOMAINS));
  const [elements] = useState(() => sceneFor(sys));
  const worker = useRef<Worker | null>(null);
  const overlay = useRef<Overlay | null>(null);
  const api = useRef<any>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const stageRef = useRef<HTMLDivElement>(null);
  const dashRef = useRef<HTMLDivElement>(null);
  const markers = useRef<Marker[]>([]);
  const [totals, setTotals] = useState<Totals | null>(null);
  const [nodeLive, setNodeLive] = useState<Float32Array | null>(null);
  const [playing, setPlaying] = useState(true);
  const [speed, setSpeed] = useState(1);
  const [traffic, setTraffic] = useState(TRAFFIC);
  const [selected, setSelected] = useState(-1);
  const [ramp, setRamp] = useState<null | { from: number; broke?: string }>(null);
  const totalsRef = useRef<Totals | null>(null);

  const post = (m: any) => worker.current?.postMessage(m);
  const mark = (label: string) => totalsRef.current && markers.current.push({ t: totalsRef.current.t, label });

  // Worker, overlay and render loop.
  useEffect(() => {
    const w = new Worker(new URL("./sim.worker.ts", import.meta.url), { type: "module" });
    worker.current = w;
    w.postMessage({ type: "init", system: sys, traffic: TRAFFIC, probes: PROBES, probeBudget: BUDGET, seed: 1 });
    const byId = new Map(elements.map((e) => [e.id, e]));
    const paths = sys.edges.map((e) => {
      const el = byId.get(e.id);
      return el && el.points ? pathFromArrow(el) : null;
    });
    const ov = new Overlay(canvasRef.current!, sys.nodes, paths);
    overlay.current = ov;
    let dash: any = null;
    const statsWaiters = new Map<number, (s: any) => void>();
    w.onmessage = (ev) => {
      const m = ev.data;
      if (m.type === "frame") {
        ov.frame = { nodes: m.nodes, edges: m.edges };
        totalsRef.current = m;
        setTotals(m);
        setNodeLive(m.nodes);
      } else if (m.type === "series") dash?.update(m);
      else if (m.type === "stats") statsWaiters.get(m.id)?.(m);
    };
    let raf = 0;
    const frameTimes: number[] = [];
    let lastT = performance.now();
    const loop = (now: number) => {
      frameTimes.push(now - lastT);
      if (frameTimes.length > 2000) frameTimes.splice(0, 1000);
      lastT = now;
      ov.draw(now);
      raf = requestAnimationFrame(loop);
    };
    raf = requestAnimationFrame(loop);

    // Dashboard: lazy chunk, so Excalidraw-only users never load uPlot.
    if (DASH) {
      import("./dashboard.ts").then(({ createDashboard }) => {
        const specs: ChartSpec[] = [
          { kind: "latency", title: "All routes: response time", route: sys.routes.length, sloMs: 300 },
          { kind: "traffic", title: "All routes: traffic and errors", route: sys.routes.length },
        ];
        for (let i = 0; specs.length < CHARTS && i < sys.nodes.length; i++) {
          if (sys.nodes[i].kind.startsWith("compute") || sys.nodes[i].kind.startsWith("data")) specs.push({ kind: "node", title: sys.nodes[i].label, node: i });
        }
        dash = createDashboard(dashRef.current!, specs.slice(0, CHARTS), () => markers.current);
        w.postMessage({ type: "subscribe", nodes: specs.filter((s) => s.kind === "node").map((s: any) => s.node), routes: [sys.routes.length] });
        (window as any).__dashReady = true;
      });
    }

    // Benchmark hooks.
    let statsId = 0;
    const longTasks: number[] = [];
    try {
      new PerformanceObserver((l) => l.getEntries().forEach((e) => longTasks.push(e.duration))).observe({ type: "longtask", buffered: false });
    } catch {}
    (window as any).__bench = {
      nodes: sys.nodes.length,
      edges: sys.edges.length,
      async measure(ms: number) {
        frameTimes.length = 0;
        longTasks.length = 0;
        const s0 = await this.stats();
        await new Promise((r) => setTimeout(r, ms));
        const ft = [...frameTimes].sort((a, b) => a - b);
        const q = (p: number) => ft[Math.min(ft.length - 1, Math.floor(p * ft.length))];
        const s1 = await this.stats();
        void s0;
        return {
          fps: 1000 / (ft.reduce((a, b) => a + b, 0) / ft.length),
          frameP95: q(0.95),
          frameP99: q(0.99),
          framesOver33: ft.filter((x) => x > 33.4).length,
          longTasks: longTasks.length,
          longTaskMaxMs: Math.max(0, ...longTasks),
          dots: ov.dotsDrawn,
          worker: s1,
          heapMB: (performance as any).memory ? (performance as any).memory.usedJSHeapSize / 1048576 : null,
        };
      },
      stats() {
        const id = ++statsId;
        return new Promise((r) => {
          statsWaiters.set(id, r);
          w.postMessage({ type: "stats", id });
        });
      },
      set(m: any) {
        w.postMessage(m);
      },
      // Overlay position vs Excalidraw's own transform, for every Node on screen (CSS px).
      alignment() {
        const st = api.current.getAppState();
        let worst = 0, checked = 0;
        for (const n of sys.nodes) {
          const ours = ov.toScreen(n.x, n.y);
          const theirs = sceneCoordsToViewportCoords({ sceneX: n.x, sceneY: n.y }, st);
          const dx = ours.x + st.offsetLeft - theirs.x, dy = ours.y + st.offsetTop - theirs.y;
          worst = Math.max(worst, Math.hypot(dx, dy));
          checked++;
        }
        return { worstPx: worst, checked, zoom: st.zoom.value };
      },
      view: () => ov.view,
      setView(v: View) {
        api.current.updateScene({ appState: { scrollX: v.scrollX, scrollY: v.scrollY, zoom: { value: v.zoom } } });
      },
    };
    (window as any).__ready = true;
    return () => {
      cancelAnimationFrame(raf);
      w.terminate();
      dash?.destroy();
    };
  }, []);

  // Selection: the canvas is read-only in Sim Mode, so clicks are hit-tested here (as on the frozen Rough Draft).
  const onPointerDown = (e: React.PointerEvent) => {
    if (!api.current) return;
    const p = viewportCoordsToSceneCoords({ clientX: e.clientX, clientY: e.clientY }, api.current.getAppState());
    const hit = sys.nodes.findIndex((n) => p.x >= n.x && p.x <= n.x + n.w && p.y >= n.y && p.y <= n.y + n.h);
    setSelected(hit);
    if (overlay.current) overlay.current.selected = hit;
  };

  // Find breaking point: ramp traffic 5% every simulated 10 s until the p99 SLO or 1% errors break.
  useEffect(() => {
    if (!ramp || ramp.broke || !totals) return;
    if (totals.p99 > 300 || totals.err > 0.01) {
      const hot = sys.nodes
        .map((n, i) => ({ n, u: nodeLive ? nodeLive[i * FN] : 0 }))
        .filter((x) => x.u > 0.9)
        .sort((a, b) => b.u - a.u)
        .slice(0, 3)
        .map((x) => x.n.label);
      setRamp({ ...ramp, broke: `Broke at ≈${fmtRps(totals.traffic)} rps (p99 ${fmtMs(totals.p99)}, ${(totals.err * 100).toFixed(1)}% errors). Saturated: ${hot.join(", ") || "none"}` });
      mark("breaking point");
      return;
    }
    const id = setTimeout(() => {
      const next = Math.round(totals.traffic * 1.05);
      setTraffic(next);
      post({ type: "traffic", traffic: next });
    }, 10_000 / speed);
    return () => clearTimeout(id);
  }, [ramp, totals?.t]);

  const sel = selected >= 0 ? sys.nodes[selected] : null;
  const live = selected >= 0 && nodeLive ? nodeLive.subarray(selected * FN, selected * FN + FN) : null;
  const patch = (p: any, label: string) => {
    post({ type: "patch", node: selected, patch: p });
    mark(label);
  };
  const t = totals;
  const sloBad = t && (t.p99 > 300 || t.err > 0.01);

  return (
    <div className="app">
      <div className="bar">
        <strong>Sim Mode</strong>
        <span className="muted">Scenario: Normal day</span>
        <button onClick={() => { setPlaying(!playing); post({ type: "play", playing: !playing }); }}>{playing ? "❚❚ Pause" : "▶ Play"}</button>
        {[1, 10, 100].map((s) => (
          <button key={s} className={speed === s ? "on" : ""} onClick={() => { setSpeed(s); post({ type: "speed", speed: s }); }}>{s}×</button>
        ))}
        <label>
          Traffic <input type="range" min={2} max={5} step={0.01} value={Math.log10(traffic)} onChange={(e) => { const v = Math.round(10 ** Number(e.target.value)); setTraffic(v); post({ type: "traffic", traffic: v }); }} /> {fmtRps(traffic)} rps
        </label>
        <button onClick={() => { setRamp({ from: traffic }); mark("ramp start"); }}>Find breaking point</button>
        <span className="spacer" />
        <span className={sloBad ? "slo bad" : "slo ok"}>{sloBad ? "SLO broken" : "SLO met"}</span>
        <span className="muted">≈ simulated · {t ? `${Math.floor(t.t / 60)}:${String(Math.floor(t.t % 60)).padStart(2, "0")}` : ""}</span>
        <button className="apply" disabled>Apply to diagram…</button>
      </div>
      <div className="main">
        <div className="stage" ref={stageRef} onPointerDownCapture={onPointerDown}>
          {MEMO ? (
            <Canvas elements={elements} api={api} overlay={overlay} />
          ) : (
            <Excalidraw
              initialData={{ elements, appState: { viewModeEnabled: true, zoom: { value: 0.6 as any }, scrollX: 40, scrollY: 40 } }}
              viewModeEnabled
              onExcalidrawAPI={(a: any) => (api.current = a)}
              onScrollChange={(scrollX: number, scrollY: number, zoom: { value: number }) => {
                if (overlay.current) overlay.current.view = { scrollX, scrollY, zoom: zoom.value };
              }}
            />
          )}
          <canvas ref={canvasRef} className="overlay" />
          {ramp?.broke && (
            <div className="toast">
              {ramp.broke}
              <button onClick={() => setRamp(null)}>OK</button>
            </div>
          )}
        </div>
        <aside className="inspector">
          {!sel && <p className="muted">Select a Node to see it live and change its sizing.</p>}
          {sel && live && (
            <>
              <h3>{sel.label}</h3>
              <div className="muted">{sel.kind}</div>
              <dl>
                <dt>Traffic</dt><dd>{fmtRps(live[1])} rps</dd>
                <dt>Utilisation</dt><dd>{Math.round(live[0] * 100)}%</dd>
                <dt>Errors</dt><dd>{(live[2] * 100).toFixed(1)}%</dd>
                <dt>Queue</dt><dd>{fmtMs(live[6] * 1000)}</dd>
                <dt>Instances</dt><dd>{live[3]}{live[4] ? ` (+${live[4]} warming)` : ""}</dd>
              </dl>
              <div className="row">
                <button onClick={() => patch({ count: live[3] - 1 }, `${sel.label} −1`)}>− instance</button>
                <button onClick={() => patch({ count: live[3] + 1 }, `${sel.label} +1`)}>+ instance</button>
              </div>
              <label>Size
                <select defaultValue="1" onChange={(e) => patch({ capPerInstance: sel.capPerInstance * Number(e.target.value) }, `${sel.label} resized`)}>
                  <option value="1">2 vCPU · 4 GB</option>
                  <option value="2">4 vCPU · 8 GB</option>
                  <option value="4">8 vCPU · 16 GB</option>
                </select>
              </label>
              <div className="row">
                <button onClick={() => patch({ down: !live[5] }, `${sel.label} ${live[5] ? "restored" : "killed"}`)}>{live[5] ? "Restore" : "Kill Node"}</button>
              </div>
            </>
          )}
        </aside>
      </div>
      {DASH && (
        <div className="dash">
          <div className="tiles">
            <div className="tile"><span>Traffic</span><b>{t ? fmtRps(t.rate) : "–"} rps</b></div>
            <div className={`tile ${t && t.err > 0.01 ? "bad" : ""}`}><span>Errors</span><b>{t ? (t.err * 100).toFixed(2) : "–"}%</b></div>
            <div className="tile"><span>p50</span><b>{t ? fmtMs(t.p50) : "–"}</b></div>
            <div className={`tile ${t && t.p95 > 300 ? "bad" : ""}`}><span>p95</span><b>{t ? fmtMs(t.p95) : "–"}</b></div>
            <div className={`tile ${t && t.p99 > 300 ? "bad" : ""}`}><span>p99 (target 300 ms)</span><b>{t ? fmtMs(t.p99) : "–"}</b></div>
            <div className="tile"><span>Cost</span><b>{t ? `$${t.cost.toFixed(2)}/h` : "–"}</b></div>
          </div>
          <div className="charts" ref={dashRef} />
        </div>
      )}
    </div>
  );
}

const css = `
* { box-sizing: border-box; }
html, body, #root { margin: 0; height: 100%; font: 13px system-ui, sans-serif; color: #212529; }
.app { display: flex; flex-direction: column; height: 100%; }
.bar { display: flex; gap: 8px; align-items: center; padding: 6px 10px; border-bottom: 1px solid #dee2e6; background: #f8f9fa; }
.bar button { padding: 3px 8px; border: 1px solid #ced4da; background: #fff; border-radius: 4px; cursor: pointer; }
.bar button.on { background: #1c7ed6; color: #fff; border-color: #1c7ed6; }
.bar .apply { margin-left: 4px; }
.spacer { flex: 1; }
.muted { color: #868e96; }
.slo { padding: 2px 8px; border-radius: 10px; font-weight: 600; }
.slo.ok { background: #d3f9d8; color: #2b8a3e; } .slo.bad { background: #ffe3e3; color: #c92a2a; }
.main { flex: 1; display: flex; min-height: 0; }
.stage { position: relative; flex: 1; min-width: 0; }
.overlay { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; z-index: 3; }
.inspector { width: 260px; border-left: 1px solid #dee2e6; padding: 10px; overflow: auto; }
.inspector h3 { margin: 0 0 2px; }
.inspector dl { display: grid; grid-template-columns: auto 1fr; gap: 4px 10px; }
.inspector dd { margin: 0; text-align: right; font-variant-numeric: tabular-nums; }
.inspector .row { display: flex; gap: 6px; margin: 8px 0; }
.inspector select { width: 100%; margin-top: 4px; }
.toast { position: absolute; left: 50%; top: 12px; transform: translateX(-50%); z-index: 5; background: #fff; border: 1px solid #e03131; border-radius: 6px; padding: 8px 12px; box-shadow: 0 2px 8px rgba(0,0,0,.15); display: flex; gap: 10px; align-items: center; }
.dash { height: 300px; border-top: 1px solid #dee2e6; display: flex; flex-direction: column; }
.tiles { display: flex; gap: 8px; padding: 6px 10px; }
.tile { border: 1px solid #dee2e6; border-radius: 6px; padding: 4px 10px; min-width: 110px; }
.tile span { display: block; color: #868e96; font-size: 11px; } .tile b { font-size: 16px; font-variant-numeric: tabular-nums; }
.tile.bad { border-color: #e03131; } .tile.bad b { color: #c92a2a; }
.charts { flex: 1; display: flex; flex-wrap: wrap; gap: 4px 12px; overflow: auto; padding: 0 10px; align-content: flex-start; }
.chart .u-title { font-size: 11px; font-weight: 600; text-align: left; }
.chart .u-legend { font-size: 10px; }
`;
const style = document.createElement("style");
style.textContent = css;
document.head.appendChild(style);

createRoot(document.getElementById("root")!).render(<App />);
