// Draws a Production Diagram from its meaning (Graph) and geometry (ticket 016):
// fixed 200×72 cards with icon, label, Service name and Data Classification
// badge; Groups as styled rectangles behind their Nodes; elbow arrows styled
// by interaction. Everything Designer draws is regenerated from the Graph,
// so arrows are always re-created from skeletons when Nodes move (spike 015).
import { convertToExcalidrawElements, FONT_FAMILY } from "@excalidraw/excalidraw";
import type { BinaryFileData, BinaryFiles } from "@excalidraw/excalidraw/types";
import type { ExcalidrawElement, FileId } from "@excalidraw/excalidraw/element/types";
import type { Catalog, GEdge, GGroup, GNode, Graph, Meta } from "../types";
import { ICONS } from "../generated/icons";
import { placeLabels, type Route } from "./geometry";
import { edgeText, routeEdges } from "./routing";
import { CARD_H, CARD_W, GROUP_HEAD, GROUP_PAD, type Box } from "./constants";

export { CARD_H, CARD_W, GROUP_HEAD, GROUP_PAD };

export type { Box };
export type OverlayStatus = "add" | "remove" | "change" | "move";
export interface Overlay {
  status: Map<string, OverlayStatus>; // node / group / edge id
  ghosts: Map<string, Box>; // old position of moved Nodes
  focus?: Set<string>; // ids of the change being stepped through (others dimmed)
}

export const CATEGORY_COLOR: Record<string, string> = {
  compute: "#1971c2",
  data: "#2f9e44",
  network: "#6741d9",
  security: "#e03131",
  messaging: "#e8590c",
  observability: "#0c8599",
  external: "#495057",
};

const OVERLAY_COLOR: Record<OverlayStatus, string> = { add: "#2f9e44", remove: "#e03131", change: "#f08c00", move: "#1971c2" };
const OVERLAY_BADGE: Record<OverlayStatus, string> = { add: "+", remove: "−", change: "~", move: "↦" };
const CLASS_ORDER = ["PHI", "PII", "confidential", "public"];
const CLASS_COLOR: Record<string, string> = { PHI: "#c92a2a", PII: "#e8590c", confidential: "#5c940d", public: "#868e96" };

export const category = (kind?: string) => (kind ? kind.split(".")[0] : "external");

/** The most sensitive Data Classification, shown as the card badge. */
export const badgeClass = (classes?: string[]) => CLASS_ORDER.find((c) => classes?.includes(c));

function groupStyle(g: GGroup) {
  const base = { strokeColor: "#868e96", backgroundColor: "transparent", strokeStyle: "solid" as const, fillStyle: "solid" as const, strokeWidth: 1 };
  switch (g.kind) {
    case "region":
      return { ...base, strokeStyle: "dashed" as const, strokeWidth: 2 };
    case "account":
      return { ...base, strokeColor: "#343a40", strokeWidth: 2 };
    case "network":
      return { ...base, strokeColor: "#6741d9", backgroundColor: "#f8f9fa" };
    case "subnet":
      return { ...base, strokeColor: "#adb5bd", backgroundColor: g.tier === "public" ? "#ebfbee" : g.tier === "isolated" ? "#fff4e6" : "#e7f5ff" };
    case "zone":
      return { ...base, strokeStyle: "dotted" as const, strokeWidth: 2 };
    case "boundary":
      return { ...base, strokeColor: "#495057", strokeStyle: "dashed" as const };
  }
  return base;
}

const EDGE_STYLE: Record<string, "solid" | "dashed" | "dotted"> = { sync: "solid", async: "dashed", stream: "dotted" };

function ellipsis(s: string, max: number) {
  return s.length > max ? s.slice(0, max - 1) + "…" : s;
}

/** Icon files: one per (icon, colour), as SVG data URLs. */
export function iconFile(icon: string, color: string): BinaryFileData | null {
  const svg = ICONS[icon];
  if (!svg) return null;
  const colored = svg.replace(/currentColor/g, color);
  const id = `tabler-${icon}-${color.slice(1)}` as FileId;
  return { id, mimeType: "image/svg+xml", dataURL: `data:image/svg+xml;base64,${btoa(colored)}` as BinaryFileData["dataURL"], created: 0 };
}

