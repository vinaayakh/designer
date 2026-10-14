// Sim Mode (sim tickets 006–009): the Production Diagram, read-only, with
// simulated traffic drawn over it. Changes go into a Scenario
// (sim/scenarios/baseline.yaml), never the diagram; "Apply to diagram" turns
// them into a Proposal reviewed in the usual overlay.
import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Excalidraw, viewportCoordsToSceneCoords } from "@excalidraw/excalidraw";
import { api } from "../api";
import type { Graph } from "../types";
import { FN } from "./engine.ts";
import { Overlay, pathFromArrow } from "./overlay.ts";
import { newScenario, resolve, sizingFromAttrs, type Profiles, type Resolved, type RunSummary, type Scenario } from "./resolve.ts";
import { scenarioChanges } from "./apply.ts";

const SLUG = "baseline";
type Live = { t: number; traffic: number; cost: number; rate: number; err: number; p50: number; p95: number; p99: number; nodes: Float32Array; edges: Float32Array };

const ms = (x: number) => (!Number.isFinite(x) ? "–" : x >= 1000 ? `${(x / 1000).toFixed(1)} s` : `${Math.round(x)} ms`);
const rps = (x: number) => (!Number.isFinite(x) ? "–" : x >= 1000 ? `${(x / 1000).toFixed(1)}k` : `${Math.round(x)}`);
const pct = (x: number) => `${Math.round(x * 100)}%`;
const usd = (x: number) => `$${x.toFixed(2)}/h`;

/** The diagram, read-only; props never change so live updates never re-render it (sim ticket 001). */
const Diagram = memo(function Diagram({ scene, onApi, onScroll }: { scene: any; onApi: (a: any) => void; onScroll: (x: number, y: number, z: number) => void }) {
  const initial = useMemo(() => ({ elements: scene.elements, files: scene.files, appState: { viewModeEnabled: true } }), [scene]);
  const scroll = useCallback((x: number, y: number, z: { value: number }) => onScroll(x, y, z.value), [onScroll]);
  return <Excalidraw initialData={initial} viewModeEnabled onExcalidrawAPI={onApi} onScrollChange={scroll} />;
});

