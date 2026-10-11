// Layout (ticket 016).
// - ELK (layered, left to right) in a lazily loaded Web Worker for whole
//   diagrams: the Conversion Preview and Tidy up. Positions the user drew
//   are ordering hints (semi-interactive crossing minimisation).
// - Local placement for Proposal changes: no ELK, nothing outside the
//   touched Groups moves.
import type { Graph, GNode, GGroup } from "../types";
import { CARD_H, CARD_W, GROUP_HEAD, GROUP_PAD, type Box } from "./constants";
import { boxesOverlap, center, flowDirection, inflate, labelSize, segmentHitsBox, wrapLabel, type Pt } from "./geometry";
import { perf } from "../perf";

const GAP_X = 60;
const GAP_Y = 36;

// ---------- ELK

type ElkNode = {
  id: string; x?: number; y?: number; width?: number; height?: number;
  children?: ElkNode[]; edges?: { id: string; sources: string[]; targets: string[]; labels?: { text: string; width: number; height: number }[] }[];
  layoutOptions?: Record<string, string>;
};
let elkPromise: Promise<{ layout: (g: ElkNode) => Promise<ElkNode> }> | null = null;

/** Tests and the layout harness run ELK in-process instead of in a Web Worker. */
export function useElk(instance: { layout: (g: ElkNode) => Promise<ElkNode> }) {
  elkPromise = Promise.resolve(instance);
}

function elk() {
  elkPromise ??= import("elkjs/lib/elk-api.js").then((m: any) => {
    const ELK = m.default ?? m;
    return new ELK({ workerUrl: "/elk-worker.min.js" });
  });
  return elkPromise;
}

/** Loads ELK and runs a tiny layout so the first real one is fast. Called when Design starts. */
export function warmLayout() {
  void elk().then((e) => e.layout({ id: "warm", children: [{ id: "a", width: 10, height: 10 }] }).catch(() => {}));
}

const ROOT_OPTIONS: Record<string, string> = {
  "elk.algorithm": "layered",
  "elk.direction": "RIGHT",
  "elk.hierarchyHandling": "INCLUDE_CHILDREN",
  "elk.edgeRouting": "ORTHOGONAL",
  "elk.layered.spacing.nodeNodeBetweenLayers": String(GAP_X + 110),
  "elk.spacing.nodeNode": String(GAP_Y + 24),
  "elk.layered.crossingMinimization.semiInteractive": "true",
  "elk.layered.nodePlacement.strategy": "BRANDES_KOEPF",
  "elk.spacing.componentComponent": "60",
  "elk.edgeLabels.placement": "CENTER",
  "elk.spacing.edgeLabel": "8",
};
const GROUP_OPTIONS: Record<string, string> = {
  "elk.padding": `[top=${GROUP_HEAD + 8},left=${GROUP_PAD},bottom=${GROUP_PAD},right=${GROUP_PAD}]`,
  "elk.nodeSize.constraints": "MINIMUM_SIZE",
  "elk.nodeSize.minimum": "(260,140)",
};

/**
 * Lays out the Nodes and Groups under `rootId` ("" = the whole diagram).
 * hints: positions to keep the ordering of. Returns absolute boxes.
 */
/**
 * Shared services: components many others talk to (logs, metrics, audit,
 * secrets, keys, identity, backups, threat detection). In a left-to-right
 * flow their many long connections stretch the whole diagram, so they're laid
 * out in a band under the components that use them instead.
 */
export const isSharedService = (kind?: string) =>
  !!kind && (kind.startsWith("observability.") || ["security.secrets", "security.key-management", "security.identity", "security.threat-detection", "data.backup"].includes(kind));

export interface LayoutOptions {
  /** Shared services go in a band under the flow (default on). */
  band?: boolean;
  /** ELK's node placement strategy. */
  placement?: "BRANDES_KOEPF" | "NETWORK_SIMPLEX" | "LINEAR_SEGMENTS";
  /** Gap between columns and between cards in a column. */
  layerGap?: number;
  nodeGap?: number;
  /** Size edge labels in ELK so it leaves room for them (default on). */
  labels?: boolean;
  /** Feed ELK in flow order and break loops by it, so the main path runs left to right (default on). */
  flowOrder?: boolean;
  /** Keep the Rough Draft's positions as ordering hints (default on). */
  hints?: boolean;
  /** Flow direction. */
  direction?: "RIGHT" | "DOWN";
  /** Which column a long connection's label goes in (ELK's centre-label strategy). */
  labelLayer?: "MEDIAN_LAYER" | "TAIL_LAYER" | "HEAD_LAYER" | "SPACE_EFFICIENT_LAYER" | "WIDEST_LAYER" | "CENTER_LAYER";
  /** Width given to ELK for a label at most: gaps fit typical labels; longer ones find room along their path. */
  labelCap?: number;
  /** "recursive": each Group laid out on its own, choosing its direction; "global": one ELK run over everything. */
  mode?: "recursive" | "global";
  /** Screen shape the recursive layout aims for (width / height). */
  aspect?: number;
  /** How much shorter connections count against a more screen-shaped layout. */
  lengthWeight?: number;
}
const DEFAULT_LAYOUT: Required<LayoutOptions> = { band: true, placement: "BRANDES_KOEPF", layerGap: 140, nodeGap: 56, labels: true, flowOrder: true, hints: true, direction: "RIGHT", labelLayer: "MEDIAN_LAYER", labelCap: 9999, mode: "recursive", aspect: 1.6, lengthWeight: 0.1 };

/**
 * How far each component is from where traffic enters (users and clients, or
 * anything nothing points at), following arrows. A loop (traffic in through a
 * gateway, out through NAT back to it) is broken against this order, so the
 * way in runs left to right and only the way back is reversed.
 */
export function flowRank(g: Graph): Map<string, number> {
  const ids = g.nodes.map((n) => n.id);
  const hasIn = new Set(g.edges.map((e) => e.to));
  const out = new Map<string, string[]>();
  for (const e of g.edges) out.set(e.from, [...(out.get(e.from) ?? []), e.to]);
  const rank = new Map<string, number>();
  let frontier = g.nodes.filter((n) => n.kind === "external.user" || !hasIn.has(n.id)).map((n) => n.id);
  if (!frontier.length && ids.length) frontier = [ids[0]];
  for (const id of frontier) rank.set(id, 0);
  while (frontier.length) {
    const next: string[] = [];
    for (const id of frontier) for (const t of out.get(id) ?? []) if (!rank.has(t)) { rank.set(t, rank.get(id)! + 1); next.push(t); }
    frontier = next;
  }
  // Whatever wasn't reached sits just after its earliest ranked neighbour.
  for (let changed = true; changed; ) {
    changed = false;
    for (const e of g.edges) {
      if (rank.has(e.from) && !rank.has(e.to)) { rank.set(e.to, rank.get(e.from)! + 1); changed = true; }
      else if (rank.has(e.to) && !rank.has(e.from)) { rank.set(e.from, Math.max(0, rank.get(e.to)! - 1)); changed = true; }
    }
  }
  for (const id of ids) if (!rank.has(id)) rank.set(id, 0);
  return rank;
}

