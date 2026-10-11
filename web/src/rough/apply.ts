// Applies a Proposal's changes to the Rough Draft (a hand-drawn scene).
// Shapes that change (added, moved, grown, relabelled) are re-created from
// skeletons with the same IDs and styles, together with every arrow touching
// them, so Excalidraw re-binds and re-routes the arrows (spike 015).
import { convertToExcalidrawElements } from "@excalidraw/excalidraw";
import type { ExcalidrawElement } from "@excalidraw/excalidraw/element/types";
import type { Change, Graph } from "../types";
import { bindArrows, routeGeometry, type ArrowBinding, type Box, type OverlayStatus } from "../production/render";
import { arrowSegments, bestRoute, estimateLabel, labelBox, placeLabels, type Box as GBox, type Pt } from "../production/geometry";
import { placeChanges } from "../production/layout";
import { perf } from "../perf";

const PREVIEW: Record<OverlayStatus, { strokeColor: string; strokeStyle?: string; backgroundColor?: string; opacity?: number }> = {
  add: { strokeColor: "#2f9e44", strokeStyle: "dashed", backgroundColor: "#ebfbee" },
  remove: { strokeColor: "#e03131", opacity: 40 },
  change: { strokeColor: "#f08c00", backgroundColor: "#fff9db" },
  move: { strokeColor: "#1971c2" },
};

const SHAPES = new Set(["rectangle", "ellipse", "diamond", "image"]);

function boxOf(el: ExcalidrawElement): Box {
  return { x: el.x, y: el.y, w: el.width, h: el.height };
}

/** Rough Draft geometry of every Node and Group shape. */
export function roughGeometry(elements: readonly ExcalidrawElement[], g: Graph): Map<string, Box> {
  const ids = new Set([...g.nodes.map((n) => n.id), ...g.groups.map((x) => x.id)]);
  const geo = new Map<string, Box>();
  for (const el of elements) if (!el.isDeleted && ids.has(el.id)) geo.set(el.id, boxOf(el));
  return geo;
}

export interface RoughResult {
  elements: ExcalidrawElement[];
  status: Map<string, OverlayStatus>; // for the change list / focus
}

/**
 * base: the Rough Draft graph; result: after the changes (from the runtime's
 * simulation); changes: the ones to draw. styled: which changes are drawn as
 * an overlay (green/red/orange); the others are applied as normal parts of
 * the sketch. Pass true to style all of them, false to apply all of them.
 */
export function applyRough(elements: readonly ExcalidrawElement[], base: Graph, result: Graph, changes: Change[], styled: boolean | Set<string>): RoughResult {
  return perf.time("rough.apply", () => apply(elements, base, result, changes, styled), { elements: elements.length, changes: changes.length });
}