export function serviceText(n: GNode, cat: Catalog): string {
  if (n.service) {
    for (const c of Object.values(cat.clouds)) {
      const s = c.services.find((x) => x.id === n.service);
      if (s) return s.name;
    }
    return n.service;
  }
  if (n.serviceName) return `${n.serviceName} (not in catalog)`;
  return cat.kinds.find((k) => k.id === n.kind)?.gloss ?? "";
}

export function groupKindName(g: GGroup, cat: Catalog, cloud: string): string {
  const names = cat.clouds[cloud]?.groupNames ?? {};
  switch (g.kind) {
    case "network":
      return names.network ?? "Virtual network";
    case "subnet":
      return `${g.tier ?? ""} subnet`.trim();
    case "account":
      return names.account ?? "Account";
    case "zone":
      return names.zone ?? "Availability zone";
    case undefined:
    case "":
      return "group";
  }
  return g.kind!;
}

type Skel = Record<string, any>;

/**
 * Builds the Excalidraw elements for a whole Production Diagram.
 * keep: elements the user drew that aren't Designer's (kept as they are).
 */
export function renderProduction(
  g: Graph,
  geo: Map<string, Box>,
  cat: Catalog,
  cloud: string,
  overlay?: Overlay,
  keep: readonly ExcalidrawElement[] = [],
): { elements: ExcalidrawElement[]; files: BinaryFiles } {
  const files: BinaryFiles = {};
  const addFile = (icon: string, color: string) => {
    const f = iconFile(icon, color);
    if (!f) return null;
    files[f.id] = f;
    return f.id;
  };
  const kindIcon = (kind?: string) => cat.kinds.find((k) => k.id === kind)?.icon ?? "box";
  const st = (id: string) => overlay?.status.get(id);
  const dim = (id: string) => (overlay?.focus && overlay.focus.size > 0 && !overlay.focus.has(id) ? 35 : undefined);
  const skel: Skel[] = [];
  const meta = new Map<string, Meta>(); // element id -> customData.designer

  // Groups, outermost first so they sit behind what they contain.
  const depth = (gr: GGroup): number => {
    let d = 0;
    let p = gr.parent;
    const seen = new Set<string>();
    while (p && !seen.has(p)) {
      seen.add(p);
      d++;
      p = g.groups.find((x) => x.id === p)?.parent;
    }
    return d;
  };
  for (const gr of [...g.groups].sort((a, b) => depth(a) - depth(b))) {
    const b = geo.get(gr.id);
    if (!b) continue;
    const s = groupStyle(gr);
    const o = st(gr.id);
    const color = o ? OVERLAY_COLOR[o] : s.strokeColor;
    skel.push({
      type: "rectangle", id: gr.id, x: b.x, y: b.y, width: b.w, height: b.h, roughness: 0, roundness: { type: 3 },
      strokeColor: color, backgroundColor: s.backgroundColor, fillStyle: s.fillStyle, strokeWidth: o ? 2 : s.strokeWidth,
      strokeStyle: o === "add" ? "dashed" : s.strokeStyle, opacity: o === "remove" ? 45 : dim(gr.id) ?? 100,
    });
    meta.set(gr.id, { role: "group", kind: gr.kind, tier: gr.tier, parent: gr.parent || undefined, label: gr.label });
    const gk = cat.groupKinds.find((k) => k.id === gr.kind);
    const iconId = addFile(gk?.icon ?? "border-outer", "#495057");
    if (iconId) {
      skel.push({ type: "image", id: `${gr.id}~i`, x: b.x + 10, y: b.y + 9, width: 18, height: 18, fileId: iconId, groupIds: [] });
      meta.set(`${gr.id}~i`, { role: "icon", node: gr.id });
    }
    skel.push({ type: "text", id: `${gr.id}~l`, x: b.x + 34, y: b.y + 4, text: gr.label || "(unnamed)", fontSize: 14, fontFamily: FONT_FAMILY.Nunito, strokeColor: "#343a40" });
    meta.set(`${gr.id}~l`, { role: "label", node: gr.id });
    skel.push({ type: "text", id: `${gr.id}~k`, x: b.x + 34, y: b.y + 21, text: groupKindName(gr, cat, cloud), fontSize: 10, fontFamily: FONT_FAMILY.Nunito, strokeColor: "#868e96" });
    meta.set(`${gr.id}~k`, { role: "decor", node: gr.id });
    if (o) badge(skel, meta, gr.id, b, o);
  }

  // Ghost outlines where moved Nodes used to be.
  for (const [id, b] of overlay?.ghosts ?? []) {
    skel.push({ type: "rectangle", id: `${id}~ghost`, x: b.x, y: b.y, width: b.w, height: b.h, roughness: 0, roundness: { type: 3 }, strokeColor: OVERLAY_COLOR.move, strokeStyle: "dashed", backgroundColor: "transparent", opacity: 50 });
    meta.set(`${id}~ghost`, { role: "decor", node: id });
  }

  for (const n of g.nodes) {
    const b = geo.get(n.id);
    if (!b) continue;
    const o = st(n.id);
    const tint = CATEGORY_COLOR[category(n.kind)] ?? "#495057";
    const stroke = o ? OVERLAY_COLOR[o] : tint;
    const op = o === "remove" ? 45 : dim(n.id) ?? 100;
    const group = `card-${n.id}`;
    skel.push({
      type: "rectangle", id: n.id, x: b.x, y: b.y, width: CARD_W, height: CARD_H, roughness: 0, roundness: { type: 3 },
      strokeColor: stroke, strokeWidth: 2, strokeStyle: o === "add" ? "dashed" : "solid",
      backgroundColor: o === "add" ? "#ebfbee" : o === "change" ? "#fff9db" : "#ffffff", fillStyle: "solid", groupIds: [group], opacity: op,
    });
    meta.set(n.id, { role: "node", kind: n.kind, service: n.service, serviceName: n.serviceName, classes: n.classes, attrs: n.attrs, parent: n.parent || undefined, label: n.label });
    const iconId = addFile(kindIcon(n.kind), tint);
    if (iconId) {
      skel.push({ type: "image", id: `${n.id}~i`, x: b.x + 12, y: b.y + 16, width: 40, height: 40, fileId: iconId, groupIds: [group], opacity: op });
      meta.set(`${n.id}~i`, { role: "icon", node: n.id });
    }
    const cls = badgeClass(n.classes);
    skel.push({ type: "text", id: `${n.id}~l`, x: b.x + 62, y: b.y + 13, text: ellipsis(n.label || "(unnamed)", cls ? 11 : 14), fontSize: 16, fontFamily: FONT_FAMILY.Nunito, strokeColor: "#1e1e1e", groupIds: [group], opacity: op });
    meta.set(`${n.id}~l`, { role: "label", node: n.id });
    skel.push({ type: "text", id: `${n.id}~s`, x: b.x + 62, y: b.y + 40, text: ellipsis(serviceText(n, cat), 22), fontSize: 11, fontFamily: FONT_FAMILY.Nunito, strokeColor: "#868e96", groupIds: [group], opacity: op });
    meta.set(`${n.id}~s`, { role: "decor", node: n.id });
    if (cls) {
      skel.push({ type: "text", id: `${n.id}~b`, x: b.x + CARD_W - 46, y: b.y + 6, text: cls === "confidential" ? "CONF" : cls, fontSize: 11, fontFamily: FONT_FAMILY.Nunito, strokeColor: CLASS_COLOR[cls], groupIds: [group], opacity: op });
      meta.set(`${n.id}~b`, { role: "decor", node: n.id });
    }
    if (o === "remove") {
      skel.push({ type: "line", id: `${n.id}~x`, x: b.x, y: b.y + CARD_H / 2, points: [[0, 0], [CARD_W, 0]], strokeColor: OVERLAY_COLOR.remove, strokeWidth: 2, roughness: 0, groupIds: [group] });
      meta.set(`${n.id}~x`, { role: "decor", node: n.id });
    }
    if (o) badge(skel, meta, n.id, b, o);
  }

  const bindings: ArrowBinding[] = [];
  const routed = routeEdges(g, geo);
  for (const e of g.edges) {
    const r = routed.get(e.id);
    if (!r) continue;
    const o = st(e.id);
    skel.push(arrowSkeleton(e, r.route, o, dim(e.id), r.text));
    bindings.push({ arrow: e.id, from: e.from, to: e.to, fs: r.route.fromPoint, ts: r.route.toPoint });
    meta.set(e.id, { role: "edge", style: e.style, protocol: e.protocol, label: e.label });
  }

  const els = convertToExcalidrawElements(skel as any, { regenerateIds: false }) as ExcalidrawElement[];
  for (const el of els) {
    const m = meta.get(el.id);
    if (m) (el as any).customData = { designer: clean(m) };
  }
  bindArrows(els, bindings);
  placeLabels(els, new Map([...routed].filter(([, r]) => r.label).map(([id, r]) => [id, { t: r.labelAt, pts: r.abs }])));
  return { elements: [...els, ...keep], files };
}