/**
 * Lays out the Nodes and Groups under `rootId` ("" = the whole diagram):
 * the core flow with ELK (layered, left to right), then shared services in a
 * band under it. ELK rejects some graphs (e.g. a constraint it can't honour),
 * so it retries without constraints and hints, and finally uses a grid; a
 * layout always comes back.
 */
export function elkLayout(g: Graph, hints: Map<string, { x: number; y: number }>, rootId = "", origin = { x: 0, y: 0 }, options: LayoutOptions = {}): Promise<Map<string, Box>> {
  return perf.timeAsync("layout.elk", () => layoutGraph(g, hints, rootId, origin, options), { nodes: g.nodes.length, edges: g.edges.length, groups: g.groups.length, scope: rootId || "all" });
}

async function layoutGraph(g: Graph, hints: Map<string, { x: number; y: number }>, rootId: string, origin: { x: number; y: number }, options: LayoutOptions): Promise<Map<string, Box>> {
  const o = { ...DEFAULT_LAYOUT, ...options };
  if (o.mode === "recursive") return recursiveLayout(g, hints, rootId, origin, o);
  const shared = new Set(o.band ? g.nodes.filter((n) => isSharedService(n.kind)).map((n) => n.id) : []);
  const core: Graph = shared.size
    ? { ...g, nodes: g.nodes.filter((n) => !shared.has(n.id)), edges: g.edges.filter((e) => !shared.has(e.from) && !shared.has(e.to)) }
    : g;
  // Groups that would hold only shared services are left out of the flow too.
  const keepGroup = (id: string): boolean =>
    core.nodes.some((n) => n.parent === id) || g.groups.some((x) => x.parent === id && keepGroup(x.id));
  const coreGraph: Graph = shared.size ? { ...core, groups: g.groups.filter((x) => keepGroup(x.id)) } : core;
  let boxes: Map<string, Box> | null = null;
  for (const constrain of [true, false]) {
    try {
      boxes = await runElk(coreGraph, constrain ? hints : new Map(), rootId, origin, constrain, o);
      break;
    } catch (e) {
      console.warn(`ELK layout failed${constrain ? "; retrying without constraints" : "; using a grid"}:`, e);
    }
  }
  boxes ??= gridLayout(coreGraph, rootId, origin);
  if (shared.size) placeBand(g, boxes, shared);
  return boxes;
}

const BAND_GAP = 90; // between the flow and the band: room for labels on the connections down to it

/**
 * Puts shared services in a row under the flow, inside the Group they belong
 * to: each under the components it connects to (the average position of its
 * neighbours), packed left to right without overlap, wrapping when the row
 * gets wider than the flow above it. Groups grow to fit; anything below a
 * grown Group is pushed down.
 */
function placeBand(g: Graph, boxes: Map<string, Box>, shared: Set<string>) {
  const byParent = new Map<string, GNode[]>();
  for (const n of g.nodes) if (shared.has(n.id)) byParent.set(n.parent ?? "", [...(byParent.get(n.parent ?? "") ?? []), n]);
  const neighbours = (id: string) => g.edges.flatMap((e) => (e.from === id ? [e.to] : e.to === id ? [e.from] : []));
  // Containers deepest first, so a Group's band is in place before its parent's.
  const depth = (id: string) => (id ? ancestors(g, id).length + 1 : 0);
  for (const parent of [...byParent.keys()].sort((a, b) => depth(b) - depth(a))) {
    const items = byParent.get(parent)!;
    const pBox = parent ? boxes.get(parent) : undefined;
    // What the band goes under: the container's laid-out children.
    const kids = [...g.nodes, ...g.groups].filter((x) => (x.parent ?? "") === parent && boxes.has(x.id) && !shared.has(x.id)).map((x) => boxes.get(x.id)!);
    const content = kids.length
      ? { x: Math.min(...kids.map((b) => b.x)), y: Math.min(...kids.map((b) => b.y)), r: Math.max(...kids.map((b) => b.x + b.w)), b: Math.max(...kids.map((b) => b.y + b.h)) }
      : pBox ? { x: pBox.x + GROUP_PAD, y: pBox.y + GROUP_HEAD + 8, r: pBox.x + pBox.w - GROUP_PAD, b: pBox.y + GROUP_HEAD + 8 - BAND_GAP } : { x: 0, y: 0, r: 0, b: -BAND_GAP };
    // Ideal x: under its placed neighbours; services only linked to other services follow those.
    const ideal = new Map<string, number>();
    const centreX = (id: string) => { const b = boxes.get(id); return b ? b.x + b.w / 2 : undefined; };
    for (const n of items) {
      const xs = neighbours(n.id).map(centreX).filter((x): x is number => x !== undefined);
      if (xs.length) ideal.set(n.id, xs.reduce((a, b) => a + b, 0) / xs.length);
    }
    for (const n of items) {
      if (ideal.has(n.id)) continue;
      const xs = neighbours(n.id).map((id) => ideal.get(id)).filter((x): x is number => x !== undefined);
      ideal.set(n.id, xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : (content.x + content.r) / 2);
    }
    // Pack left to right in order of ideal position; wrap when wider than the flow.
    const order = [...items].sort((a, b) => ideal.get(a.id)! - ideal.get(b.id)!);
    const rowWidth = Math.max(content.r - content.x, 4 * CARD_W + 3 * GAP_X);
    let y = content.b + BAND_GAP;
    let row: GNode[] = [];
    const flush = () => {
      if (!row.length) return;
      let prevRight = -Infinity;
      for (const n of row) {
        const want = ideal.get(n.id)! - CARD_W / 2;
        const x = Math.max(want, prevRight + GAP_X, content.x);
        boxes.set(n.id, { x, y, w: CARD_W, h: CARD_H });
        prevRight = x + CARD_W;
      }
      // Slide the row back left if it ran past the flow's right edge.
      const over = prevRight - Math.max(content.r, content.x + rowWidth);
      if (over > 0) for (const n of row) { const b = boxes.get(n.id)!; boxes.set(n.id, { ...b, x: Math.max(content.x, b.x - over) }); }
      y += CARD_H + GAP_Y;
      row = [];
    };
    let used = 0;
    for (const n of order) {
      if (row.length && used + CARD_W > rowWidth) { flush(); used = 0; }
      row.push(n);
      used += CARD_W + GAP_X;
    }
    flush();
    if (pBox) {
      const bandBottom = Math.max(...items.map((n) => boxes.get(n.id)!.y + CARD_H));
      const right = Math.max(...items.map((n) => boxes.get(n.id)!.x + CARD_W));
      const grownH = Math.max(pBox.h, bandBottom + GROUP_PAD - pBox.y);
      const grownW = Math.max(pBox.w, right + GROUP_PAD - pBox.x);
      const dh = grownH - pBox.h;
      boxes.set(parent, { ...pBox, w: grownW, h: grownH });
      if (dh > 0) makeRoomBelow(g, boxes, parent, pBox.y + pBox.h, dh);
    }
  }
  // Groups that hold only shared services weren't laid out: wrap them around their band.
  for (const gr of g.groups) if (!boxes.has(gr.id)) wrapGroup(g, boxes, gr.id);
  growGroups(g, boxes);
}