function apply(elements: readonly ExcalidrawElement[], base: Graph, result: Graph, changes: Change[], styled: boolean | Set<string>): RoughResult {
  const isStyled = (ch: Change) => (styled === true ? true : styled === false ? false : styled.has(ch.id));
  const live = elements.filter((e) => !e.isDeleted);
  const byId = new Map(live.map((e) => [e.id, e]));
  const textOf = (id: string) => live.find((t) => t.type === "text" && (t as any).containerId === id);
  const status = new Map<string, OverlayStatus>();

  // What each change does to which element. status holds overlay looks (styled changes only).
  const removed = new Set<string>(); // all removed shapes
  const removedShown = new Set<string>(); // removed, but drawn in red for review
  const place = new Set<string>();
  const relabel = new Map<string, string>();
  const restyle = new Map<string, OverlayStatus>();
  const edgeKey = (a?: string, b?: string) => `${a}>${b}`;
  const removedEdges = new Set<string>();
  const removedEdgesShown = new Set<string>();
  const changedEdges = new Map<string, string>(); // key -> new label
  for (const ch of changes) {
    const show = isStyled(ch);
    const mark = (key: string, st: OverlayStatus) => { if (show) status.set(key, st); };
    switch (ch.op) {
      case "add_node":
      case "add_group":
        place.add(ch.elementId!);
        mark(ch.elementId!, "add");
        break;
      case "move_node":
        place.add(ch.elementId!);
        mark(ch.elementId!, "move");
        if (show) restyle.set(ch.elementId!, "move");
        break;
      case "update_node":
      case "update_group": {
        const label = ch.set?.label;
        if (typeof label === "string") relabel.set(ch.elementId!, label);
        if (ch.parentId !== undefined && ch.parentId !== null) place.add(ch.elementId!);
        mark(ch.elementId!, "change");
        restyle.set(ch.elementId!, "change"); // re-created either way, to carry the new meaning
        break;
      }
      case "remove_node":
      case "remove_group":
        removed.add(ch.elementId!);
        if (show) removedShown.add(ch.elementId!);
        mark(ch.elementId!, "remove");
        break;
      case "add_edge":
        mark(edgeKey(ch.fromId, ch.toId), "add");
        break;
      case "update_edge":
        changedEdges.set(edgeKey(ch.fromId, ch.toId), String(ch.set?.label ?? ""));
        mark(edgeKey(ch.fromId, ch.toId), "change");
        break;
      case "remove_edge":
        removedEdges.add(edgeKey(ch.fromId, ch.toId));
        if (show) removedEdgesShown.add(edgeKey(ch.fromId, ch.toId));
        mark(edgeKey(ch.fromId, ch.toId), "remove");
        break;
    }
  }

  // Geometry: existing shapes keep theirs; new and moved ones are placed.
  const geo = roughGeometry(live, base);
  // New shapes take the sketch's typical size, with room for hand-written arrow labels.
  const sizes = [...geo.entries()].filter(([id]) => base.nodes.some((n) => n.id === id)).map(([, b]) => b);
  const median = (xs: number[], d: number) => (xs.length ? [...xs].sort((x, y) => x - y)[Math.floor(xs.length / 2)] : d);
  const size = { w: median(sizes.map((b) => b.w), 200), h: median(sizes.map((b) => b.h), 90) };
  // Hand-written labels are about 11px per character and wrap at roughly 240px.
  const labelWidth = (from: string, to: string) => {
    const e = result.edges.find((x) => x.from === from && x.to === to);
    return e?.label ? Math.min(e.label.length * 11, 240) : 0;
  };
  const placed = placeChanges(result, geo, place, { size, gap: 130, avoid: arrowSegments(live), labelWidth });

  // Which shapes must be re-created.
  const dirty = new Set<string>([...place, ...relabel.keys(), ...restyle.keys()]);
  for (const [id, b] of placed) {
    const old = geo.get(id);
    if (old && (old.x !== b.x || old.y !== b.y || old.w !== b.w || old.h !== b.h)) dirty.add(id);
  }
  for (const id of removedShown) dirty.add(id);

  // New arrows bind to their ends, which must be in the batch.
  for (const e of result.edges) {
    if (!base.edges.some((b) => b.from === e.from && b.to === e.to)) {
      if (byId.has(e.from)) dirty.add(e.from);
      if (byId.has(e.to)) dirty.add(e.to);
    }
  }
  // Arrows: existing ones touching dirty shapes are re-created too (a re-created
  // shape only lists the arrows created with it in its boundElements).
  const arrows = live.filter((e) => e.type === "arrow");
  const arrowEnds = (a: ExcalidrawElement) => [(a as any).startBinding?.elementId as string | undefined, (a as any).endBinding?.elementId as string | undefined];
  const dirtyArrows = new Set<string>();
  // Until nothing changes: an arrow touching a dirty shape is re-created, which
  // makes both its ends dirty, which may pull in more arrows.
  for (let grew = true; grew; ) {
    grew = false;
    for (const a of arrows) {
      if (dirtyArrows.has(a.id)) continue;
      const [s, t] = arrowEnds(a);
      const key = edgeKey(s, t);
      const touches = (s && dirty.has(s)) || (t && dirty.has(t)) || changedEdges.has(key) || removedEdges.has(key) || (s && removed.has(s)) || (t && removed.has(t));
      if (!touches) continue;
      dirtyArrows.add(a.id);
      grew = true;
      for (const end of [s, t]) if (end && byId.has(end) && !removed.has(end)) dirty.add(end);
    }
  }

  const resultNode = (id: string) => result.nodes.find((n) => n.id === id) ?? result.groups.find((x) => x.id === id);
  const baseNode = (id: string) => base.nodes.find((n) => n.id === id) ?? base.groups.find((x) => x.id === id);
  const isGroup = (id: string) => result.groups.some((x) => x.id === id) || base.groups.some((x) => x.id === id);

  const skel: Record<string, any>[] = [];
  const keepVersion = new Map<string, number>();
  for (const id of dirty) {
    if (removed.has(id) && !removedShown.has(id)) continue;
    const old = byId.get(id);
    const b = placed.get(id) ?? (old ? boxOf(old) : undefined);
    if (!b) continue;
    const label = relabel.get(id) ?? resultNode(id)?.label ?? baseNode(id)?.label ?? "";
    const st = status.get(id);
    const look = st ? PREVIEW[st] : undefined;
    const type = old && SHAPES.has(old.type) && old.type !== "image" ? old.type : "rectangle";
    skel.push({
      type, id, x: b.x, y: b.y, width: b.w, height: b.h,
      strokeColor: look?.strokeColor ?? old?.strokeColor ?? "#1e1e1e",
      backgroundColor: look?.backgroundColor ?? old?.backgroundColor ?? "transparent",
      fillStyle: old?.fillStyle ?? "solid", strokeWidth: old?.strokeWidth ?? 2,
      strokeStyle: (look?.strokeStyle as any) ?? old?.strokeStyle ?? "solid", roughness: old?.roughness ?? 1,
      roundness: old?.roundness ?? { type: 3 }, opacity: look?.opacity ?? old?.opacity ?? 100,
      seed: old?.seed ?? seedOf(id),
      customData: roughMeta(old?.customData, resultNode(id) as any),
      groupIds: old?.groupIds ?? [],
      ...(label ? { label: { text: label, ...(isGroup(id) ? { verticalAlign: "top" } : {}) } } : {}),
    });
    if (old) keepVersion.set(id, old.version);
  }

  const bindings: ArrowBinding[] = [];
  const boxFor = (id: string) => placed.get(id) ?? (byId.get(id) ? boxOf(byId.get(id)!) : undefined);
  const moved = (id: string) => {
    const was = geo.get(id), now = placed.get(id);
    return !was || !now || was.x !== now.x || was.y !== now.y || was.w !== now.w || was.h !== now.h;
  };
  // Labels already on the canvas (arrows we keep as they are), then each new route's label as it is placed.
  const labelBoxes: GBox[] = [];
  const labelAt = new Map<string, { t: number; pts: Pt[] }>();
  const lineSegs: [Pt, Pt][] = arrowSegments(live.filter((el) => !dirtyArrows.has(el.id)));
  const addLine = (pts: Pt[]) => { for (let i = 1; i < pts.length; i++) lineSegs.push([pts[i - 1], pts[i]]); };
  for (const el of live) {
    if (el.type !== "arrow" || dirtyArrows.has(el.id)) continue;
    const t = textOf(el.id) as any;
    if (t) labelBoxes.push({ x: t.x, y: t.y, w: t.width, h: t.height });
  }
  const shapeBoxes = () => [...placed.entries()].filter(([id]) => !isGroup(id) && !(removed.has(id) && !removedShown.has(id)));
  const routed = (arrowId: string, from: string, to: string, text = "") => {
    const a = boxFor(from), b = boxFor(to);
    if (!a || !b) return {};
    const old = byId.get(arrowId) as any;
    if (old && !moved(from) && !moved(to) && old.startBinding?.elementId === from && old.endBinding?.elementId === to) {
      // Neither end moved: keep the arrow's own path.
      bindings.push({ arrow: arrowId, from, to, fs: old.startBinding.fixedPoint ?? [1, 0.5], ts: old.endBinding.fixedPoint ?? [0, 0.5] });
      const pts = (old.points as number[][]).map((p) => [old.x + p[0], old.y + p[1]] as Pt);
      const oldText = textOf(arrowId) as any;
      const t = typeof oldText?.labelPosition === "number" ? oldText.labelPosition : 0.5;
      if (text) labelBoxes.push(labelBox(pts, estimateLabel(text), t));
      labelAt.set(arrowId, { t, pts });
      addLine(pts);
      return { x: old.x, y: old.y, points: old.points, width: old.width, height: old.height };
    }
    const size = text ? estimateLabel(text) : undefined;
    const r = bestRoute(a, b, shapeBoxes().filter(([id]) => id !== from && id !== to).map(([, bx]) => bx), true, size, labelBoxes, lineSegs);
    const abs = r.points.map(([x, y]) => [x + r.x, y + r.y] as Pt);
    if (size) labelBoxes.push(labelBox(abs, size, r.labelAt));
    labelAt.set(arrowId, { t: r.labelAt, pts: abs });
    addLine(abs);
    bindings.push({ arrow: arrowId, from, to, fs: r.fromPoint, ts: r.toPoint });
    return routeGeometry(r);
  };
  const arrowStyle = (st?: OverlayStatus) => (st ? { strokeColor: PREVIEW[st].strokeColor, opacity: PREVIEW[st].opacity ?? 100, strokeStyle: PREVIEW[st].strokeStyle ?? "solid" } : {});
  for (const id of dirtyArrows) {
    const a = byId.get(id)!;
    const [s, t] = arrowEnds(a);
    const key = edgeKey(s, t);
    const gone = removedEdges.has(key) || (s !== undefined && removed.has(s)) || (t !== undefined && removed.has(t));
    const goneShown = removedEdgesShown.has(key) || (s !== undefined && removedShown.has(s)) || (t !== undefined && removedShown.has(t));
    if (gone && !goneShown) continue;
    if (!s || !t || !dirty.has(s) || !dirty.has(t)) continue;
    const lbl = changedEdges.get(key) ?? (textOf(id) as any)?.originalText ?? (textOf(id) as any)?.text ?? "";
    skel.push({
      type: "arrow", id, ...routed(id, s, t, lbl), seed: a.seed, roughness: a.roughness, strokeColor: a.strokeColor,
      strokeWidth: a.strokeWidth, strokeStyle: a.strokeStyle, customData: a.customData,
      ...arrowStyle(gone ? "remove" : status.get(key)),
      ...(lbl ? { label: { text: lbl } } : {}),
    });
    keepVersion.set(id, a.version);
  }
  for (const e of result.edges) {
    if (base.edges.some((b) => b.from === e.from && b.to === e.to)) continue;
    const id = `${e.from.slice(0, 10)}-${e.to.slice(0, 10)}`;
    skel.push({
      type: "arrow", id, ...routed(id, e.from, e.to, e.label ?? ""), seed: seedOf(id),
      roughness: 1, strokeColor: "#1e1e1e", ...arrowStyle(status.get(edgeKey(e.from, e.to))), ...(e.label ? { label: { text: e.label } } : {}),
    });
  }

  const created = convertToExcalidrawElements(skel as any, { regenerateIds: false }) as ExcalidrawElement[];
  bindArrows(created, bindings);
  placeLabels(created, labelAt);
  for (const el of created) {
    const v = keepVersion.get(el.id);
    if (v !== undefined) Object.assign(el as any, { version: v + 1, versionNonce: Math.floor(Math.random() * 2 ** 31) });
  }
  const recreated = new Set(created.map((e) => e.id));
  // Drop old versions of re-created shapes and arrows, their bound texts, and (on apply) removed things.
  const replacedContainers = new Set([...dirty, ...dirtyArrows]);
  const out: ExcalidrawElement[] = [];
  for (const el of live) {
    if (recreated.has(el.id)) continue;
    const container = (el as any).containerId as string | undefined;
    if (container && replacedContainers.has(container)) continue;
    if ((removed.has(el.id) && !removedShown.has(el.id)) || (container && removed.has(container) && !removedShown.has(container))) continue;
    if (dirtyArrows.has(el.id)) continue; // re-created above, or a removed edge
    out.push(el);
  }
  // Groups go behind what they contain.
  const groupsFirst = created.filter((e) => isGroup(e.id) || ((e as any).containerId && isGroup((e as any).containerId)));
  const rest = created.filter((e) => !groupsFirst.includes(e));
  return { elements: [...groupsFirst, ...out, ...rest], status };
}

/** Keeps what the result says about a Node (Kind tag, classes, attributes) on its shape. */
function roughMeta(old: Record<string, any> | undefined, n: { kind?: string; classes?: string[]; attrs?: Record<string, string> } | undefined) {
  if (!n) return old;
  const designer = { ...(old?.designer ?? {}) };
  for (const k of ["kind", "classes", "attrs"] as const) {
    const v = n[k];
    if (v === undefined || (Array.isArray(v) && !v.length) || (v && typeof v === "object" && !Array.isArray(v) && !Object.keys(v).length)) delete designer[k];
    else designer[k] = v;
  }
  return Object.keys(designer).length ? { ...(old ?? {}), designer } : old;
}

/** A stable seed for a shape's hand-drawn strokes, so it looks the same every time it is drawn. */
function seedOf(id: string): number {
  let h = 2166136261;
  for (let i = 0; i < id.length; i++) h = Math.imul(h ^ id.charCodeAt(i), 16777619);
  return (h >>> 0) % 2147483647 || 1;
}