export type ArrowBinding = { arrow: string; from: string; to: string; fs: [number, number]; ts: [number, number] };

/**
 * Binds arrows to their ends ourselves: the skeleton's own binding doesn't
 * route arrows between existing shapes (ticket 016's documented fallback).
 */
export function bindArrows(els: readonly ExcalidrawElement[], bindings: ArrowBinding[]) {
  const byId = new Map(els.map((el) => [el.id, el as any]));
  for (const b of bindings) {
    const a = byId.get(b.arrow);
    if (!a) continue;
    a.startBinding = { elementId: b.from, fixedPoint: b.fs, mode: "orbit" };
    a.endBinding = { elementId: b.to, fixedPoint: b.ts, mode: "orbit" };
    for (const id of [b.from, b.to]) {
      const t = byId.get(id);
      if (t && !(t.boundElements ?? []).some((x: any) => x.id === b.arrow)) t.boundElements = [...(t.boundElements ?? []), { id: b.arrow, type: "arrow" }];
    }
  }
}

/** Skeleton geometry for a routed arrow. */
export function routeGeometry(r: Route) {
  const xs = r.points.map((p) => p[0]), ys = r.points.map((p) => p[1]);
  return { x: r.x, y: r.y, points: r.points, width: Math.max(...xs) - Math.min(...xs), height: Math.max(...ys) - Math.min(...ys) };
}