/** A Group grew downwards: push down what sits below it (siblings, then its ancestors' siblings). */
function makeRoomBelow(g: Graph, boxes: Map<string, Box>, id: string, oldBottom: number, dh: number) {
  let cur = id;
  while (true) {
    const parent = g.groups.find((x) => x.id === cur)?.parent ?? g.nodes.find((x) => x.id === cur)?.parent ?? "";
    const me = boxes.get(cur)!;
    for (const sib of [...g.nodes, ...g.groups]) {
      if (sib.id === cur || (sib.parent ?? "") !== parent) continue;
      const b = boxes.get(sib.id);
      if (!b || b.y < oldBottom - 1 || b.x >= me.x + me.w || b.x + b.w <= me.x) continue;
      shiftSubtree(g, boxes, sib.id, dh);
    }
    if (!parent) break;
    const pb = boxes.get(parent);
    if (!pb) break;
    const need = me.y + me.h + GROUP_PAD - (pb.y + pb.h);
    if (need <= 0) break;
    const pOld = pb.y + pb.h;
    boxes.set(parent, { ...pb, h: pb.h + need });
    cur = parent;
    oldBottom = pOld;
    dh = need;
  }
}

function shiftSubtree(g: Graph, boxes: Map<string, Box>, id: string, dy: number) {
  const b = boxes.get(id);
  if (b) boxes.set(id, { ...b, y: b.y + dy });
  for (const c of [...g.nodes, ...g.groups]) if (c.parent === id) shiftSubtree(g, boxes, c.id, dy);
}

function wrapGroup(g: Graph, boxes: Map<string, Box>, id: string) {
  const kids = [...g.nodes, ...g.groups].filter((x) => x.parent === id).map((x) => boxes.get(x.id)).filter((b): b is Box => !!b);
  if (!kids.length) return;
  const x = Math.min(...kids.map((b) => b.x)) - GROUP_PAD, y = Math.min(...kids.map((b) => b.y)) - GROUP_HEAD - 8;
  const r = Math.max(...kids.map((b) => b.x + b.w)) + GROUP_PAD, bt = Math.max(...kids.map((b) => b.y + b.h)) + GROUP_PAD;
  boxes.set(id, { x, y, w: r - x, h: bt - y });
}

async function runElk(g: Graph, hints: Map<string, { x: number; y: number }>, rootId: string, origin: { x: number; y: number }, constrain: boolean, o: Required<LayoutOptions> = DEFAULT_LAYOUT): Promise<Map<string, Box>> {
  // Users and clients go in the first column only if nothing points at them,
  // external services in the last only if they point at nothing: ELK refuses
  // a FIRST node with incoming edges (or a LAST node with outgoing ones).
  const hasIn = new Set(g.edges.map((e) => e.to));
  const hasOut = new Set(g.edges.map((e) => e.from));
  const layerOf = (n: GNode) => {
    if (!constrain) return undefined;
    if ((n.kind === "external.user" || n.kind === "external.client") && !hasIn.has(n.id)) return "FIRST";
    if ((n.kind === "external.service" || n.kind === "external.system") && !hasOut.has(n.id)) return "LAST";
    return undefined;
  };
  // In flow order: a Group ranks as its earliest component.
  const rank = o.flowOrder ? flowRank(g) : new Map<string, number>();
  const groupRank = (id: string): number => Math.min(Infinity, ...g.nodes.filter((n) => n.parent === id).map((n) => rank.get(n.id) ?? 0), ...g.groups.filter((x) => x.parent === id).map((x) => groupRank(x.id)));
  const rankOf = (id: string) => rank.get(id) ?? groupRank(id);
  if (!o.hints) hints = new Map();
  // Direction is judged where the two ends' containers first differ: a link
  // between two sibling Groups points forward only if the source's Group
  // comes earlier in the flow (a Group holding both the way in and the way
  // out can't straddle the Groups between them).
  const parentOf = (id: string) => g.nodes.find((n) => n.id === id)?.parent ?? g.groups.find((x) => x.id === id)?.parent ?? "";
  const chain = (id: string) => { const c = [id]; let p = parentOf(id); while (p) { c.unshift(p); p = parentOf(p); } return c; };
  const isBackEdge = (from: string, to: string) => {
    const a = chain(from), b = chain(to);
    let i = 0;
    while (i < a.length - 1 && i < b.length - 1 && a[i] === b[i]) i++;
    const ra = rankOf(a[i]), rb = rankOf(b[i]);
    if (a[i] !== b[i] && ra !== rb) return ra > rb;
    return (rank.get(from) ?? 0) > (rank.get(to) ?? 0);
  };
  const kids = (parent: string): ElkNode[] => {
    const out: ElkNode[] = [];
    for (const gr of g.groups.filter((x) => (x.parent ?? "") === parent)) {
      const h = hints.get(gr.id);
      out.push({ id: gr.id, ...(h ? { x: h.x, y: h.y } : {}), layoutOptions: GROUP_OPTIONS, children: kids(gr.id) });
    }
    for (const n of g.nodes.filter((x) => (x.parent ?? "") === parent)) {
      const h = hints.get(n.id);
      const layer = layerOf(n);
      out.push({ id: n.id, width: CARD_W, height: CARD_H, ...(h ? { x: h.x, y: h.y } : {}), ...(layer && parent === "" ? { layoutOptions: { "elk.layered.layering.layerConstraint": layer } } : {}) });
    }
    if (o.flowOrder) out.sort((a, b) => rankOf(a.id) - rankOf(b.id));
    return out;
  };
  const inside = new Set<string>();
  const collect = (parent: string) => {
    for (const gr of g.groups.filter((x) => (x.parent ?? "") === parent)) {
      inside.add(gr.id);
      collect(gr.id);
    }
    for (const n of g.nodes.filter((x) => (x.parent ?? "") === parent)) inside.add(n.id);
  };
  collect(rootId);
  const root: ElkNode = {
    id: "__root",
    layoutOptions: {
      ...ROOT_OPTIONS,
      "elk.layered.nodePlacement.strategy": o.placement,
      "elk.direction": o.direction,
      "elk.layered.edgeLabels.centerLabelPlacementStrategy": o.labelLayer,
      ...(o.flowOrder ? { "elk.layered.considerModelOrder.strategy": "NODES_AND_EDGES" } : {}),
      "elk.layered.spacing.nodeNodeBetweenLayers": String(o.layerGap),
      "elk.spacing.nodeNode": String(o.nodeGap),
    },
    children: kids(rootId),
    // Edge labels are sized so ELK leaves room for them between the cards.
    edges: g.edges.filter((e) => inside.has(e.from) && inside.has(e.to) && e.from !== e.to).sort((x, y) => (rank.get(x.from) ?? 0) - (rank.get(y.from) ?? 0)).map((e) => {
      const text = wrapLabel([e.protocol, e.label].filter(Boolean).join(" · "));
      const size = text && o.labels ? labelSize(text) : null;
      // Connections pointing back against the flow (e.g. egress back out to the gateway traffic came in
      // through) are given to ELK reversed, so it lays out a one-way flow. Drawn arrows keep their direction.
      const back = o.flowOrder && isBackEdge(e.from, e.to);
      const [src, dst] = back ? [e.to, e.from] : [e.from, e.to];
      return { id: e.id, sources: [src], targets: [dst], ...(size ? { labels: [{ text, width: Math.min(size.w, o.labelCap) + 12, height: size.h + 8 }] } : {}) };
    }),
  };
  const out = await (await elk()).layout(root);
  const boxes = new Map<string, Box>();
  const walk = (n: ElkNode, ox: number, oy: number) => {
    for (const c of n.children ?? []) {
      const x = ox + (c.x ?? 0);
      const y = oy + (c.y ?? 0);
      boxes.set(c.id, { x, y, w: c.width ?? CARD_W, h: c.height ?? CARD_H });
      walk(c, x, y);
    }
  };
  walk(out, origin.x, origin.y);
  return boxes;
}