export default function SimMode({ latest, answers, onClose, onApplied }: { latest: number; answers: Record<string, string>; onClose: () => void; onApplied: () => void }) {
  const [graph, setGraph] = useState<Graph | null>(null);
  const [scene, setScene] = useState<any>(null);
  const [profiles, setProfiles] = useState<Profiles | null>(null);
  const [sc, setSc] = useState<Scenario | null>(null);
  const [error, setError] = useState("");
  const [live, setLive] = useState<Live | null>(null);
  const [playing, setPlaying] = useState(true);
  const [speed, setSpeed] = useState(1);
  const [selected, setSelected] = useState<string | null>(null);
  const [running, setRunning] = useState<"" | "baseline" | "test">("");
  const [applying, setApplying] = useState(false);
  const exApi = useRef<any>(null);
  const canvas = useRef<HTMLCanvasElement>(null);
  const overlay = useRef<Overlay | null>(null);
  const liveWorker = useRef<Worker | null>(null);
  const runWorker = useRef<Worker | null>(null);

  // Load the diagram, the Profiles and the Scenario.
  useEffect(() => {
    Promise.all([api.graph("production"), api.version(latest), api.simProfiles(), api.scenario(SLUG)])
      .then(([g, v, p, s]) => {
        setGraph(g.graph);
        setScene(v.scene);
        setProfiles(p);
        setSc(s ? ({ sizing: {}, flow: {}, added: [], ...s } as Scenario) : newScenario(latest, answers));
      })
      .catch((e) => setError(e.message));
  }, [latest]);

  // Save the Scenario as it changes.
  useEffect(() => {
    if (!sc) return;
    const id = setTimeout(() => api.saveScenario(SLUG, sc).catch((e) => setError(e.message)), 400);
    return () => clearTimeout(id);
  }, [sc]);

  const r: Resolved | null = useMemo(() => (graph && profiles && sc ? resolve(graph, profiles, answers, sc) : null), [graph, profiles, sc]);

  // Geometry for the overlay: Node rectangles and arrow paths from the scene; added Nodes beside their store.
  useEffect(() => {
    if (!r || !scene || !canvas.current) return;
    const els = new Map<string, any>(scene.elements.map((e: any) => [e.id, e]));
    for (const n of r.system.nodes) {
      const el = els.get(n.id);
      if (el) Object.assign(n, { x: el.x, y: el.y, w: el.width, h: el.height });
    }
    for (const a of sc!.added) {
      const n = r.system.nodes.find((x) => x.id === a.ref)!;
      const to = els.get(a.to);
      if (n && to) Object.assign(n, { x: to.x, y: to.y - to.height - 60, w: to.width, h: to.height });
    }
    const paths = r.system.edges.map((e) => {
      const el = els.get(r.edgeIds[r.system.edges.indexOf(e)]);
      if (el?.points) return pathFromArrow(el);
      const a = r.system.nodes[e.from], b = r.system.nodes[e.to];
      return pathFromArrow({ x: a.x + a.w / 2, y: a.y + a.h / 2, points: [[0, 0], [b.x + b.w / 2 - a.x - a.w / 2, b.y + b.h / 2 - a.y - a.h / 2]] });
    });
    const ov = new Overlay(canvas.current, r.system.nodes, paths);
    if (overlay.current) ov.view = overlay.current.view;
    overlay.current = ov;
  }, [r, scene]);

  // The live worker restarts on every Scenario change; headless runs use a second worker.
  useEffect(() => {
    if (!r) return;
    const w = liveWorker.current ?? new Worker(new URL("./worker.ts", import.meta.url), { type: "module" });
    liveWorker.current = w;
    w.onmessage = (e) => {
      if (e.data.type !== "frame") return;
      setLive(e.data);
      if (overlay.current) overlay.current.frame = { nodes: e.data.nodes, edges: e.data.edges };
    };
    w.postMessage({ type: "init", system: r.system, seed: sc!.seed, traffic: sc!.trafficRps || r.trafficRps });
    w.postMessage({ type: "speed", speed });
    w.postMessage({ type: "play", playing });
  }, [r]);
  useEffect(() => () => { liveWorker.current?.terminate(); runWorker.current?.terminate(); }, []);

  useEffect(() => {
    let raf = 0;
    const loop = (t: number) => {
      overlay.current?.draw(t);
      raf = requestAnimationFrame(loop);
    };
    raf = requestAnimationFrame(loop);
    return () => cancelAnimationFrame(raf);
  }, []);

  const onScroll = useCallback((x: number, y: number, z: number) => {
    if (overlay.current) overlay.current.view = { scrollX: x, scrollY: y, zoom: z };
  }, []);
  const onApi = useCallback((a: any) => {
    exApi.current = a;
    if (!a) return; // Excalidraw passes null when it unmounts
    setTimeout(() => a.setViewport({ target: a.getSceneElements().filter((e: any) => !e.isDeleted), fit: "scale-down", offsets: { ui: true }, animation: false }), 50);
  }, []);

  const pick = (e: React.PointerEvent) => {
    if (!exApi.current || !r) return;
    const p = viewportCoordsToSceneCoords({ clientX: e.clientX, clientY: e.clientY }, exApi.current.getAppState());
    const hit = r.system.nodes.find((n) => p.x >= n.x && p.x <= n.x + n.w && p.y >= n.y && p.y <= n.y + n.h && !n.kind.startsWith("external."));
    setSelected(hit?.id ?? null);
    if (overlay.current) overlay.current.selected = hit ? r.system.nodes.indexOf(hit) : -1;
  };

  const update = (f: (s: Scenario) => void) => setSc((s) => {
    const n = structuredClone(s!);
    f(n);
    return n;
  });

  const run = (which: "baseline" | "test") => {
    if (!r || running) return;
    setRunning(which);
    const w = runWorker.current ?? new Worker(new URL("./worker.ts", import.meta.url), { type: "module" });
    runWorker.current = w;
    w.onmessage = (e) => {
      if (e.data.type !== "summary") return;
      const s: RunSummary = e.data.summary;
      update((x) => (which === "baseline" ? ((x.baseline = s), delete x.last) : (x.last = s)));
      setRunning("");
    };
    w.postMessage({ type: "run", id: which, system: r.system, opts: { seed: sc!.seed, trafficRps: sc!.trafficRps || r.trafficRps, sloMs: r.sloMs, costExcludes: r.costExcludes } });
  };

  const apply = async () => {
    if (!graph || !sc || !r) return;
    const { changes, summary } = scenarioChanges(graph, sc, r);
    if (!changes.length) return setError("Nothing to apply: the Scenario matches the diagram.");
    setApplying(true);
    try {
      const why = sc.baseline && sc.last
        ? `Tested in Sim Mode at ${rps(sc.last.trafficRps)} rps: breaking point ${sc.baseline.breakRps ?? "none"} → ${sc.last.breakRps ?? "none"} rps, p99 ${ms(sc.baseline.p99)} → ${ms(sc.last.p99)}.`
        : "Tested in Sim Mode.";
      await api.simApply(`Scenario “${sc.name}” (built on v${sc.base})`, { title: `Sim Mode: ${summary}`, summary: why, base_version: sc.base, changes } as any);
      onApplied();
    } catch (e: any) {
      setError(e.message);
      setApplying(false);
    }
  };

  // The diagram moved on (an applied Scenario was accepted): keep only what still differs.
  const rebase = () => update((s) => {
    for (const [id, ov] of Object.entries(s.sizing)) {
      const n = graph!.nodes.find((x) => x.id === id);
      const now = sizingFromAttrs(n?.attrs);
      const same = n && (ov.count == null || ov.count === now.count) && (!ov.size || ov.size === now.size) && (ov.scale === undefined || JSON.stringify(ov.scale) === JSON.stringify(now.scale ?? null)) && (!ov.simAs || ov.simAs === n.attrs?.simulate_as);
      if (!n || same) delete s.sizing[id];
    }
    s.added = s.added.filter((a) => !graph!.nodes.some((n) => n.kind === "data.cache" && n.label === a.label));
    s.base = latest;
  });

  if (error && !r) return <div className="sim-shell"><div className="sim-error">{error}<button onClick={onClose}>Close</button></div></div>;
  if (!r || !sc || !scene) return <div className="sim-shell"><div className="sim-loading">Loading Sim Mode…</div></div>;

  const idx = selected ? r.system.nodes.findIndex((n) => n.id === selected) : -1;
  const node = idx >= 0 ? r.system.nodes[idx] : null;
  const ln = live && idx >= 0 ? live.nodes.subarray(idx * FN, idx * FN + FN) : null;
  const sz = selected ? r.sizing[selected] : undefined;
  const ov = selected ? sc.sizing[selected] ?? {} : {};
  const gnode = graph!.nodes.find((n) => n.id === selected);
  const traffic = sc.trafficRps || r.trafficRps;
  const bad = live && (live.p99 > r.sloMs || live.err > 0.01);
  const stale = sc.base !== latest;
  const pending = scenarioChanges(graph!, sc, r).changes.length;
  const setSizing = (patch: Partial<Scenario["sizing"][string]>) => update((s) => (s.sizing[selected!] = { ...s.sizing[selected!], ...patch }));
  const callers = selected ? r.system.edges.filter((e) => r.system.nodes[e.to].id === selected && !r.system.nodes[e.from].kind.startsWith("external.")) : [];

  return (
    <div className="sim-shell">
      <header className="sim-bar">
        <strong>Sim Mode</strong>
        <span className="muted small">Scenario “{sc.name}” · built on v{sc.base}</span>
        {stale && <button className="small" onClick={rebase} title={`The diagram is now v${latest}`}>Rebase on v{latest}</button>}
        <button onClick={() => { setPlaying(!playing); liveWorker.current?.postMessage({ type: "play", playing: !playing }); }}>{playing ? "Pause" : "Play"}</button>
        {[1, 10, 100].map((s) => (
          <button key={s} className={speed === s ? "primary" : ""} onClick={() => { setSpeed(s); liveWorker.current?.postMessage({ type: "speed", speed: s }); }}>{s}×</button>
        ))}
        <label className="sim-traffic">Traffic
          <input type="range" min={1} max={5} step={0.01} value={Math.log10(traffic)}
            onChange={(e) => { const v = Math.round(10 ** +e.target.value); update((s) => (s.trafficRps = v)); }} />
          <b>{rps(traffic)} rps</b>
        </label>
        <span className="spacer" />
        <span className={`pill ${bad ? "bad" : "ok"}`}>{bad ? "Target missed" : "Targets met"}</span>
        <span className="muted small">≈ simulated {live ? `${Math.floor(live.t / 60)}:${String(Math.floor(live.t % 60)).padStart(2, "0")}` : ""}</span>
        <button className="primary" disabled={!pending || applying || stale} onClick={apply}
          title={stale ? "Rebase first" : pending ? `${pending} change${pending === 1 ? "" : "s"} to review` : "No changes yet"}>
          Apply to diagram{pending ? ` (${pending})` : ""}
        </button>
        <button onClick={onClose}>Exit</button>
      </header>

      <div className="sim-main">
        <div className="sim-stage" onPointerDownCapture={pick}>
          <Diagram scene={scene} onApi={onApi} onScroll={onScroll} />
          <canvas ref={canvas} className="sim-overlay" />
        </div>

        <aside className="sim-side">
          {!node && (
            <>
              <h3>Scenario</h3>
              <p className="small muted">Click a component to see it live and change it. Changes stay in this Scenario until you apply them.</p>
              {error && <div className="callout small">{error}</div>}
              <h4>Components, busiest first</h4>
              <ul className="small sim-list sim-components">
                {r.system.nodes
                  .map((n, i) => ({ n, i, u: live ? live.nodes[i * FN] : 0 }))
                  .filter(({ n }) => !n.kind.startsWith("external.") && !n.id.startsWith("+"))
                  .sort((a, b) => b.u - a.u)
                  .map(({ n, i, u }) => (
                    <li key={n.id}>
                      <button className="link" onClick={() => { setSelected(n.id); if (overlay.current) overlay.current.selected = i; }}>{n.label}</button>
                      <span className={u > 0.85 ? "bad" : "muted"}> {pct(u)}</span>
                    </li>
                  ))}
              </ul>
              <h4>How your diagram was read</h4>
              <ul className="small sim-list">
                {r.notes.map((n, i) => <li key={i}>{n.text}</li>)}
                {r.excluded.length > 0 && <li>Not on the request path: {r.excluded.map((x) => x.label).join(", ")}</li>}
              </ul>
              <h4>Changes in this Scenario</h4>
              {pending === 0 ? <p className="small muted">None yet.</p> : (
                <ul className="small sim-list">
                  {Object.entries(sc.sizing).map(([id, o]) => (
                    <li key={id}>{graph!.nodes.find((n) => n.id === id)?.label}: {[o.count != null && `${o.count} instances`, o.size, o.scale && `auto ${o.scale.min}-${o.scale.max}`, o.scale === null && "no autoscaling", o.simAs && `as ${o.simAs}`].filter(Boolean).join(", ")}
                      {" "}<button className="link" onClick={() => update((s) => delete s.sizing[id])}>undo</button></li>
                  ))}
                  {sc.added.map((a) => <li key={a.ref}>{a.label} in front of {graph!.nodes.find((n) => n.id === a.to)?.label} <button className="link" onClick={() => update((s) => (s.added = s.added.filter((x) => x.ref !== a.ref)))}>undo</button></li>)}
                </ul>
              )}
            </>
          )}
          {node && sz && (
            <>
              <div className="eyebrow">{gnode?.service ?? gnode?.serviceName ?? node.kind}</div>
              <h3>{node.label}</h3>
              {sz.simAs && <p className="small muted">Simulated as {sz.simAs}</p>}
              {ln && (
                <dl className="sim-dl">
                  <dt>Traffic</dt><dd>{rps(ln[1])} req/s</dd>
                  <dt>Busy</dt><dd>{pct(ln[0])}</dd>
                  <dt>Errors</dt><dd>{(ln[2] * 100).toFixed(1)}%</dd>
                  <dt>Queue</dt><dd>{ms(ln[6] * 1000)}</dd>
                  <dt>Instances</dt><dd>{ln[3]}{ln[4] ? ` (+${ln[4]} starting)` : ""}</dd>
                  <dt>Each handles</dt><dd>≈{rps(sz.capPerInstance)} req/s</dd>
                </dl>
              )}
              {["cpu", "rate"].includes(sz.model) && (
                <>
                  <h4>Sizing</h4>
                  <label className="sim-field">Instances
                    <input type="number" min={1} max={500} value={sz.count} onChange={(e) => setSizing({ count: Math.max(1, +e.target.value) })} />
                  </label>
                  {sz.sizes.length > 0 && (
                    <label className="sim-field">Size
                      <select value={sz.size} onChange={(e) => setSizing({ size: e.target.value })}>
                        {sz.sizes.map((s) => <option key={s.id} value={s.id}>{s.id}{s.vcpu ? ` · ${s.vcpu} vCPU` : ""}{s.ram_gb ? ` · ${s.ram_gb} GB` : ""}{s.price_hour != null ? ` · $${s.price_hour}/h` : ""}</option>)}
                      </select>
                    </label>
                  )}
                  <label className="sim-check">
                    <input type="checkbox" checked={!!sz.scale} onChange={(e) => setSizing({ scale: e.target.checked ? { min: sz.count, max: sz.count * 3, target: 0.6 } : null })} />
                    Autoscaling on requests per instance
                  </label>
                  {sz.scale && (
                    <div className="row small">
                      <label className="sim-field">Min <input type="number" min={1} value={sz.scale.min} onChange={(e) => setSizing({ scale: { ...sz.scale!, min: Math.max(1, +e.target.value) } })} /></label>
                      <label className="sim-field">Max <input type="number" min={1} value={sz.scale.max} onChange={(e) => setSizing({ scale: { ...sz.scale!, max: Math.max(1, +e.target.value) } })} /></label>
                      <label className="sim-field">Target % <input type="number" min={10} max={95} value={Math.round(sz.scale.target * 100)} onChange={(e) => setSizing({ scale: { ...sz.scale!, target: Math.min(0.95, Math.max(0.1, +e.target.value / 100)) } })} /></label>
                    </div>
                  )}
                </>
              )}
              {gnode?.kind && gnode.kind !== "compute.container" && (
                <label className="sim-check">
                  <input type="checkbox" checked={(ov.simAs ?? sz.simAs) === "compute.container"}
                    onChange={(e) => setSizing({ simAs: e.target.checked ? "compute.container" : undefined })} />
                  Self-hosted: simulate as a container service
                </label>
              )}
              {node.kind.startsWith("data.") && node.kind !== "data.cache" && callers.length > 0 && !sc.added.some((a) => a.to === selected) && (
                <button style={{ marginTop: 10 }} onClick={() => {
                  const from = r.system.nodes[callers[0].from].id;
                  update((s) => s.added.push({ ref: `+cache-${s.added.length + 1}`, label: `${node.label} cache`, from, to: selected!, hit: 0.9 }));
                }}>Add a cache in front (90% hits)</button>
              )}
              {r.system.edges.some((e) => r.system.nodes[e.from].id === selected) && <h4>Calls per request</h4>}
              {r.system.edges.map((e, i) => r.system.nodes[e.from].id !== selected ? null : (
                <label key={r.edgeIds[i]} className="sim-field">→ {r.system.nodes[e.to].label}
                  <input type="number" min={0} step={0.1} value={+e.mult.toFixed(3)} onChange={(ev) => update((s) => (s.flow[r.edgeIds[i]] = { ...s.flow[r.edgeIds[i]], mult: Math.max(0, +ev.target.value) }))} />
                </label>
              ))}
              <button className="link small" style={{ marginTop: 12 }} onClick={() => setSelected(null)}>Back to the Scenario</button>
            </>
          )}
        </aside>
      </div>

      <section className="sim-results">
        <div className="sim-tiles">
          <div className="sim-tile"><span>Traffic</span><b>{live ? rps(live.rate) : "–"} rps</b></div>
          <div className={`sim-tile ${live && live.err > 0.01 ? "bad" : ""}`}><span>Errors</span><b>{live && Number.isFinite(live.err) ? `${(live.err * 100).toFixed(2)}%` : "–"}</b></div>
          <div className="sim-tile"><span>p50</span><b>{live ? ms(live.p50) : "–"}</b></div>
          <div className={`sim-tile ${live && live.p95 > r.sloMs ? "bad" : ""}`}><span>p95</span><b>{live ? ms(live.p95) : "–"}</b></div>
          <div className={`sim-tile ${live && live.p99 > r.sloMs ? "bad" : ""}`}><span>p99 (target {r.sloMs} ms)</span><b>{live ? ms(live.p99) : "–"}</b></div>
          <div className="sim-tile" title={r.costExcludes ? `${r.costExcludes} components have no price yet` : ""}><span>Cost{r.costExcludes ? ` (excl. ${r.costExcludes})` : ""}</span><b>{live ? usd(live.cost) : "–"}</b></div>
        </div>
        <div className="sim-compare">
          <div className="row">
            <h4 style={{ margin: 0 }}>Baseline and test</h4>
            <span className="spacer" />
            <button disabled={!!running} onClick={() => run("baseline")}>{running === "baseline" ? "Running…" : sc.baseline ? "Re-run baseline" : "Run baseline"}</button>
            <button className="primary" disabled={!!running || !sc.baseline} onClick={() => run("test")} title={sc.baseline ? "" : "Run the baseline first"}>{running === "test" ? "Running…" : "Run test"}</button>
          </div>
          {!sc.baseline ? (
            <p className="small muted">Run the baseline to record how the diagram as drawn behaves at {rps(traffic)} rps and where it breaks. Then change things and run a test to compare.</p>
          ) : (
            <table className="sim-table small">
              <thead><tr><th /><th>Baseline</th><th>Test</th></tr></thead>
              <tbody>
                {([
                  ["Breaking point", (s: RunSummary) => (s.breakRps == null ? s.breakReason : `≈${rps(s.breakRps)} rps`)],
                  ["Breaks because", (s: RunSummary) => s.breakReason],
                  ["Busiest then", (s: RunSummary) => s.busiest.map((b) => `${b.label} ${pct(b.util)}`).join(", ")],
                  [`p50 / p95 / p99 at ${rps(sc.baseline.trafficRps)} rps`, (s: RunSummary) => `${ms(s.p50)} / ${ms(s.p95)} / ${ms(s.p99)}`],
                  ["Errors", (s: RunSummary) => `${s.errPct.toFixed(2)}%`],
                  ["Cost", (s: RunSummary) => `${usd(s.costPerHour)}${s.costExcludes ? ` (excl. ${s.costExcludes})` : ""}`],
                ] as [string, (s: RunSummary) => string][]).map(([k, f]) => (
                  <tr key={k}><th>{k}</th><td>{f(sc.baseline!)}</td><td>{sc.last ? f(sc.last) : <span className="muted">–</span>}</td></tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </section>
    </div>
  );
}
