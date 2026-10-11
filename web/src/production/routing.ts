// Routes every arrow of a Production Diagram and places their labels, in two
// passes: first all paths (each in its own lane, around cards, off Group
// borders), then every label at the clearest spot along its own path, once
// all lines are known. Pure: no Excalidraw, so it also runs in the harness.
import type { GEdge, Graph } from "../types";
import { GROUP_HEAD, type Box } from "./constants";
import { bestRoute, labelSize, placeLabelsAvoiding, wrapLabel, type Pt, type Route } from "./geometry";

/** text: what the arrow shows (wrapped); label: its size. */
export type RoutedEdge = { route: Route; abs: Pt[]; labelAt: number; label?: { w: number; h: number }; text: string };

/** What an arrow shows: protocol and label, wrapped to keep it narrow. */
export const edgeText = (e: GEdge, width = 20) => wrapLabel([e.protocol, e.label].filter(Boolean).join(" · "), width);
export const edgeLabelSize = (e: GEdge) => {
  const t = edgeText(e);
  return t ? labelSize(t) : undefined;
};

export function routeEdges(g: Graph, geo: Map<string, Box>): Map<string, RoutedEdge> {
  const has = (id: string) => geo.has(id) && (g.nodes.some((n) => n.id === id) || g.groups.some((x) => x.id === id));
  // Arrows avoid cards they don't connect (Groups are crossed freely).
  const cardBoxes = g.nodes.filter((n) => geo.has(n.id)).map((n) => ({ id: n.id, b: geo.get(n.id)! }));
  // Arrow labels stay off Group titles (the top-left corner of each Group).
  const reserved: Box[] = g.groups.filter((x) => geo.has(x.id)).map((x) => { const b = geo.get(x.id)!; return { x: b.x, y: b.y, w: Math.min(b.w, 220), h: GROUP_HEAD }; });
  // Group borders arrows shouldn't run along.
  const groupBoxes = g.groups.filter((x) => geo.has(x.id)).map((x) => geo.get(x.id)!);
  const lines: [Pt, Pt][] = [];
  const toLabel: { id: string; pts: Pt[]; size: { w: number; h: number }; alt: { w: number; h: number }[] }[] = [];
  const narrow = new Map<string, string>();
  const obstacles = (from: string, to: string) => cardBoxes.filter((c) => c.id !== from && c.id !== to).map((c) => c.b);
  // Short arrows first: they have the fewest ways around things.
  const span = (e: GEdge) => { const p = geo.get(e.from), q = geo.get(e.to); return p && q ? Math.abs(p.x - q.x) + Math.abs(p.y - q.y) : 0; };
  const out = new Map<string, RoutedEdge>();
  for (const e of [...g.edges].sort((x, y) => span(x) - span(y))) {
    if (!has(e.from) || !has(e.to) || e.from === e.to) continue;
    const size = edgeLabelSize(e);
    // Pass 1: the path.
    const route = bestRoute(geo.get(e.from)!, geo.get(e.to)!, obstacles(e.from, e.to), false, undefined, reserved, lines, groupBoxes);
    const abs = route.points.map(([x, y]) => [x + route.x, y + route.y] as Pt);
    const tight = edgeText(e, 12);
    narrow.set(e.id, tight);
    if (size) toLabel.push({ id: e.id, pts: abs, size, alt: [labelSize(tight)] });
    for (let i = 1; i < abs.length; i++) lines.push([abs[i - 1], abs[i]]);
    out.set(e.id, { route, abs, labelAt: 0.5, label: size, text: edgeText(e) });
  }
  // Pass 2: every label, now that every line is known.
  const spots = placeLabelsAvoiding(toLabel, cardBoxes.map((c) => c.b), reserved);
  for (const [id, { t, variant }] of spots) {
    const r = out.get(id);
    if (!r) continue;
    r.labelAt = t;
    r.route.labelAt = t;
    if (variant === 1) {
      r.text = narrow.get(id)!;
      r.label = labelSize(r.text);
    }
  }
  return out;
}