/** Last resort: Groups as boxes in rows, their Nodes in a grid inside. */
function gridLayout(g: Graph, rootId: string, origin: { x: number; y: number }): Map<string, Box> {
  const boxes = new Map<string, Box>();
  const place = (parent: string, x0: number, y0: number): { w: number; h: number } => {
    let x = x0;
    let y = y0;
    let rowH = 0;
    let maxW = 0;
    const cols = 4;
    let col = 0;
    for (const n of g.nodes.filter((n) => (n.parent ?? "") === parent)) {
      boxes.set(n.id, { x, y, w: CARD_W, h: CARD_H });
      rowH = Math.max(rowH, CARD_H);
      x += CARD_W + GAP_X;
      maxW = Math.max(maxW, x - x0);
      if (++col === cols) { col = 0; x = x0; y += rowH + GAP_Y; rowH = 0; }
    }
    if (col) { y += rowH + GAP_Y; }
    for (const gr of g.groups.filter((x) => (x.parent ?? "") === parent)) {
      const inner = place(gr.id, x0 + GROUP_PAD, y + GROUP_HEAD + 8);
      const box = { x: x0, y, w: Math.max(260, inner.w + GROUP_PAD * 2), h: Math.max(140, inner.h + GROUP_HEAD + 8 + GROUP_PAD) };
      boxes.set(gr.id, box);
      y += box.h + GAP_Y;
      maxW = Math.max(maxW, box.w);
    }
    return { w: maxW, h: y - y0 };
  };
  place(rootId, origin.x, origin.y);
  improveBySwaps(g, boxes);
  return boxes;
}

/**
 * Crossing reduction after layout: tries exchanging pairs of same-size cards
 * that share a Group, keeping a swap when it removes crossings between
 * connections (straight lines between card centres) or, at equal crossings,
 * shortens them. Containers only see their own children, so this is where
 * connections between tiers and to shared services get untangled.
 */
function improveBySwaps(g: Graph, boxes: Map<string, Box>, passes = 6) {
  const centre = (id: string): Pt | undefined => { const b = boxes.get(id); return b ? [b.x + b.w / 2, b.y + b.h / 2] : undefined; };
  const edges = g.edges.filter((e) => boxes.has(e.from) && boxes.has(e.to) && e.from !== e.to);
  const cross = (p1: Pt, p2: Pt, q1: Pt, q2: Pt) => {
    const d = (a: Pt, b: Pt, c: Pt) => (b[0] - a[0]) * (c[1] - a[1]) - (b[1] - a[1]) * (c[0] - a[0]);
    const d1 = d(q1, q2, p1), d2 = d(q1, q2, p2), d3 = d(p1, p2, q1), d4 = d(p1, p2, q2);
    return ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) && ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0));
  };
  // Each connection needs room for its label between its two cards.
  const need = edges.map((e) => labelSize(wrapLabel([e.protocol, e.label].filter(Boolean).join(" · "))));
  const cost = () => {
    const segs = edges.map((e) => [centre(e.from)!, centre(e.to)!] as [Pt, Pt]);
    let c = 0;
    for (const [i, e] of edges.entries()) {
      const a = boxes.get(e.from)!, b = boxes.get(e.to)!;
      const gapX = Math.max(a.x, b.x) - Math.min(a.x + a.w, b.x + b.w);
      const gapY = Math.max(a.y, b.y) - Math.min(a.y + a.h, b.y + b.h);
      const roomy = gapX >= need[i].w + 12 || gapY >= need[i].h + 12 || (gapX < 0 && gapY < 0);
      if (!roomy && (gapX > -1 || gapY > -1)) c += 5000; // too close for its label
    }
    for (let i = 0; i < segs.length; i++) {
      const [a, b] = segs[i];
      c += Math.abs(a[0] - b[0]) + Math.abs(a[1] - b[1]);
      for (let j = i + 1; j < segs.length; j++) {
        const e1 = edges[i], e2 = edges[j];
        if (e1.from === e2.from || e1.from === e2.to || e1.to === e2.from || e1.to === e2.to) continue;
        if (cross(a, b, segs[j][0], segs[j][1])) c += 2000;
      }
    }
    return c;
  };
  const cards = g.nodes.filter((n) => boxes.has(n.id));
  let best = cost();
  for (let pass = 0; pass < passes; pass++) {
    let improved = false;
    for (let i = 0; i < cards.length; i++) {
      for (let j = i + 1; j < cards.length; j++) {
        const a = cards[i], b = cards[j];
        if ((a.parent ?? "") !== (b.parent ?? "")) continue;
        if (isSharedService(a.kind) !== isSharedService(b.kind)) continue; // shared services stay in their band
        const ba = boxes.get(a.id)!, bb = boxes.get(b.id)!;
        if (ba.w !== bb.w || ba.h !== bb.h) continue;
        boxes.set(a.id, bb);
        boxes.set(b.id, ba);
        const c = cost();
        if (c < best - 0.5) { best = c; improved = true; }
        else { boxes.set(a.id, ba); boxes.set(b.id, bb); }
      }
    }
    if (!improved) break;
  }
}