function arrowSkeleton(e: GEdge, r: Route, o: OverlayStatus | undefined, dim: number | undefined, text = edgeText(e)): Skel {
  return {
    type: "arrow", id: e.id, ...routeGeometry(r),
    elbowed: true, roughness: 0, endArrowhead: "arrow", startArrowhead: null,
    strokeColor: o ? OVERLAY_COLOR[o] : "#495057", strokeWidth: o ? 2 : 1.5,
    strokeStyle: EDGE_STYLE[e.style ?? "sync"] ?? "solid", opacity: o === "remove" ? 45 : dim ?? 100,
    ...(text ? { label: { text, fontSize: 11, fontFamily: FONT_FAMILY.Nunito } } : {}),
  };
}

function badge(skel: Skel[], meta: Map<string, Meta>, id: string, b: Box, o: OverlayStatus) {
  skel.push({ type: "text", id: `${id}~o`, x: b.x - 14, y: b.y - 18, text: OVERLAY_BADGE[o], fontSize: 20, fontFamily: FONT_FAMILY.Nunito, strokeColor: OVERLAY_COLOR[o] });
  meta.set(`${id}~o`, { role: "decor", node: id });
}

function clean(m: Meta): Meta {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(m)) {
    if (v === undefined || v === "" || (Array.isArray(v) && v.length === 0) || (v && typeof v === "object" && !Array.isArray(v) && Object.keys(v).length === 0)) continue;
    out[k] = v;
  }
  return out as Meta;
}

/** Reads Designer's meaning back from an element. */
export const metaOf = (el: { customData?: Record<string, any> }): Meta | undefined => el.customData?.designer;

/** The Node or Group an element belongs to (cards are several elements). */
export function ownerId(el: ExcalidrawElement): string | null {
  const m = metaOf(el);
  if (!m) return null;
  if (m.role === "node" || m.role === "group" || m.role === "edge") return el.id;
  return m.node ?? null;
}

/** Geometry of every Node and Group currently in a scene. */
export function geometryOf(elements: readonly ExcalidrawElement[]): Map<string, Box> {
  const geo = new Map<string, Box>();
  for (const el of elements) {
    if (el.isDeleted) continue;
    const m = metaOf(el);
    if (m?.role === "node" || m?.role === "group") geo.set(el.id, { x: el.x, y: el.y, w: el.width, h: el.height });
  }
  return geo;
}

/** Elements the user drew that Designer doesn't own (kept across re-renders). */
export function foreignElements(elements: readonly ExcalidrawElement[]): ExcalidrawElement[] {
  return elements.filter((el) => {
    if (el.isDeleted || metaOf(el)) return false;
    // Bound text of our arrows is regenerated with them.
    const c = (el as any).containerId as string | null;
    if (c && elements.some((x) => x.id === c && metaOf(x))) return false;
    return true;
  });
}
