import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CaptureUpdateAction, convertToExcalidrawElements, exportToBlob, exportToSvg, getCommonBounds, hashElementsVersion, restoreElements, serializeAsJSON } from "@excalidraw/excalidraw";
import type { AppState, BinaryFiles, ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
import type { ExcalidrawElement } from "@excalidraw/excalidraw/element/types";
import { api, retryable, CLIENT_ID, subscribe, type ServerEvent } from "./api";
import { ExportDoc, type DocFormat, type DocSignal } from "./ExportDoc";
import { perf } from "./perf";
import { Canvas } from "./Canvas";
import { Chat, toolText, type ChatItem } from "./panels/Chat";
import { Review, changeStatus, reviewStart, reviewSteps, type ReviewState } from "./panels/Review";
import { ConversionPanel, RationaleView } from "./panels/Rationale";
import { GapPanel } from "./panels/Gap";
import { Inspector, type NodePatch } from "./panels/Inspector";
import { Versions } from "./panels/Versions";
import { SettingsModal } from "./panels/Settings";
import { RecordViewer } from "./RecordViewer";
import { foreignElements, geometryOf, metaOf, ownerId, renderProduction, type Box, type Overlay } from "./production/render";
import { drawnParent, graphFromScene, hasOrphans } from "./production/graph";
import { elkLayout, growGroups, placeChanges, warmLayout } from "./production/layout";
import { arrowSegments } from "./production/geometry";
import { applyRough } from "./rough/apply";
import type { AssistantStatus, Change, GEdge, GGroup, Graph, Pending, ReportRef, State, Version, View } from "./types";

type Tab = "assistant" | "review" | "rationale" | "gap" | "inspect" | "versions";
type Scene = { elements: ExcalidrawElement[]; files: BinaryFiles };

// Properties that don't change how an element looks.
const VOLATILE = new Set(["version", "versionNonce", "updated", "seed", "index"]);
const signature = (e: ExcalidrawElement) => JSON.stringify(e, (k, v) => (VOLATILE.has(k) ? undefined : v));

/**
 * Keeps the previous object for every element that looks exactly the same as
 * before, so Excalidraw reuses its cached drawing instead of re-rasterizing it.
 */
function reuseUnchanged(prev: readonly ExcalidrawElement[], next: ExcalidrawElement[]): ExcalidrawElement[] {
  const old = new Map(prev.map((e) => [e.id, e]));
  return next.map((e) => {
    const o = old.get(e.id);
    return o && o.type === e.type && signature(o) === signature(e) ? o : e;
  });
}

type Bounds = [number, number, number, number];

/**
 * Brings a box into view and resolves when the view has arrived.
 *
 * Why it's hand-rolled: the step's new scene used to be swapped in first and
 * Excalidraw's own (time-based, ease-out) pan started on top of it. The first
 * pan frames then landed while the browser was still rasterizing the new
 * shapes, arrived late, and the ease-out curve turned them into big jumps.
 * Now the caller pans over the scene already on screen (all cached) and only
 * swaps the next step in after arrival. This pan eases in and out, so a slow
 * first frame can't become a leap, and keeps the zoom (zooming re-rasterizes
 * every shape). Already comfortably in view: nothing moves.
 */
function panTo(a: ExcalidrawImperativeAPI, [x1, y1, x2, y2]: Bounds, alive: () => boolean): Promise<void> {
  const st = a.getAppState();
  const z = st.zoom.value;
  const margin = 80; // keep clear of the toolbar and the canvas edges
  const view = { x: -st.scrollX + margin / z, y: -st.scrollY + margin / z, w: (st.width - margin * 2) / z, h: (st.height - margin * 2) / z };
  if (x1 >= view.x && y1 >= view.y && x2 <= view.x + view.w && y2 <= view.y + view.h) return Promise.resolve();
  // Centre the box in the canvas below the toolbar. A box bigger than the view
  // is centred too: the zoom is set once for the whole review and never jumps.
  const top = 60;
  const toX = st.width / 2 / z - (x1 + x2) / 2;
  const toY = (top + (st.height - top) / 2) / z - (y1 + y2) / 2;
  const fromX = st.scrollX;
  const fromY = st.scrollY;
  const duration = 380;
  const ease = (t: number) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2);
  return new Promise((resolve) => {
    let t0: number | null = null;
    const frame = (now: number) => {
      if (!alive() || a.isDestroyed) return resolve();
      t0 ??= now;
      const t = Math.min(1, (now - t0) / duration);
      const k = ease(t);
      a.updateScene({ appState: { scrollX: fromX + (toX - fromX) * k, scrollY: fromY + (toY - fromY) * k }, captureUpdate: CaptureUpdateAction.NEVER });
      if (t < 1) requestAnimationFrame(frame);
      else resolve();
    };
    requestAnimationFrame(frame);
  });
}

const HIGHLIGHT_GLOW = "__review-highlight-glow";
const HIGHLIGHT_OUTLINE = "__review-highlight";

const newId = () => Array.from(crypto.getRandomValues(new Uint8Array(15)), (b) => "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-"[b % 64]).join("");

/** Applies `needs` dependencies: a change whose prerequisite isn't accepted is dropped. */
function closure(changes: Change[], accepted: Set<string>) {
  const out = new Set(accepted);
  for (let changed = true; changed; ) {
    changed = false;
    for (const c of changes) if (out.has(c.id) && c.needs?.some((n) => !out.has(n))) { out.delete(c.id); changed = true; }
  }
  return out;
}

function roughLabels(elements: readonly ExcalidrawElement[]) {
  const m = new Map<string, string>();
  for (const el of elements) if (!el.isDeleted && el.type === "text" && (el as any).containerId) m.set((el as any).containerId, (el as any).originalText || (el as any).text);
  return m;
}

function parseScene(data: any): Scene {
  const elements = restoreElements(data?.elements ?? [], null, { repairBindings: true }) as ExcalidrawElement[];
  return { elements, files: (data?.files ?? {}) as BinaryFiles };
}

function loadFixes(dir: string): Record<string, number> {
  try {
    return JSON.parse(localStorage.getItem(`designer-fixes:${dir}`) ?? "{}");
  } catch {
    return {};
  }
}