// ---------- Recursive layout

/**
 * Lays out the diagram one container at a time, bottom-up. Each Group's
 * direct children (cards, or sub-Groups already sized) form a small flat
 * graph; connections between things deeper down are lifted to the children
 * they live in, so the flow between tiers still counts. A container holding
 * sub-Groups tries both left-to-right and top-to-bottom and keeps the one
 * closer to a screen's shape (subnet tiers stack as rows when that's more
 * compact); everything else flows left to right. Shared services go in a
 * band under each container's flow. Positions are then made absolute
 * top-down. Each ELK run is small and flat: fast, and free of ELK's limits
 * on edges between nested groups.
 */
async function recursiveLayout(g: Graph, hints: Map<string, { x: number; y: number }>, rootId: string, origin: { x: number; y: number }, o: Required<LayoutOptions>): Promise<Map<string, Box>> {
  const rank = flowRank(g);
  const shared = new Set(o.band ? g.nodes.filter((n) => isSharedService(n.kind)).map((n) => n.id) : []);
  const parentOf = (id: string) => g.nodes.find((n) => n.id === id)?.parent ?? g.groups.find((x) => x.id === id)?.parent ?? "";
  const isGroup = (id: string) => g.groups.some((x) => x.id === id);
  const childrenOf = (id: string) => [...g.groups.filter((x) => (x.parent ?? "") === id).map((x) => x.id), ...g.nodes.filter((n) => (n.parent ?? "") === id).map((n) => n.id)];
  const groupRank = (id: string): number => Math.min(Infinity, ...childrenOf(id).map((c) => (isGroup(c) ? groupRank(c) : shared.has(c) ? Infinity : rank.get(c) ?? 0)));
  const rankOf = (id: string) => (isGroup(id) ? groupRank(id) : rank.get(id) ?? 0);
  /** The direct child of `container` that holds `id` (or undefined if it isn't under it). */
  const blockOf = (container: string, id: string): string | undefined => {
    let cur = id;
    for (let guard = 0; guard < 50; guard++) {
      const p = parentOf(cur);
      if (p === container) return cur;
      if (!p) return container === "" ? cur : undefined;
      cur = p;
    }
    return undefined;
  };

  const size = new Map<string, { w: number; h: number }>();
  /** Centre of `deep` relative to Group `grp`'s box, once `grp` has been laid out. */
  const centreIn = (grp: string, deep: string): { x: number; y: number } | undefined => {
    let x = 0, y = 0, cur = deep, first: Box | undefined;
    for (let guard = 0; guard < 50; guard++) {
      const p = parentOf(cur);
      const b = rel.get(p)?.get(cur);
      if (!b) return undefined;
      first ??= b;
      x += b.x + GROUP_PAD;
      y += b.y + GROUP_HEAD + 8;
      if (p === grp) return { x: x + first.w / 2, y: y + first.h / 2 };
      cur = p;
    }
    return undefined;
  };
  const rel = new Map<string, Map<string, Box>>(); // container -> its children's boxes, relative to its content origin

  const layoutContainer = async (id: string): Promise<{ w: number; h: number }> => {
    const kids = childrenOf(id);
    for (const k of kids) if (isGroup(k)) size.set(k, await layoutContainer(k));
    const flow = kids.filter((k) => !shared.has(k));
    const band = kids.filter((k) => shared.has(k));
    const dims = (k: string) => (isGroup(k) ? size.get(k)! : { w: CARD_W, h: CARD_H });

    // Connections between this container's children (lifted from deeper down).
    // da/db: the card (or Group) deeper down that the connection really starts or ends at.
    const lifted = new Map<string, { a: string; b: string; da: string; db: string; label?: { w: number; h: number } }>();
    for (const e of g.edges) {
      const a = blockOf(id, e.from), b = blockOf(id, e.to);
      if (!a || !b || a === b || shared.has(a) || shared.has(b)) continue;
      const back = rankOf(a) > rankOf(b);
      const [src, dst] = back ? [b, a] : [a, b];
      const [ds, dd] = back ? [e.to, e.from] : [e.from, e.to];
      const key = `${src}|${ds}>${dst}|${dd}`;
      const direct = a === e.from && b === e.to;
      const text = direct && o.labels ? wrapLabel([e.protocol, e.label].filter(Boolean).join(" · ")) : "";
      const lab = text ? labelSize(text) : undefined;
      const prev = lifted.get(key);
      if (!prev || (lab && (!prev.label || lab.w * lab.h > prev.label.w * prev.label.h))) lifted.set(key, { a: src, b: dst, da: ds, db: dd, label: lab ?? prev?.label });
    }

    const order = [...flow].sort((x, y) => rankOf(x) - rankOf(y) || (hints.get(x)?.y ?? 0) - (hints.get(y)?.y ?? 0));
    const tryLayout = async (direction: "RIGHT" | "DOWN", joined = false): Promise<Map<string, Box>> => {
      if (!order.length) return new Map();
      // A connection into a Group attaches to a port on its border level with the card inside it, so
      // what's outside lines up with what it really connects to rather than the Group's middle.
      const ports = new Map<string, Map<string, { id: string; x: number; y: number; width: number; height: number; layoutOptions: Record<string, string> }>>();
      const end = (block: string, deep: string, way: "in" | "out"): string => {
        if (block === deep || !isGroup(block)) return block;
        const c = centreIn(block, deep);
        if (!c) return block;
        const d = dims(block);
        const id = `${block}::${deep}::${way}`;
        const side = direction === "RIGHT" ? (way === "in" ? "WEST" : "EAST") : way === "in" ? "NORTH" : "SOUTH";
        const at = direction === "RIGHT" ? { x: way === "in" ? 0 : d.w, y: c.y } : { x: c.x, y: way === "in" ? 0 : d.h };
        if (!ports.has(block)) ports.set(block, new Map());
        ports.get(block)!.set(id, { id, ...at, width: 1, height: 1, layoutOptions: { "elk.port.side": side } });
        return id;
      };
      const edges = [...lifted.values()].map((l, i) => ({ id: `e${i}`, sources: [end(l.a, l.da, "out")], targets: [end(l.b, l.db, "in")], ...(l.label ? { labels: [{ text: "", width: Math.min(l.label.w, o.labelCap) + 12, height: l.label.h + 8 }] } : {}) }));
      const graph = {
        id: "c",
        layoutOptions: {
          "elk.algorithm": "layered",
          "elk.direction": direction,
          "elk.edgeRouting": "ORTHOGONAL",
          "elk.layered.spacing.nodeNodeBetweenLayers": String(o.layerGap),
          "elk.spacing.nodeNode": String(o.nodeGap),
          "elk.layered.nodePlacement.strategy": o.placement,
          "elk.layered.considerModelOrder.strategy": "NODES_AND_EDGES",
          "elk.edgeLabels.placement": "CENTER",
          "elk.spacing.edgeLabel": "8",
          "elk.spacing.componentComponent": String(o.layerGap),
          // Joined: unconnected children share the first layer instead of becoming columns of their own.
          "elk.separateConnectedComponents": String(!joined),
        },
        children: order.map((k) => {
          const pts = ports.get(k);
          return { id: k, width: dims(k).w, height: dims(k).h, ...(pts ? { ports: [...pts.values()], layoutOptions: { "elk.portConstraints": "FIXED_POS" } } : {}) };
        }),
        edges,
      };
      try {
        const out = await (await elk()).layout(graph);
        return new Map((out.children ?? []).map((c) => [c.id, { x: c.x ?? 0, y: c.y ?? 0, w: c.width ?? CARD_W, h: c.height ?? CARD_H }]));
      } catch (err) {
        console.warn("ELK layout failed; using a grid:", err);
        const m = new Map<string, Box>();
        let x = 0;
        for (const k of order) { const d = dims(k); m.set(k, { x, y: 0, ...d }); x += d.w + o.layerGap; }
        return m;
      }
    };

    // Groups of groups (e.g. a VPC of subnet tiers) may stack; the band may go under or beside the
    // flow. Keep the candidate that is most screen-shaped with the shortest connections.
    const flows: Map<string, Box>[] = [await tryLayout("RIGHT")];
    if (flow.filter(isGroup).length >= 2) flows.push(await tryLayout("DOWN"));
    // A child with no connections here (e.g. a bucket only the CDN outside reads) may sit beside the flow.
    const unconnected = order.filter((k) => ![...lifted.values()].some((l) => l.a === k || l.b === k));
    if (unconnected.length && order.length > unconnected.length) flows.push(await tryLayout("RIGHT", true));
    let best: { boxes: Map<string, Box>; w: number; h: number; score: number } | null = null;
    for (const c of flows) {
      for (const side of band.length ? (["below", "right"] as const) : (["below"] as const)) {
        const placed = withBand(c, band, id, side);
        const all = [...placed.values()];
        const w = all.length ? Math.max(...all.map((b) => b.x + b.w)) : 0;
        const h = all.length ? Math.max(...all.map((b) => b.y + b.h)) : 0;
        const score = Math.max(w / o.aspect, h) + 0.5 * Math.min(w / o.aspect, h) + o.lengthWeight * connectionLength(placed, id);
        if (!best || score < best.score - 1) best = { boxes: placed, w, h, score };
      }
    }
    rel.set(id, best!.boxes);
    if (id === rootId) return { w: best!.w, h: best!.h };
    return { w: Math.max(260, best!.w + GROUP_PAD * 2), h: Math.max(140, best!.h + GROUP_HEAD + 8 + GROUP_PAD) };
  };

  /** Space between two neighbouring band items: room for a label when they're connected to each other. */
  const bandGap = (a: string | undefined, b: string, along: "x" | "y") => {
    if (!a) return along === "x" ? GAP_X : GAP_Y;
    const e = g.edges.find((x) => (x.from === a && x.to === b) || (x.from === b && x.to === a));
    if (!e) return along === "x" ? GAP_X : GAP_Y;
    const size = labelSize(wrapLabel([e.protocol, e.label].filter(Boolean).join(" · ")));
    return along === "x" ? Math.max(GAP_X, size.w + 28) : Math.max(GAP_Y, size.h + 28);
  };

  /** Total length of the connections between a container's children (centre to centre), lifted from deeper down. */
  const connectionLength = (boxes: Map<string, Box>, container: string) => {
    let total = 0;
    for (const e of g.edges) {
      const a = blockOf(container, e.from), b = blockOf(container, e.to);
      const ba = a ? boxes.get(a) : undefined, bb = b ? boxes.get(b) : undefined;
      if (!ba || !bb || a === b) continue;
      total += Math.abs(ba.x + ba.w / 2 - (bb.x + bb.w / 2)) + Math.abs(ba.y + ba.h / 2 - (bb.y + bb.h / 2));
    }
    return total;
  };

  /**
   * Adds a container's shared services in a band under its flow (each under the children it connects
   * to) or in a column to its right (each beside them).
   */
  const withBand = (flowBoxes: Map<string, Box>, band: string[], container: string, side: "below" | "right" = "below"): Map<string, Box> => {
    const out = new Map(flowBoxes);
    if (!band.length) return out;
    if (side === "right") {
      const all0 = [...flowBoxes.values()];
      const right0 = all0.length ? Math.max(...all0.map((b) => b.x + b.w)) + BAND_GAP + 40 : 0;
      const idealY = new Map<string, number>();
      for (const sId of band) {
        const ys: number[] = [];
        for (const e of g.edges) {
          const other = e.from === sId ? e.to : e.to === sId ? e.from : null;
          const blk = other ? blockOf(container, other) : undefined;
          const b = blk ? flowBoxes.get(blk) : undefined;
          if (b) ys.push(b.y + b.h / 2);
        }
        idealY.set(sId, ys.length ? ys.reduce((a, b) => a + b, 0) / ys.length : 0);
      }
      // Each service as level with what it connects to as it can be, spread over the flow's height
      // when there's room (space between them for the labels of the connections coming in), and
      // pulled back up when the column would run past the flow's bottom.
      const sorted = [...band].sort((a, b) => idealY.get(a)! - idealY.get(b)!);
      const height = all0.length ? Math.max(...all0.map((b) => b.y + b.h)) : 0;
      const slack = sorted.length > 1 ? (height - sorted.length * CARD_H) / (sorted.length - 1) : 0;
      const gapAfter = sorted.map((sId, i) => (i ? Math.max(bandGap(sorted[i - 1], sId, "y"), Math.min(slack, CARD_H * 2)) : 0));
      const ys: number[] = [];
      for (const [i, sId] of sorted.entries()) ys.push(Math.max(idealY.get(sId)! - CARD_H / 2, i ? ys[i - 1] + CARD_H + gapAfter[i] : 0, 0));
      for (let i = sorted.length - 1; i >= 0; i--) {
        const limit = i === sorted.length - 1 ? height - CARD_H : ys[i + 1] - CARD_H - gapAfter[i + 1];
        ys[i] = Math.max(0, Math.min(ys[i], limit));
      }
      for (const [i, sId] of sorted.entries()) out.set(sId, { x: right0, y: ys[i], w: CARD_W, h: CARD_H });
      return out;
    }
    const all = [...flowBoxes.values()];
    const right = all.length ? Math.max(...all.map((b) => b.x + b.w)) : 0;
    const bottom = all.length ? Math.max(...all.map((b) => b.y + b.h)) + BAND_GAP : 0;
    const ideal = new Map<string, number>();
    for (const sId of band) {
      const xs: number[] = [];
      for (const e of g.edges) {
        const other = e.from === sId ? e.to : e.to === sId ? e.from : null;
        if (!other) continue;
        const blk = blockOf(container, other);
        const b = blk ? flowBoxes.get(blk) : undefined;
        if (b) xs.push(b.x + b.w / 2);
      }
      if (xs.length) ideal.set(sId, xs.reduce((a, b) => a + b, 0) / xs.length);
    }
    for (const sId of band) {
      if (ideal.has(sId)) continue;
      const xs = g.edges.flatMap((e) => (e.from === sId ? [e.to] : e.to === sId ? [e.from] : [])).map((x) => ideal.get(x)).filter((x): x is number => x !== undefined);
      ideal.set(sId, xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : right / 2);
    }
    const rowWidth = Math.max(right, 4 * CARD_W + 3 * GAP_X);
    let y = bottom;
    let row: string[] = [];
    const flush = () => {
      let prevRight = -Infinity;
      let prev: string | undefined;
      for (const sId of row) {
        const x = Math.max(ideal.get(sId)! - CARD_W / 2, prevRight + bandGap(prev, sId, "x"), 0);
        out.set(sId, { x, y, w: CARD_W, h: CARD_H });
        prevRight = x + CARD_W;
        prev = sId;
      }
      const over = prevRight - Math.max(right, rowWidth);
      if (over > 0) for (const sId of row) { const b = out.get(sId)!; out.set(sId, { ...b, x: Math.max(0, b.x - over) }); }
      y += CARD_H + GAP_Y;
      row = [];
    };
    let used = 0;
    for (const sId of [...band].sort((a, b) => ideal.get(a)! - ideal.get(b)!)) {
      if (row.length && used + CARD_W > rowWidth) { flush(); used = 0; }
      row.push(sId);
      used += CARD_W + GAP_X;
    }
    if (row.length) flush();
    return out;
  };

  await layoutContainer(rootId);

  // Top-down: absolute positions.
  const boxes = new Map<string, Box>();
  const place = (id: string, ox: number, oy: number) => {
    for (const [k, b] of rel.get(id) ?? []) {
      const ab = { x: ox + b.x, y: oy + b.y, w: b.w, h: b.h };
      boxes.set(k, ab);
      if (isGroup(k)) place(k, ab.x + GROUP_PAD, ab.y + GROUP_HEAD + 8);
    }
  };
  place(rootId, origin.x, origin.y);
  improveBySwaps(g, boxes);
  return boxes;
}


// ---------- Local placement for Proposals

export interface PlaceOptions {
  /** Size of a new Node (default: a Production card). */
  size?: { w: number; h: number };
  /** Space kept between shapes. */
  gap?: number;
  /** Segments of existing arrows: a new shape shouldn't sit on top of them. */
  avoid?: [Pt, Pt][];
  /** Width of an edge's label: side-by-side shapes leave room for it. */
  labelWidth?: (from: string, to: string) => number;
}

function ancestors(g: Graph, id: string): string[] {
  const out: string[] = [];
  let p = g.nodes.find((n) => n.id === id)?.parent ?? g.groups.find((x) => x.id === id)?.parent;
  const seen = new Set<string>();
  while (p && !seen.has(p)) {
    seen.add(p);
    out.push(p);
    p = g.groups.find((x) => x.id === p)?.parent;
  }
  return out;
}

/**
 * Places new and moved Nodes and Groups of a Proposal (ticket 016): a Node
 * goes inside its parent Group, next to the Node it connects to most, on the
 * side the diagram's flow says (downstream of what feeds it, upstream of what
 * it feeds). Candidate slots are scored: never overlapping another shape,
 * avoiding existing arrows, and keeping its own arrows clear of other
 * shapes. Groups grow to fit and never shrink. Nothing else moves.
 */