export function Workspace({ initial, reload }: { initial: State; reload: () => Promise<State> }) {
  const [st, setSt] = useState(initial);
  const cat = st.catalog;
  const cloud = st.brief.answers.cloud || "generic";
  const versions = st.versions ?? [];
  const latest = versions.length ? versions[versions.length - 1].n : 0;
  const frozen = !!st.project.frozenAt;

  const apiRef = useRef<ExcalidrawImperativeAPI | null>(null);
  const [ready, setReady] = useState(false);
  const [view, setView] = useState<View>(initial.view);
  const [viewing, setViewing] = useState<number | null>(null);
  const [pending, setPending] = useState<Pending | null>(initial.pending);
  const [status, setStatus] = useState<AssistantStatus>(initial.assistant);
  const [tab, setTab] = useState<Tab>(initial.pending ? "review" : "assistant");
  const [items, setItems] = useState<ChatItem[]>([]);
  const [clicked, setClicked] = useState<string[]>([]);
  const [selection, setSelection] = useState<string[]>([]);
  const [graph, setGraph] = useState<Graph | null>(null);
  const [labels, setLabels] = useState<Map<string, string>>(new Map());
  const [review, setReview] = useState<ReviewState>(() => (initial.pending ? reviewStart(initial.pending) : { mode: "step", index: 0, decisions: {} }));
  const [laidOut, setLaidOut] = useState(false);
  const [layoutError, setLayoutError] = useState("");
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<{ text: string; error?: boolean } | null>(null);
  const [record, setRecord] = useState<string | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [connected, setConnected] = useState(true);
  // Canvas edits not yet saved: "pending" while the autosave waits, "failed" when it couldn't
  // reach the runtime (kept, retried every few seconds and as soon as the connection is back).
  const [unsaved, setUnsavedState] = useState<"" | "pending" | "failed">("");
  const unsavedRef = useRef<"" | "pending" | "failed">("");
  const setUnsaved = useCallback((v: "" | "pending" | "failed") => {
    if (unsavedRef.current === v) return;
    unsavedRef.current = v;
    setUnsavedState(v);
  }, []);
  const [fixed, setFixed] = useState<Record<string, number>>(() => loadFixes(initial.project.dir));
  const exportMenu = useRef<HTMLDetailsElement>(null);
  useEffect(() => {
    const close = (e: PointerEvent) => {
      const m = exportMenu.current;
      if (m?.open && !m.contains(e.target as Node)) m.open = false;
    };
    window.addEventListener("pointerdown", close);
    return () => window.removeEventListener("pointerdown", close);
  }, []);
  const [precheck, setPrecheck] = useState<{ code: string; message: string }[] | null>(null);

  const reviewing = !!pending && pending.kind === "proposal" && (pending.view === view);
  const previewing = !!pending && pending.kind === "conversion";
  const mode: "edit" | "frozen" | "version" | "review" | "preview" =
    previewing ? "preview" : reviewing ? "review" : viewing !== null && view === "production" && viewing !== latest ? "version" : view === "rough" && frozen ? "frozen" : "edit";
  const viewMode = mode !== "edit";

  const toastIt = useCallback((text: string, error = false) => {
    setToast({ text, error });
    window.setTimeout(() => setToast(null), error ? 6000 : 3000);
  }, []);
  const fail = useCallback((e: unknown) => toastIt((e as Error).message ?? String(e), true), [toastIt]);

  // ---------- scene loading

  const lastSaved = useRef<number>(0);
  const loading = useRef(0);
  const skipLoad = useRef(false);
  const reviewBase = useRef<Scene | null>(null);
  const previewScene = useRef<Scene | null>(null);
  const modeRef = useRef(mode);
  modeRef.current = mode;
  const viewRef = useRef(view);
  viewRef.current = view;
  const latestRef = useRef(latest);
  latestRef.current = latest;

  const show = useCallback((s: Scene, fit = true) => {
    const a = apiRef.current;
    if (!a) return;
    a.setElementRenderOverrides(null);
    a.addFiles(Object.values(s.files));
    a.updateScene({ elements: s.elements, captureUpdate: CaptureUpdateAction.NEVER });
    a.history.clear();
    lastSaved.current = hashElementsVersion(s.elements);
    setUnsaved("");
    if (fit && s.elements.length) a.setViewport({ target: s.elements.filter((e) => !e.isDeleted), fit: "scale-down", offsets: { ui: true } });
  }, []);

  const fetchScene = useCallback(async (v: View, n?: number): Promise<Scene> => {
    if (v === "rough") return parseScene(await api.rough());
    const num = n ?? latestRef.current;
    if (!num) return { elements: [], files: {} };
    return parseScene((await api.version(num)).scene);
  }, []);

  const refreshGraph = useCallback((els?: readonly ExcalidrawElement[]) => {
    const a = apiRef.current;
    const elements = els ?? a?.getSceneElements() ?? [];
    if (viewRef.current === "production" || modeRef.current === "preview") {
      const g = graphFromScene(elements);
      setGraph(g);
      setLabels((old) => {
        const m = new Map(old);
        for (const n of [...g.nodes, ...g.groups]) m.set(n.id, n.label || n.kind || "(unnamed)");
        return m;
      });
    } else {
      setGraph(null);
      setLabels((old) => new Map([...old, ...roughLabels(elements)]));
    }
  }, []);

  const label = useCallback((id?: string) => (id ? labels.get(id) ?? "(removed)" : "the top level"), [labels]);

  // ---------- Proposal review: overlay and apply

  // A review step's scene: built ahead of time where possible, then swapped in.
  type Built = { elements: ExcalidrawElement[]; files: BinaryFiles; target: Set<string>; changed: Set<string>; view: View; labels: [string, string][] };
  const builtCache = useRef(new Map<string, Promise<Built | null>>());
  const baseGen = useRef(0);
  const buildKey = (p: Pending, rs: ReviewState) =>
    `${baseGen.current}|${p.id}|${p.revision}|${rs.mode}|${rs.index}|${Object.keys(rs.decisions).filter((k) => rs.decisions[k] === "accepted").sort().join(",")}`;

  const buildOverlay = useCallback(async (p: Pending, rs: ReviewState): Promise<Built | null> => {
    const base = reviewBase.current;
    if (!base) return null;
    const changes = p.payload.changes;
    const steps = reviewSteps(changes);
    const step = rs.mode === "step" && rs.index < steps.length ? steps[rs.index] : undefined;
    // Step through: what you've accepted, drawn as part of the diagram, plus the step under review.
    // Full review: every change, all highlighted.
    const considered = rs.mode === "full"
      ? changes.filter((c) => rs.decisions[c.id] !== "skipped")
      : changes.filter((c) => rs.decisions[c.id] === "accepted" || step?.changes.includes(c));
    const styled = new Set(rs.mode === "full" ? considered.map((c) => c.id) : step ? step.changes.map((c) => c.id) : []);
    const inStep = (c: Change) => !!step?.changes.includes(c);
    const sim = await api.simulate(considered.map((c) => c.id));
    const labelList: [string, string][] = [...sim.base.nodes, ...sim.base.groups, ...sim.graph.nodes, ...sim.graph.groups].map((n) => [n.id, n.label || n.kind || "(unnamed)"]);
    let elements: ExcalidrawElement[];
    let files: BinaryFiles = base.files;
    const focusIds = new Set<string>();
    const changedIds = new Set<string>();
    for (const c of considered) [c.elementId, c.fromId, c.toId].forEach((x) => x && changedIds.add(x));
    if (p.view === "rough") {
      elements = applyRough(base.elements, sim.base, sim.graph, considered, styled).elements;
      for (const c of step?.changes ?? []) [c.elementId, c.fromId, c.toId].forEach((x) => x && focusIds.add(x));
    } else {
      const geo = geometryOf(base.elements);
      const place = new Set<string>();
      for (const c of considered) {
        if ((c.op === "add_node" || c.op === "add_group" || c.op === "move_node") && c.elementId) place.add(c.elementId);
        if ((c.op === "update_node" || c.op === "update_group") && c.parentId !== undefined && c.elementId) place.add(c.elementId);
      }
      const placed = placeChanges(sim.graph, geo, place, { avoid: arrowSegments(base.elements) });
      const union: Graph = { ...sim.graph, nodes: [...sim.graph.nodes], groups: [...sim.graph.groups], edges: [...sim.graph.edges] };
      const status = new Map<string, ReturnType<typeof changeStatus>>();
      const key = (e: GEdge) => `${e.from}>${e.to}`;
      for (const c of considered) {
        if (!styled.has(c.id)) continue;
        const s = changeStatus(c);
        if (c.op.endsWith("_edge")) {
          const k = `${c.fromId}>${c.toId}`;
          const e = (c.op === "remove_edge" ? sim.base.edges : sim.graph.edges).find((x) => key(x) === k);
          if (e) {
            status.set(e.id, s);
            if (inStep(c)) focusIds.add(e.id);
            if (c.op === "remove_edge") union.edges.push(e);
          }
        } else if (c.elementId) {
          status.set(c.elementId, s);
          if (inStep(c)) focusIds.add(c.elementId);
          if (c.op === "remove_node") {
            const n = sim.base.nodes.find((x) => x.id === c.elementId);
            if (n) union.nodes.push(n);
            for (const e of sim.base.edges) if ((e.from === c.elementId || e.to === c.elementId) && !union.edges.some((x) => x.id === e.id)) { union.edges.push(e); status.set(e.id, "remove"); }
          }
          if (c.op === "remove_group") {
            const g = sim.base.groups.find((x) => x.id === c.elementId);
            if (g) union.groups.push(g);
          }
        }
      }
      for (const [id, b] of geo) if (!placed.has(id)) placed.set(id, b);
      const ghosts = new Map<string, Box>();
      for (const c of considered) if (styled.has(c.id) && c.op === "move_node" && c.elementId && geo.get(c.elementId)) ghosts.set(c.elementId, geo.get(c.elementId)!);
      const ov: Overlay = { status, ghosts, focus: rs.mode === "step" ? focusIds : undefined };
      const r = renderProduction(union, placed, cat, cloud, ov, foreignElements(base.elements));
      elements = r.elements;
      files = { ...base.files, ...r.files };
    }
    const owns = (ids: Set<string>) => (e: ExcalidrawElement) => ids.has(e.id) || ids.has(metaOf(e)?.node ?? "") || ids.has((e as any).containerId ?? "");
    const target = new Set(elements.filter((e) => !e.isDeleted && owns(focusIds)(e)).map((e) => e.id));
    const changed = new Set(elements.filter((e) => !e.isDeleted && owns(changedIds)(e)).map((e) => e.id));
    // Highlight the change being reviewed: a frame around it (everything else is dimmed when shown).
    if (target.size) {
      const [x1, y1, x2, y2] = getCommonBounds(elements.filter((e) => target.has(e.id)));
      const pad = 14;
      const frame = { x: x1 - pad, y: y1 - pad, width: x2 - x1 + pad * 2, height: y2 - y1 + pad * 2, roughness: 0, roundness: { type: 3 }, locked: true };
      const [glow, outline] = convertToExcalidrawElements([
        { type: "rectangle", id: HIGHLIGHT_GLOW, ...frame, strokeColor: "transparent", backgroundColor: "#5b4bd6", fillStyle: "solid", opacity: 10, seed: 1 },
        { type: "rectangle", id: HIGHLIGHT_OUTLINE, ...frame, strokeColor: "#5b4bd6", strokeWidth: 3, backgroundColor: "transparent", seed: 2 },
      ] as any, { regenerateIds: false }) as ExcalidrawElement[];
      elements = [glow, ...elements, outline];
    }
    return { elements, files, target, changed, view: p.view, labels: labelList };
  }, [cat, cloud]);

  const builtFor = useCallback((p: Pending, rs: ReviewState) => {
    const k = buildKey(p, rs);
    let b = builtCache.current.get(k);
    if (!b) {
      b = perf.timeAsync("review.build", () => buildOverlay(p, rs), { step: rs.index, mode: rs.mode, changes: p.payload.changes.length }).catch(() => null);
      builtCache.current.set(k, b);
    }
    return b;
  }, [buildOverlay]);

  const showToken = useRef(0);
  const showOverlay = useCallback(async (b: Built, follow: boolean) => {
    const a = apiRef.current;
    if (!a) return;
    const token = ++showToken.current;
    // Move first, over what's already on screen; then swap the next step in.
    if (follow && b.target.size) {
      await perf.frames("review.pan", () => panTo(a, getCommonBounds(b.elements.filter((e) => b.target.has(e.id))) as Bounds, () => token === showToken.current));
      if (token !== showToken.current) return; // a newer step took over
    }
    setLabels((old) => {
      const m = new Map(old);
      for (const [id, l] of b.labels) m.set(id, l);
      return m;
    });
    a.addFiles(Object.values(b.files));
    const elements = perf.time("review.swap", () => {
      const els = reuseUnchanged(a.getSceneElements(), b.elements);
      a.updateScene({ elements: els, captureUpdate: CaptureUpdateAction.NEVER });
      return els;
    }, { elements: b.elements.length });
    // The Production renderer dims by itself; the Rough Draft uses render overrides (no document change).
    if (b.view === "rough" && b.target.size) {
      const keep = new Set([...b.target, HIGHLIGHT_GLOW, HIGHLIGHT_OUTLINE]);
      a.setElementRenderOverrides(new Map(elements.filter((e) => !keep.has(e.id)).map((e) => [e.id, { opacity: Math.min(e.opacity, 30) }])));
    } else a.setElementRenderOverrides(null);
  }, []);

  // Builds the next few steps while you read the current one (assuming you accept as you go),
  // one at a time in idle time and after the pan has settled, so the next Accept is just a swap.
  const prefetchTimer = useRef<number | undefined>(undefined);
  const prefetch = useCallback((p: Pending, rs: ReviewState, ahead = 3) => {
    window.clearTimeout(prefetchTimer.current);
    if (rs.mode !== "step") return;
    const steps = reviewSteps(p.payload.changes);
    const queue: ReviewState[] = [];
    let decisions = { ...rs.decisions };
    for (let k = 0; k < ahead && rs.index + k < steps.length; k++) {
      decisions = { ...decisions };
      for (const c of steps[rs.index + k].changes) decisions[c.id] = "accepted";
      const next = steps.findIndex((s, i) => i > rs.index + k && !s.changes.every((c) => decisions[c.id] === "accepted"));
      queue.push({ mode: "step", index: next === -1 ? steps.length : next, decisions });
    }
    const ric = (window as any).requestIdleCallback as undefined | ((fn: () => void, o: { timeout: number }) => number);
    const idle = (fn: () => void) => (ric ? ric(fn, { timeout: 2000 }) : setTimeout(fn, 50));
    const run = () => {
      const nextRs = queue.shift();
      if (!nextRs) return;
      idle(() => { void builtFor(p, nextRs).then(() => run()); });
    };
    prefetchTimer.current = window.setTimeout(run, 650);
  }, [builtFor]);

  const overlayFor = useCallback(async (p: Pending, rs: ReviewState, follow = true) => {
    const b = await builtFor(p, rs);
    if (!b || !reviewBase.current) return;
    await showOverlay(b, follow);
    prefetch(p, rs);
  }, [builtFor, showOverlay, prefetch]);

  /** At the start of a review: one zoom that fits everything the Proposal touches, so steps only pan. */
  const fitWholeProposal = useCallback(async (p: Pending) => {
    const a = apiRef.current;
    const all = await builtFor(p, { mode: "full", index: 0, decisions: {} });
    if (!a || !all || !all.changed.size) return;
    // Those elements aren't on the canvas yet, so fit their bounds.
    const [x1, y1, x2, y2] = getCommonBounds(all.elements.filter((e) => all.changed.has(e.id)));
    a.setViewport({ target: { x: x1, y: y1, width: x2 - x1, height: y2 - y1 }, fit: "scale-down", offsets: { ui: true }, animation: false });
  }, [builtFor]);

  const acceptProposal = useCallback(async (p: Pending, acceptedIds: string[]) => {
    const a = apiRef.current;
    const base = reviewBase.current;
    if (!a || !base) return;
    setBusy(true);
    try {
      const acc = closure(p.payload.changes, new Set(acceptedIds));
      const sim = await api.simulate([...acc]);
      let final: Scene;
      if (p.view === "rough") {
        final = { elements: applyRough(base.elements, sim.base, sim.graph, p.payload.changes.filter((c) => acc.has(c.id)), false).elements, files: base.files };
      } else {
        const geo = geometryOf(base.elements);
        const place = new Set<string>();
        for (const c of p.payload.changes) {
          if (!acc.has(c.id) || !c.elementId) continue;
          if (c.op === "add_node" || c.op === "add_group" || c.op === "move_node") place.add(c.elementId);
          if ((c.op === "update_node" || c.op === "update_group") && c.parentId !== undefined) place.add(c.elementId);
        }
        const placed = placeChanges(sim.graph, geo, place, { avoid: arrowSegments(base.elements) });
        const r = renderProduction(sim.graph, placed, cat, cloud, undefined, foreignElements(base.elements));
        final = { elements: r.elements, files: { ...base.files, ...r.files } };
      }
      const json = JSON.parse(serializeAsJSON(final.elements, a.getAppState(), final.files, "local"));
      const res = await api.decide({ id: p.id, revision: p.revision, action: "accept", accepted: [...acc], scene: json });
      // One Undo reverts the whole Proposal: restore the base, then apply the result as one step.
      skipLoad.current = true;
      a.setElementRenderOverrides(null);
      a.addFiles(Object.values(final.files));
      a.updateScene({ elements: base.elements, captureUpdate: CaptureUpdateAction.NEVER });
      a.updateScene({ elements: final.elements, captureUpdate: CaptureUpdateAction.IMMEDIATELY });
      lastSaved.current = hashElementsVersion(a.getSceneElements());
      setPending(null);
      reviewBase.current = null;
      if (p.payload.addresses?.length && res.version) {
        setFixed((f) => {
          const n = { ...f };
          for (const ad of p.payload.addresses!) n[ad.replace(/^gap:/, "")] = res.version!;
          try { localStorage.setItem(`designer-fixes:${st.project.dir}`, JSON.stringify(n)); } catch { /* per-viewer convenience only */ }
          return n;
        });
      }
      const fresh = await reload();
      setSt(fresh);
      setTab("assistant");
      toastIt(`Applied ${acc.size} change${acc.size === 1 ? "" : "s"}${res.version ? ` as v${res.version}` : ""}.`);
      refreshGraph();
    } catch (e) {
      fail(e);
    } finally {
      setBusy(false);
    }
  }, [cat, cloud, fail, refreshGraph, reload, st.project.dir, toastIt]);

  // ---------- Conversion Preview

  const buildPreview = useCallback(async () => {
    setLaidOut(false);
    setLayoutError("");
    const sim = await api.simulate(null);
    const rough = await fetchScene("rough");
    const hints = new Map<string, { x: number; y: number }>();
    for (const el of rough.elements) if (!el.isDeleted) hints.set(el.id, { x: el.x + el.width / 2, y: el.y + el.height / 2 });
    const geo = await elkLayout(sim.graph, hints);
    const r = renderProduction(sim.graph, geo, cat, cloud);
    previewScene.current = { elements: r.elements, files: r.files };
    show(previewScene.current);
    refreshGraph(r.elements);
    setLaidOut(true);
  }, [cat, cloud, fetchScene, refreshGraph, show]);

  // ---------- load whatever the current mode shows

  const pendingKey = pending ? `${pending.id}:${pending.revision}:${pending.awaitingRevision ? 1 : 0}` : "";
  useEffect(() => {
    if (!ready) return;
    if (skipLoad.current) {
      skipLoad.current = false;
      return;
    }
    const token = ++loading.current;
    (async () => {
      if (mode === "preview") {
        if (!pending?.awaitingRevision) await buildPreview().catch((e) => setLayoutError((e as Error).message ?? String(e)));
        else show(await fetchScene("rough"));
        return;
      }
      if (mode === "review" && pending) {
        const base = await fetchScene(pending.view, pending.view === "production" ? pending.baseVersion : undefined);
        if (token !== loading.current) return;
        reviewBase.current = base;
        baseGen.current++;
        builtCache.current.clear();
        show(base, false);
        refreshGraph(base.elements);
        await fitWholeProposal(pending);
        // While revising or asking, the canvas still shows what's been accepted.
        await overlayFor(pending, pending.awaitingRevision || pending.question ? { ...review, index: reviewSteps(pending.payload.changes).length } : review, false);
        return;
      }
      const s = await fetchScene(view, mode === "version" ? viewing! : undefined);
      if (token !== loading.current) return;
      show(s);
      refreshGraph(s.elements);
    })().catch(fail);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, mode, view, viewing, pendingKey, latest === 0]);

  // Re-draw the overlay as the review moves.
  useEffect(() => {
    if (mode === "review" && pending && !pending.awaitingRevision && reviewBase.current) overlayFor(pending, review).catch(fail);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [review.mode, review.index, JSON.stringify(review.decisions)]);

  // ---------- autosave and Production hand edits

  const saveTimer = useRef<number | undefined>(undefined);
  const dragStart = useRef<Map<string, Box> | null>(null);

  const retryTimer = useRef<number | undefined>(undefined);
  const saving = useRef(false);
  const saveNow = useCallback(async () => {
    const a = apiRef.current;
    if (!a || modeRef.current !== "edit" || saving.current) return;
    window.clearTimeout(retryTimer.current);
    const els = a.getSceneElements();
    const h = hashElementsVersion(els);
    if (h === lastSaved.current) return setUnsaved("");
    const json = serializeAsJSON(els, a.getAppState(), a.getFiles(), "local");
    const wasFailing = unsavedRef.current === "failed";
    saving.current = true;
    try {
      if (viewRef.current === "rough") await api.saveRough(json);
      else {
        const v = await api.saveVersion(JSON.parse(json), "edit", latestRef.current);
        if (v.n !== latestRef.current) setSt(await reload());
      }
      lastSaved.current = h;
      // Edits made while this save was on its way get their own (the autosave is already waiting).
      if (hashElementsVersion(a.getSceneElements()) === h) setUnsaved("");
      if (wasFailing) {
        perf.log("info", "saved after a failed attempt");
        toastIt("Saved.");
      }
    } catch (e) {
      if (retryable(e)) {
        // Keep the edit and try again: the runtime may be restarting.
        if (unsavedRef.current !== "failed") toastIt("Couldn't save; your changes are kept and will be saved when Designer is reachable again.", true);
        setUnsaved("failed");
        retryTimer.current = window.setTimeout(() => void saveNow(), 5000);
      } else {
        setUnsaved("failed");
        fail(e);
      }
    } finally {
      saving.current = false;
    }
  }, [fail, reload, setUnsaved, toastIt]);
  // Back online: save what's waiting right away.
  useEffect(() => {
    if (connected && unsavedRef.current === "failed") void saveNow();
  }, [connected, saveNow]);
  // Don't let a reload or close drop edits that aren't saved yet.
  useEffect(() => {
    const warn = (e: BeforeUnloadEvent) => {
      if (!unsavedRef.current) return;
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, []);

  const rerender = useCallback((mutate: (g: Graph) => void, place?: Set<string>) => {
    const a = apiRef.current;
    if (!a) return;
    const els = a.getSceneElements();
    const g = graphFromScene(els);
    mutate(g);
    const ids = new Set([...g.nodes.map((n) => n.id), ...g.groups.map((x) => x.id)]);
    for (const n of [...g.nodes, ...g.groups]) if (n.parent && !ids.has(n.parent)) n.parent = undefined;
    let geo = geometryOf(els);
    if (place?.size) geo = placeChanges(g, geo, place);
    const r = renderProduction(g, geo, cat, cloud, undefined, foreignElements(els));
    a.addFiles(Object.values(r.files));
    a.updateScene({ elements: r.elements, captureUpdate: CaptureUpdateAction.IMMEDIATELY });
    refreshGraph(r.elements);
  }, [cat, cloud, refreshGraph]);

  const onChange = useCallback((elements: readonly ExcalidrawElement[], appState: AppState) => {
    // Selection → Nodes for the Assistant's context and the inspector.
    const owners = new Set<string>();
    for (const id of Object.keys(appState.selectedElementIds ?? {})) {
      const el = elements.find((e) => e.id === id);
      if (!el) continue;
      const o = ownerId(el) ?? ((el as any).containerId as string | undefined) ?? (["rectangle", "ellipse", "diamond"].includes(el.type) ? el.id : null);
      if (o) owners.add(o);
    }
    const sel = [...owners];
    setSelection((prev) => (prev.join() === sel.join() ? prev : sel));
    if (modeRef.current !== "edit") return;
    if (hashElementsVersion(elements) === lastSaved.current) return;
    if (unsavedRef.current === "") setUnsaved("pending");
    if (unsavedRef.current === "failed") return; // the retry saves everything, these edits too
    window.clearTimeout(saveTimer.current);
    saveTimer.current = window.setTimeout(() => {
      if (viewRef.current === "production" && apiRef.current && hasOrphans(apiRef.current.getSceneElements())) rerender(() => {});
      refreshGraph();
      void saveNow();
    }, 700);
  }, [refreshGraph, rerender, saveNow, setUnsaved]);

  const onPointerDown = useCallback(() => {
    if (viewRef.current === "production" && modeRef.current === "edit" && apiRef.current) dragStart.current = geometryOf(apiRef.current.getSceneElements());
  }, []);
  // Production: a Node dropped into a Group gets that Group as its parent (ticket 006).
  const onPointerUp = useCallback(() => {
    const a = apiRef.current;
    const before = dragStart.current;
    dragStart.current = null;
    if (!a || !before || viewRef.current !== "production" || modeRef.current !== "edit") return;
    window.setTimeout(() => {
      const els = a.getSceneElements();
      const geo = geometryOf(els);
      const g = graphFromScene(els);
      const groups = g.groups.map((x) => ({ id: x.id, ...geo.get(x.id)! })).filter((x) => x.w);
      const moves: [string, string | undefined][] = [];
      for (const n of g.nodes) {
        const b0 = before.get(n.id);
        const b1 = geo.get(n.id);
        if (!b0 || !b1 || (b0.x === b1.x && b0.y === b1.y)) continue;
        const p = drawnParent(groups, b1);
        if (p !== (n.parent || undefined)) moves.push([n.id, p]);
      }
      if (moves.length) rerender((gg) => { for (const [id, p] of moves) { const n = gg.nodes.find((x) => x.id === id); if (n) n.parent = p; } });
    }, 0);
  }, [rerender]);

  // Frozen Rough Draft and previews: our own click handling (spike 015).
  const onViewClick = useCallback((x: number, y: number) => {
    const a = apiRef.current;
    if (!a) return;
    let best: ExcalidrawElement | null = null;
    for (const el of a.getSceneElements()) {
      if (el.isDeleted || !["rectangle", "ellipse", "diamond"].includes(el.type)) continue;
      const m = metaOf(el);
      if (m && m.role !== "node") continue;
      if (x >= el.x && x <= el.x + el.width && y >= el.y && y <= el.y + el.height && (!best || el.width * el.height < best.width * best.height)) best = el;
    }
    if (!best) return;
    const id = best.id;
    setSelection([id]);
    setClicked((c) => (c.includes(id) ? c.filter((v) => v !== id) : [...c, id]));
  }, []);

  // ---------- events from the runtime

  const handlers = useRef<(e: ServerEvent) => void>(() => {});
  handlers.current = (e: ServerEvent) => {
    const push = (it: Omit<ChatItem, "id">) => setItems((xs) => [...xs, { ...it, id: Date.now() + Math.random() }]);
    switch (e.type) {
      case "hello":
        reload().then((s) => { setSt(s); setStatus(s.assistant); setPending(s.pending); }).catch(() => {});
        break;
      case "assistant.status":
        setStatus(e.data);
        break;
      case "assistant.started":
        if (e.data.kind === "design") warmLayout();
        break;
      case "assistant.delta":
        setItems((xs) => {
          const last = xs[xs.length - 1];
          if (last?.kind === "assistant") return [...xs.slice(0, -1), { ...last, text: last.text + e.data.text }];
          return [...xs, { id: Date.now() + Math.random(), kind: "assistant", text: e.data.text }];
        });
        break;
      case "assistant.tool":
        push({ kind: "tool", text: toolText(e.data.name, e.data.input) });
        break;
      case "assistant.web":
        if (!e.data.allowed) push({ kind: "tool", text: `Blocked a web request outside the trusted sources (${e.data.tool})` });
        break;
      case "assistant.done":
        if (e.data.kind === "document" && e.data.failed) setDocSignal((s) => ({ seq: (s?.seq ?? 0) + 1, failed: e.data.failed }));
        if (e.data.failed) push({ kind: "error", text: e.data.failed, retry: e.data.kind && e.data.kind !== "chat" });
        else if (e.data.interrupted) push({ kind: "notice", text: "Stopped." });
        break;
      case "assistant.notice":
        push({ kind: "notice", text: e.data.text });
        break;
      case "proposal": {
        const p = e.data as Pending;
        setPending(p);
        setReview(reviewStart(p));
        if (p.kind === "proposal" && p.view !== viewRef.current) setView(p.view);
        setTab("review");
        break;
      }
      case "proposal.closed":
        setPending(null);
        reload().then(setSt).catch(() => {});
        if (e.data.conversion) {
          setView("production");
          setViewing(null);
        }
        break;
      case "document":
        setDocSignal((s) => ({ seq: (s?.seq ?? 0) + 1, version: e.data.version }));
        break;
      case "gapreport":
        reload().then(setSt).catch(() => {});
        setTab("gap");
        toastIt("Gap Report ready.");
        break;
      case "scene.saved":
        if (e.data.client !== CLIENT_ID) {
          reload().then((s) => {
            setSt(s);
            if (modeRef.current === "edit" && e.data.view === viewRef.current) fetchScene(viewRef.current).then((sc) => { show(sc, false); refreshGraph(sc.elements); });
          }).catch(() => {});
        }
        break;
      case "brief.saved":
        if (e.data.client !== CLIENT_ID) reload().then(setSt).catch(() => {});
        break;
    }
  };
  useEffect(() => subscribe((e) => handlers.current(e), setConnected), []);

  // ---------- actions

  const contextIds = useMemo(() => [...new Set([...clicked, ...selection])].filter((id) => labels.has(id)), [clicked, selection, labels]);
  const send = async (text: string) => {
    setItems((xs) => [...xs, { id: Date.now(), kind: "user", text }]);
    try {
      await api.chat(text, view, contextIds);
      setClicked([]);
    } catch (e) {
      setItems((xs) => [...xs, { id: Date.now() + 1, kind: "error", text: (e as Error).message }]);
    }
  };

  const startConversion = async () => {
    try {
      await saveNow();
      const issues = (await api.precheck()) ?? [];
      if (issues.length && !precheck) {
        setPrecheck(issues);
        return;
      }
      setPrecheck(null);
      await api.convert();
      setTab("assistant");
      setItems((xs) => [...xs, { id: Date.now(), kind: "notice", text: "Conversion started: the Assistant is designing the Production Diagram." }]);
    } catch (e) {
      fail(e);
    }
  };

  const decide = async (action: "reject" | "reply", reply?: string) => {
    if (!pending) return;
    setBusy(true);
    try {
      const accepted = pending.payload.changes.filter((c) => review.decisions[c.id] === "accepted").map((c) => c.id);
      await api.decide({ id: pending.id, revision: pending.revision, action, reply, ...(action === "reply" ? { accepted } : {}) });
      if (action === "reject") {
        setPending(null);
        reviewBase.current = null;
        toastIt("Rejected.");
      }
    } catch (e) {
      fail(e);
    } finally {
      setBusy(false);
    }
  };

  const acceptConversion = async () => {
    const a = apiRef.current;
    const s = previewScene.current;
    if (!pending || !a || !s) return;
    setBusy(true);
    try {
      const json = JSON.parse(serializeAsJSON(s.elements, a.getAppState(), s.files, "local"));
      await api.decide({ id: pending.id, revision: pending.revision, action: "accept", scene: json });
      setPending(null);
      setSt(await reload());
      setView("production");
      setViewing(null);
      setTab("rationale");
      toastIt("Production v1 created. Running the Gap Check…");
    } catch (e) {
      fail(e);
    } finally {
      setBusy(false);
    }
  };

  const tidy = async (scope?: string) => {
    const a = apiRef.current;
    if (!a) return;
    const els = a.getSceneElements();
    const g = graphFromScene(els);
    const geo = geometryOf(els);
    const hints = new Map([...geo].map(([id, b]) => [id, { x: b.x + b.w / 2, y: b.y + b.h / 2 }]));
    let origin = { x: Infinity, y: Infinity };
    if (scope && geo.get(scope)) origin = { x: geo.get(scope)!.x + 24, y: geo.get(scope)!.y + 44 };
    else for (const b of geo.values()) origin = { x: Math.min(origin.x, b.x), y: Math.min(origin.y, b.y) };
    if (!Number.isFinite(origin.x)) origin = { x: 0, y: 0 };
    const laid = await elkLayout(g, hints, scope ?? "", origin);
    const next = new Map(geo);
    for (const [id, b] of laid) next.set(id, b);
    if (scope) growGroups(g, next, geo);
    const r = renderProduction(g, next, cat, cloud, undefined, foreignElements(els));
    a.addFiles(Object.values(r.files));
    a.updateScene({ elements: r.elements, captureUpdate: CaptureUpdateAction.IMMEDIATELY });
    const json = serializeAsJSON(a.getSceneElements(), a.getAppState(), a.getFiles(), "local");
    try {
      await api.saveVersion(JSON.parse(json), "tidy", latestRef.current, scope ? "Tidy group" : "Tidy up");
      lastSaved.current = hashElementsVersion(a.getSceneElements());
      setSt(await reload());
    } catch (e) {
      fail(e);
    }
  };

  const addNode = (kind: string, service?: string) => {
    const id = newId();
    const sel = graph?.groups.find((x) => x.id === selection[0])?.id ?? graph?.nodes.find((n) => n.id === selection[0])?.parent;
    const gloss = cat.kinds.find((k) => k.id === kind)?.gloss ?? kind;
    rerender((g) => g.nodes.push({ id, label: gloss.charAt(0).toUpperCase() + gloss.slice(1), kind, service, parent: sel }), new Set([id]));
  };
  const updateNode = (id: string, p: NodePatch) => {
    if (view === "rough") {
      const a = apiRef.current;
      if (!a) return;
      const els = a.getSceneElements().map((el) => {
        if (el.id === id && "kind" in p) return { ...el, customData: { ...(el.customData ?? {}), designer: { ...(metaOf(el) ?? {}), kind: p.kind } }, version: el.version + 1 } as ExcalidrawElement;
        if ("label" in p && el.type === "text" && (el as any).containerId === id) return { ...el, text: p.label, originalText: p.label, version: el.version + 1 } as ExcalidrawElement;
        return el;
      });
      a.updateScene({ elements: els, captureUpdate: CaptureUpdateAction.IMMEDIATELY });
      return;
    }
    rerender((g) => { const n = g.nodes.find((x) => x.id === id); if (n) Object.assign(n, p); });
  };
  const updateGroup = (id: string, p: Partial<GGroup>) => rerender((g) => { const x = g.groups.find((y) => y.id === id); if (x) Object.assign(x, p); });
  const updateEdge = (id: string, p: Partial<GEdge>) => rerender((g) => { const x = g.edges.find((y) => y.id === id); if (x) Object.assign(x, p); });

  // arc42 document export: the images come from the version on the canvas.
  const [docExport, setDocExport] = useState<DocFormat | null>(null);
  const [docSignal, setDocSignal] = useState<DocSignal | null>(null);
  const docImages = () => perf.timeAsync("export.images", docImagesNow, { format: docExport });
  const docImagesNow = async () => {
    const a = apiRef.current;
    if (!a) return {};
    const elements = a.getSceneElements().filter((e) => !e.id.startsWith("__review"));
    const appState = { ...a.getAppState(), exportBackground: true, viewBackgroundColor: "#ffffff" };
    if (docExport === "pdf") {
      const svg = await exportToSvg({ elements, appState, files: a.getFiles(), exportPadding: 24 });
      return { svg: new XMLSerializer().serializeToString(svg) };
    }
    const blob = await exportToBlob({ elements, appState: { ...appState, exportScale: 2 }, files: a.getFiles(), mimeType: "image/png", exportPadding: 24 });
    const bytes = new Uint8Array(await blob.arrayBuffer());
    let bin = "";
    for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
    return { png: btoa(bin) };
  };
  const canExportDoc = view === "production" && !previewing && latest > 0 && !(pending && pending.view === "production");

  const exportImage = async (kind: "png" | "svg") => {
    const a = apiRef.current;
    if (!a) return;
    const elements = a.getSceneElements();
    const appState = { ...a.getAppState(), exportBackground: true, viewBackgroundColor: "#ffffff" };
    const name = `${st.project.name}-${view === "production" ? `v${viewing ?? latest}` : "rough-draft"}`;
    const blob = kind === "png"
      ? await exportToBlob({ elements, appState, files: a.getFiles(), mimeType: "image/png", exportPadding: 24 })
      : new Blob([new XMLSerializer().serializeToString(await exportToSvg({ elements, appState, files: a.getFiles(), exportPadding: 24 }))], { type: "image/svg+xml" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `${name}.${kind}`;
    link.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 2000);
  };

  const selectNode = (id: string) => {
    const a = apiRef.current;
    setSelection([id]);
    if (!a) return;
    const els = a.getSceneElements().filter((e) => e.id === id);
    if (els.length) a.setViewport({ target: els, fit: "none", offsets: { ui: true }, animation: true });
  };

  // ---------- render

  const [rat, setRat] = useState<Version["rationale"]>(undefined);
  useEffect(() => {
    const n = viewing ?? latest;
    if (!n) return setRat(undefined);
    api.version(n).then((v) => setRat(v.version.rationale), () => setRat(undefined));
  }, [viewing, latest]);

  const banner =
    mode === "frozen" ? "The Rough Draft is frozen. Click a shape to add it to your message." :
    mode === "review" ? "The canvas is locked while this Proposal waits for review." :
    mode === "preview" ? "Conversion Preview: read-only. Accept, reply or reject in the sidebar." :
    mode === "version" ? `Viewing v${viewing} (read-only)` : null;

  const tabs: [Tab, string][] = [
    ["assistant", "Assistant"],
    ...(pending ? ([["review", previewing ? "Conversion" : "Review"]] as [Tab, string][]) : []),
    ...(latest ? ([["rationale", "Rationale"], ["gap", "Gap Report"]] as [Tab, string][]) : []),
    ["inspect", "Inspect"],
    ...(latest ? ([["versions", "Versions"]] as [Tab, string][]) : []),
  ];
  const reports: ReportRef[] = st.reports ?? [];

  return (
    <div className="shell">
      <header className="topbar">
        <div className="brand">
          <span className="mark">Designer</span>
          <span className="sep">/</span>
          <span className="name" title={st.project.dir}>{st.project.name}</span>
        </div>
        <div className="tabs" role="tablist">
          <button className={view === "rough" && !previewing ? "on" : ""} onClick={() => { setView("rough"); setViewing(null); }} disabled={previewing}>Rough Draft{frozen ? " (frozen)" : ""}</button>
          <button className={view === "production" || previewing ? "on" : ""} disabled={!latest && !previewing} onClick={() => { setView("production"); setViewing(null); }}>
            {previewing ? "Conversion Preview" : latest ? `Production v${viewing ?? latest}` : "Production"}
          </button>
        </div>
        {!frozen && !previewing && (
          <button className="primary" disabled={status.busy || !!pending} onClick={startConversion} title="Turn the Rough Draft into a Production Diagram">Convert</button>
        )}
        <span className="spacer" />
        {!connected && <span className="pill bad">Disconnected — retrying</span>}
        {unsaved === "failed" && <span className="pill bad" title="Your edits are kept in this tab and saved as soon as Designer answers again">Unsaved changes — retrying</span>}
        <details className="menu" ref={exportMenu}>
          <summary><button className="ghost" tabIndex={-1} onClick={(e) => { e.preventDefault(); exportMenu.current!.open = !exportMenu.current!.open; }}>Export</button></summary>
          <div className="menu-list">
            <button className="ghost" onClick={() => { exportMenu.current!.open = false; void exportImage("png"); }}>PNG image</button>
            <button className="ghost" onClick={() => { exportMenu.current!.open = false; void exportImage("svg"); }}>SVG image</button>
            <div className="menu-sep" />
            <button className="ghost" disabled={!canExportDoc} title={canExportDoc ? "" : "Open a Production version (not during a review)"}
              onClick={() => { exportMenu.current!.open = false; setDocExport("docx"); }}>arc42 document · Word</button>
            <button className="ghost" disabled={!canExportDoc} title={canExportDoc ? "" : "Open a Production version (not during a review)"}
              onClick={() => { exportMenu.current!.open = false; setDocExport("pdf"); }}>arc42 document · PDF</button>
          </div>
        </details>
        <button className="ghost" onClick={() => setSettingsOpen(true)}>Settings</button>
      </header>

      <div className="main">
        <div className="canvas">
          <Canvas onApi={(a) => { apiRef.current = a; setReady(true); }} onChange={onChange} viewMode={viewMode} onViewClick={onViewClick} onPointerDown={onPointerDown} onPointerUp={onPointerUp} />
          {banner && <div className={`canvas-banner ${mode === "review" || mode === "preview" ? "locked" : ""}`}>{banner}</div>}
        </div>

        <aside className="sidebar">
          <nav className="sidebar-tabs">
            {tabs.map(([t, l]) => <button key={t} className={tab === t ? "on" : ""} onClick={() => setTab(t)}>{l}</button>)}
          </nav>
          {tab === "assistant" && (
            <Chat
              items={items}
              status={status}
              context={contextIds.map((id) => ({ id, label: labels.get(id) || "(unlabeled)" }))}
              onRemoveContext={(id) => { setClicked((c) => c.filter((x) => x !== id)); setSelection((s) => s.filter((x) => x !== id)); }}
              onSend={send}
              onStop={() => api.interrupt().catch(fail)}
              onRetry={() => api.retry().catch(fail)}
              onWarm={() => api.warm().catch((e) => setItems((xs) => [...xs, { id: Date.now(), kind: "error", text: e.message }]))}
              canSend={!pending?.awaitingRevision}
              placeholder={view === "rough" && !frozen ? "Ask about the sketch, or ask for changes…" : "Ask about the design, or ask for a change…"}
            />
          )}
          {tab === "review" && pending && pending.kind === "proposal" && (
            <Review pending={pending} state={review} setState={setReview} label={label} busy={busy}
              onFocus={() => {}}
              onAccept={(ids) => acceptProposal(pending, ids)}
              onReject={() => decide("reject")}
              onReply={(t) => decide("reply", t)}
              onRejectChange={(change, accepted, reason) => {
                // Nothing is under review while the Assistant reworks the rest: show only what's accepted.
                setReview({ mode: "step", index: reviewSteps(pending.payload.changes).length, decisions: Object.fromEntries(accepted.map((id) => [id, "accepted" as const])) });
                return api.decide({ id: pending.id, revision: pending.revision, action: "reject_change", change, accepted, reply: reason }).catch(fail);
              }}
              onAnswer={(text) => api.decide({ id: pending.id, revision: pending.revision, action: "clarify", reply: text }).catch(fail)} />
          )}
          {tab === "review" && pending && pending.kind === "conversion" && (
            <ConversionPanel pending={pending} label={label} selected={selection[0]} onSelect={selectNode} onOpenRecord={setRecord} busy={busy} laidOut={laidOut} layoutError={layoutError} onRetryLayout={() => buildPreview().catch((e) => setLayoutError((e as Error).message ?? String(e)))}
              onAccept={acceptConversion} onReject={() => decide("reject")} onReply={(t) => decide("reply", t)} />
          )}
          {tab === "rationale" && (
            <div className="panel"><RationaleView r={rat} label={label} selected={selection[0]} onSelect={selectNode} onOpenRecord={setRecord} /></div>
          )}
          {tab === "gap" && (
            <GapPanel reports={reports} latest={latest} label={label} onSelectNode={(id) => { setView("production"); selectNode(id); }} onOpenRecord={setRecord}
              fixed={fixed} busy={status.busy} pendingProposal={!!pending}
              onCheck={() => api.gapCheck(latest).then(() => setItems((xs) => [...xs, { id: Date.now(), kind: "notice", text: `Gap Check of v${latest} started.` }])).catch(fail)} />
          )}
          {tab === "inspect" && (
            <Inspector g={view === "production" ? graph : roughInspectGraph(apiRef.current, labels)} cat={cat} cloud={cloud} selected={selection[0] ?? null} view={view}
              locked={mode !== "edit"} onNode={updateNode} onGroup={updateGroup} onEdge={updateEdge} onAddNode={addNode} onTidy={view === "production" ? tidy : undefined} />
          )}
          {tab === "versions" && (
            <Versions versions={versions} viewing={viewing} onView={(n) => { setView("production"); setViewing(n); }} onLatest={() => setViewing(null)}
              onRestore={async (n) => {
                try {
                  const v = await api.version(n);
                  await api.saveVersion(v.scene, "restore", latest, `Restored v${n}`);
                  setViewing(null);
                  setSt(await reload());
                } catch (e) {
                  fail(e);
                }
              }} />
          )}
        </aside>
      </div>

      {precheck && (
        <div className="modal-back" onClick={() => setPrecheck(null)}>
          <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-label="Before Conversion">
            <h2>Before Conversion</h2>
            <p className="lede" style={{ marginTop: 6 }}>A quick local check found a few things worth fixing first. No AI is involved.</p>
            <ul>{precheck.map((p, i) => <li key={i}>{p.message}</li>)}</ul>
            <div className="row">
              <button onClick={() => setPrecheck(null)}>Fix them first</button>
              <span className="spacer" />
              <button className="primary" onClick={startConversion} disabled={precheck.some((p) => p.code === "empty")}>Convert anyway</button>
            </div>
          </div>
        </div>
      )}
      {docExport && (
        <ExportDoc version={viewing ?? latest} format={docExport} signal={docSignal} images={docImages}
          onClose={() => setDocExport(null)} onDone={(text) => toastIt(text)} />
      )}
      {record && <RecordViewer id={record} onClose={() => setRecord(null)} onOpen={setRecord} />}
      {settingsOpen && (
        <SettingsModal settings={st.settings} schema={st.briefSchema} brief={st.brief} name={st.project.name} trusted={st.trusted} kbVersion={st.kbVersion} cloudLocked={latest > 0}
          onClose={() => setSettingsOpen(false)} onSaved={() => reload().then(setSt)} />
      )}
      {toast && <div className={`toast ${toast.error ? "error" : ""}`} role="status">{toast.text}</div>}
    </div>
  );
}

/** The Rough Draft has no stored graph; the inspector edits labels and optional Kind tags. */
function roughInspectGraph(a: ExcalidrawImperativeAPI | null, labels: Map<string, string>): Graph | null {
  if (!a) return null;
  const nodes = a.getSceneElements()
    .filter((e) => !e.isDeleted && ["rectangle", "ellipse", "diamond"].includes(e.type))
    .map((e) => ({ id: e.id, label: labels.get(e.id) ?? "", kind: metaOf(e)?.kind }));
  return { production: false, nodes, groups: [], edges: [], notes: null, issues: null };
}