export function placeChanges(result: Graph, geo: Map<string, Box>, place: Set<string>, opts: PlaceOptions = {}): Map<string, Box> {
  const size = opts.size ?? { w: CARD_W, h: CARD_H };
  const gap = opts.gap ?? GAP_X;
  const avoid = opts.avoid ?? [];
  const out = new Map(geo);
  for (const id of place) out.delete(id);
  const flow = flowDirection(result.edges, geo);
  const isGroup = (id: string) => result.groups.some((x) => x.id === id);

  // Free: not overlapping any placed shape, except the Groups it sits in.
  const free = (b: Box, parent: string | undefined, self: string) => {
    const anc = new Set(parent ? [parent, ...ancestors(result, parent)] : []);
    for (const [id, o] of out) {
      if (id === self || anc.has(id)) continue;
      // Other Groups and Nodes: keep clear of them, with room for arrow labels.
      if (boxesOverlap(b, o, isGroup(id) ? gap / 3 : gap / 2)) return false;
    }
    return true;
  };
  const fitsParent = (b: Box, parent: string | undefined) => {
    const p = parent ? out.get(parent) : undefined;
    if (!p) return true;
    return b.x >= p.x + GROUP_PAD - 1 && b.y >= p.y + GROUP_HEAD - 1 && b.x + b.w <= p.x + p.w - GROUP_PAD + 1;
  };
  // Lower is better: arrows under the box, and its own arrows crossing other shapes.
  const cost = (b: Box, self: string, origin: Pt) => {
    let c = 0;
    const padded = inflate(b, 6);
    for (const [p, q] of avoid) if (segmentHitsBox(p, q, padded)) c += 50;
    const mid = center(b);
    for (const e of result.edges) {
      const other = e.from === self ? e.to : e.to === self ? e.from : null;
      const ob = other ? out.get(other) : undefined;
      if (!other || !ob || isGroup(other)) continue;
      for (const [id, o] of out) {
        if (id === other || id === self || isGroup(id)) continue;
        if (segmentHitsBox(mid, center(ob), inflate(o, 4))) c += 30;
      }
    }
    return c + Math.hypot(mid[0] - origin[0], mid[1] - origin[1]) / 100;
  };

  const groupsToPlace = result.groups.filter((gr) => place.has(gr.id));
  const nodesToPlace = result.nodes.filter((n) => place.has(n.id));

  // New Groups first, outermost first, sized for what they will hold.
  const depth = (gr: GGroup) => ancestors(result, gr.id).length;
  for (const gr of groupsToPlace.sort((a, b) => depth(a) - depth(b))) {
    const leaves = result.nodes.filter((n) => n.parent === gr.id).length;
    const cols = Math.max(1, Math.min(3, leaves));
    const rows = Math.max(1, Math.ceil(leaves / cols));
    const w = Math.max(260, GROUP_PAD * 2 + cols * size.w + (cols - 1) * gap);
    const h = Math.max(140, GROUP_HEAD + 8 + GROUP_PAD + rows * size.h + (rows - 1) * GAP_Y);
    out.set(gr.id, scanSlot(out, gr.id, gr.parent, w, h, gap, free));
  }

  for (const n of nodesToPlace) {
    const parent = n.parent || undefined;
    const anchor = anchorOf(result, out, n, isGroup);
    const w = size.w;
    const h = size.h;
    let chosen: Box | null = null;
    if (anchor) {
      const a = out.get(anchor.id)!;
      // Downstream of what feeds it, upstream of what it feeds; then sideways, then diagonally.
      const fwd: Pt = anchor.downstream ? flow : [-flow[0], -flow[1]];
      const side: Pt = [fwd[1], fwd[0]];
      const dirs: Pt[] = [fwd, side, [-side[0], -side[1]], [fwd[0] + side[0], fwd[1] + side[1]], [fwd[0] - side[0], fwd[1] - side[1]], [-fwd[0], -fwd[1]]];
      const origin = center(a);
      let best = Infinity;
      for (const [di, d] of dirs.entries()) {
        // Side by side, the arrow between them needs room for its label.
        const label = opts.labelWidth ? Math.max(opts.labelWidth(anchor.id, n.id), opts.labelWidth(n.id, anchor.id)) : 0;
        const gx = d[0] !== 0 ? Math.max(gap, label + 40) : gap;
        for (let k = 1; k <= 4; k++) {
          const cx = origin[0] + d[0] * k * ((a.w + w) / 2 + gx);
          const cy = origin[1] + d[1] * k * ((a.h + h) / 2 + gap);
          const b = { x: cx - w / 2, y: cy - h / 2, w, h };
          if (!fitsParent(b, parent) || !free(b, parent, n.id)) continue;
          const c = cost(b, n.id, origin) + di * 2 + k;
          if (c < best) {
            best = c;
            chosen = b;
          }
          break; // the nearest free slot in this direction
        }
      }
    }
    out.set(n.id, chosen ?? scanSlot(out, n.id, parent, w, h, gap, free));
  }

  growGroups(result, out, geo);
  return out;
}

/** The placed Node a new Node connects to most (preferring its own Group), and whether it sits downstream of it. */
function anchorOf(g: Graph, out: Map<string, Box>, n: GNode, isGroup: (id: string) => boolean) {
  const count = new Map<string, { n: number; incoming: number }>();
  for (const e of g.edges) {
    const other = e.from === n.id ? e.to : e.to === n.id ? e.from : null;
    if (!other || !out.has(other) || isGroup(other)) continue;
    const c = count.get(other) ?? { n: 0, incoming: 0 };
    c.n++;
    if (e.to === n.id) c.incoming++;
    count.set(other, c);
  }
  let best: { id: string; downstream: boolean } | null = null;
  let score = -1;
  for (const [id, c] of count) {
    const same = (g.nodes.find((x) => x.id === id)?.parent || undefined) === (n.parent || undefined) ? 1000 : 0;
    if (c.n + same > score) {
      score = c.n + same;
      best = { id, downstream: c.incoming * 2 >= c.n };
    }
  }
  return best;
}

/** First free slot inside a Group (scanning rows), or beside everything at the top level. */
function scanSlot(out: Map<string, Box>, self: string, parent: string | undefined, w: number, h: number, gap: number, free: (b: Box, parent: string | undefined, self: string) => boolean): Box {
  const p = parent ? out.get(parent) : undefined;
  if (p) {
    const x0 = p.x + GROUP_PAD;
    const y0 = p.y + GROUP_HEAD + 8;
    const cols = Math.max(1, Math.floor((p.w - GROUP_PAD * 2 + gap) / (w + gap)));
    for (let row = 0; row < 50; row++) {
      for (let c = 0; c < cols; c++) {
        const b = { x: x0 + c * (w + gap), y: y0 + row * (h + GAP_Y), w, h };
        if (free(b, parent, self)) return b;
      }
    }
  }
  let maxX = -Infinity;
  let minY = Infinity;
  for (const [id, b] of out) {
    if (id === self) continue;
    maxX = Math.max(maxX, b.x + b.w);
    minY = Math.min(minY, b.y);
  }
  if (!Number.isFinite(maxX)) return { x: 0, y: 0, w, h };
  for (let row = 0; row < 50; row++) {
    const b = { x: maxX + gap * 2, y: minY + row * (h + GAP_Y), w, h };
    if (free(b, parent, self)) return b;
  }
  return { x: maxX + gap * 2, y: minY, w, h };
}

/** Groups grow to contain their children; they never shrink (ticket 016). */
export function growGroups(g: Graph, out: Map<string, Box>, before?: Map<string, Box>) {
  const depth = (id: string) => ancestors(g, id).length;
  const groups = [...g.groups].sort((a, b) => depth(b.id) - depth(a.id)); // innermost first
  for (const gr of groups) {
    const box = out.get(gr.id);
    if (!box) continue;
    let { x, y } = box;
    let r = box.x + box.w;
    let bot = box.y + box.h;
    for (const c of [...g.nodes.filter((n) => n.parent === gr.id), ...g.groups.filter((x) => x.parent === gr.id)]) {
      const cb = out.get(c.id);
      if (!cb) continue;
      x = Math.min(x, cb.x - GROUP_PAD);
      y = Math.min(y, cb.y - GROUP_HEAD - 8);
      r = Math.max(r, cb.x + cb.w + GROUP_PAD);
      bot = Math.max(bot, cb.y + cb.h + GROUP_PAD);
    }
    const prev = before?.get(gr.id);
    if (prev) {
      x = Math.min(x, prev.x);
      y = Math.min(y, prev.y);
      r = Math.max(r, prev.x + prev.w);
      bot = Math.max(bot, prev.y + prev.h);
    }
    out.set(gr.id, { x, y, w: r - x, h: bot - y });
  }
}
