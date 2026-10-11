// Geometry for placing and routing what the runtime draws: which way a
// diagram flows, whether a segment crosses a box, and arrow routes that
// avoid other shapes.
export type Box = { x: number; y: number; w: number; h: number };
export type Pt = [number, number];
export type Side = "left" | "right" | "top" | "bottom";

export const center = (b: Box): Pt => [b.x + b.w / 2, b.y + b.h / 2];

export function inflate(b: Box, pad: number): Box {
  return { x: b.x - pad, y: b.y - pad, w: b.w + pad * 2, h: b.h + pad * 2 };
}

export function boxesOverlap(a: Box, b: Box, pad = 0): boolean {
  return a.x < b.x + b.w + pad && b.x < a.x + a.w + pad && a.y < b.y + b.h + pad && b.y < a.y + a.h + pad;
}

/** Whether segment p→q passes through box b (Liang–Barsky clipping). */
export function segmentHitsBox(p: Pt, q: Pt, b: Box): boolean {
  let t0 = 0;
  let t1 = 1;
  const dx = q[0] - p[0];
  const dy = q[1] - p[1];
  const checks: [number, number][] = [
    [-dx, p[0] - b.x],
    [dx, b.x + b.w - p[0]],
    [-dy, p[1] - b.y],
    [dy, b.y + b.h - p[1]],
  ];
  for (const [pp, qq] of checks) {
    if (pp === 0) {
      if (qq < 0) return false;
      continue;
    }
    const r = qq / pp;
    if (pp < 0) {
      if (r > t1) return false;
      if (r > t0) t0 = r;
    } else {
      if (r < t0) return false;
      if (r < t1) t1 = r;
    }
  }
  return t1 - t0 > 1e-6;
}

export function polylineHits(points: Pt[], b: Box): boolean {
  for (let i = 1; i < points.length; i++) if (segmentHitsBox(points[i - 1], points[i], b)) return true;
  return false;
}

export function polylineLength(points: Pt[]): number {
  let l = 0;
  for (let i = 1; i < points.length; i++) l += Math.hypot(points[i][0] - points[i - 1][0], points[i][1] - points[i - 1][1]);
  return l;
}

/** The unit direction the diagram's arrows mostly point in (right, left, up or down). */
export function flowDirection(edges: { from: string; to: string }[], geo: Map<string, Box>): Pt {
  let sx = 0;
  let sy = 0;
  for (const e of edges) {
    const a = geo.get(e.from);
    const b = geo.get(e.to);
    if (!a || !b) continue;
    const [ax, ay] = center(a);
    const [bx, by] = center(b);
    const d = Math.hypot(bx - ax, by - ay) || 1;
    sx += (bx - ax) / d;
    sy += (by - ay) / d;
  }
  if (Math.abs(sx) < 0.5 && Math.abs(sy) < 0.5) return [1, 0];
  return Math.abs(sx) >= Math.abs(sy) ? [Math.sign(sx), 0] : [0, Math.sign(sy)];
}

const horizontal = (s: Side) => s === "left" || s === "right";
const outward = (s: Side): Pt => (s === "left" ? [-1, 0] : s === "right" ? [1, 0] : s === "top" ? [0, -1] : [0, 1]);

/** labelAt: where along the path (0–1, by length) the label sits — Excalidraw's labelPosition. */
export type Route = { x: number; y: number; points: Pt[]; fromPoint: Pt; toPoint: Pt; labelAt: number };

/** The orthogonal path between two side points (straight, L or Z). */
function orthogonal(a: Pt, sa: Side, b: Pt, sb: Side): Pt[] {
  if (horizontal(sa) && horizontal(sb)) {
    if (Math.abs(a[1] - b[1]) < 1) return [a, b];
    const mx = (a[0] + b[0]) / 2;
    return [a, [mx, a[1]], [mx, b[1]], b];
  }
  if (!horizontal(sa) && !horizontal(sb)) {
    if (Math.abs(a[0] - b[0]) < 1) return [a, b];
    const my = (a[1] + b[1]) / 2;
    return [a, [a[0], my], [b[0], my], b];
  }
  return horizontal(sa) ? [a, [b[0], a[1]], b] : [a, [a[0], b[1]], b];
}

/** A U-shaped route leaving and entering on the same side, `off` beyond both shapes. */
function around(p: Pt, q: Pt, s: Side, a: Box, b: Box, off: number): Pt[] {
  switch (s) {
    case "bottom": { const y = Math.max(a.y + a.h, b.y + b.h) + off; return [p, [p[0], y], [q[0], y], q]; }
    case "top": { const y = Math.min(a.y, b.y) - off; return [p, [p[0], y], [q[0], y], q]; }
    case "right": { const x = Math.max(a.x + a.w, b.x + b.w) + off; return [p, [x, p[1]], [x, q[1]], q]; }
    case "left": { const x = Math.min(a.x, b.x) - off; return [p, [x, p[1]], [x, q[1]], q]; }
  }
}

/** Whether a route leaves and enters along its sides' outward directions (no doubling back into a shape). */
function sane(points: Pt[], sa: Side, sb: Side): boolean {
  if (points.length < 2) return false;
  const [ox, oy] = outward(sa);
  const first = [points[1][0] - points[0][0], points[1][1] - points[0][1]];
  const [ix, iy] = outward(sb);
  const n = points.length;
  const last = [points[n - 2][0] - points[n - 1][0], points[n - 2][1] - points[n - 1][1]];
  return first[0] * ox + first[1] * oy > 0 && last[0] * ix + last[1] * iy > 0;
}

/** Two axis-aligned segments lying on the same line (within `tol`) and overlapping by more than a few px. */
function collinearOverlap(p1: Pt, p2: Pt, q1: Pt, q2: Pt, tol = 6): boolean {
  const ph = Math.abs(p1[1] - p2[1]) < 0.5, qh = Math.abs(q1[1] - q2[1]) < 0.5;
  const pv = Math.abs(p1[0] - p2[0]) < 0.5, qv = Math.abs(q1[0] - q2[0]) < 0.5;
  const overlap = (a1: number, a2: number, b1: number, b2: number) => Math.min(Math.max(a1, a2), Math.max(b1, b2)) - Math.max(Math.min(a1, a2), Math.min(b1, b2)) > 4;
  if (ph && qh && Math.abs(p1[1] - q1[1]) < tol) return overlap(p1[0], p2[0], q1[0], q2[0]);
  if (pv && qv && Math.abs(p1[0] - q1[0]) < tol) return overlap(p1[1], p2[1], q1[1], q2[1]);
  return false;
}

/** Whether two segments properly cross. */
function crosses(p1: Pt, p2: Pt, q1: Pt, q2: Pt): boolean {
  const d = (a: Pt, b: Pt, c: Pt) => (b[0] - a[0]) * (c[1] - a[1]) - (b[1] - a[1]) * (c[0] - a[0]);
  const d1 = d(q1, q2, p1), d2 = d(q1, q2, p2), d3 = d(p1, p2, q1), d4 = d(p1, p2, q2);
  return ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) && ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0));
}

/** The four edges of a box, as segments (Group borders a route shouldn't run along). */
function boxEdges(b: Box): [Pt, Pt][] {
  const tl: Pt = [b.x, b.y], tr: Pt = [b.x + b.w, b.y], bl: Pt = [b.x, b.y + b.h], br: Pt = [b.x + b.w, b.y + b.h];
  return [[tl, tr], [bl, br], [tl, bl], [tr, br]];
}

/** A point on a side at `r` (0–1) along it, and the matching Excalidraw fixedPoint. */
function sideAt(b: Box, s: Side, r: number): { pt: Pt; fp: Pt } {
  switch (s) {
    case "left": return { pt: [b.x, b.y + b.h * r], fp: [0, r] };
    case "right": return { pt: [b.x + b.w, b.y + b.h * r], fp: [1, r] };
    case "top": return { pt: [b.x + b.w * r, b.y], fp: [r, 0] };
    case "bottom": return { pt: [b.x + b.w * r, b.y + b.h], fp: [r, 1] };
  }
}

/**
 * The best route from box a to box b. Scored, lowest wins:
 * crossing a shape ≫ running along another arrow or a Group border ≫ its label
 * landing on a shape, label or line ≫ running through a label ≫ crossing
 * other arrows > length and bends. Orthogonal routes always; straight lines
 * too when `straight` (hand-drawn arrows). A quick search (side centres)
 * comes first; only if that route still collides does a wider one try other
 * attachment points along the sides, middle-leg positions and detours, so
 * parallel arrows get lanes of their own.
 */
export function bestRoute(
  a: Box, b: Box, obstacles: Box[], straight: boolean,
  label?: { w: number; h: number }, labelAvoid: Box[] = [], lineAvoid: [Pt, Pt][] = [], borders: Box[] = [],
): Route {
  const sides: Side[] = ["right", "left", "bottom", "top"];
  const borderSegs = borders.flatMap(boxEdges);
  let best: { pts: Pt[]; fa: Pt; fb: Pt; score: number; at: number } | null = null;
  const consider = (pts: Pt[], fa: Pt, fb: Pt, bends: number, offCentre: number) => {
    let hits = 0;
    for (const o of obstacles) if (polylineHits(pts, inflate(o, 4))) hits++;
    let along = 0;
    let crossings = 0;
    for (let i = 1; i < pts.length; i++) {
      for (const [q1, q2] of lineAvoid) {
        if (collinearOverlap(pts[i - 1], pts[i], q1, q2, 12)) along++; // lanes stay at least 12px apart
        else if (crosses(pts[i - 1], pts[i], q1, q2)) crossings++;
      }
      for (const [q1, q2] of borderSegs) if (collinearOverlap(pts[i - 1], pts[i], q1, q2, 8)) along++;
    }
    // The label goes at the spot along the path (by length) that hits the least.
    let labelHits = 0;
    let at = 0.5;
    if (label) {
      labelHits = Infinity;
      for (const t of LABEL_SPOTS) {
        const lb = labelBox(pts, label, t);
        let h = 0;
        for (const o of [...obstacles, a, b, ...labelAvoid]) if (boxesOverlap(lb, o, 2)) h++;
        for (const [p, q] of lineAvoid) if (segmentHitsBox(p, q, lb)) h++;
        h += Math.abs(t - 0.5) * 0.5; // prefer the middle when it's equally free
        if (h < labelHits) { labelHits = h; at = t; }
      }
    }
    let throughLabels = 0;
    for (const l of labelAvoid) if (polylineHits(pts, l)) throughLabels++;
    const score = hits * 100000 + along * 30000 + labelHits * 20000 + throughLabels * 20000 + crossings * 300 + polylineLength(pts) + bends * 40 + offCentre * 30;
    if (!best || score < best.score) best = { pts, fa, fb, score, at };
  };

  const search = (ratios: number[], wide: boolean) => {
    for (const sa of sides) {
      for (const sb of sides) {
        for (const ra of ratios) {
          for (const rb of ratios) {
            const A = sideAt(a, sa, ra), B = sideAt(b, sb, rb);
            const p = A.pt, q = B.pt;
            const off = Math.abs(ra - 0.5) + Math.abs(rb - 0.5);
            const ortho = orthogonal(p, sa, q, sb);
            if (sane(ortho, sa, sb)) consider(ortho, A.fp, B.fp, ortho.length - 2, off);
            if (straight && sane([p, q], sa, sb)) consider([p, q], A.fp, B.fp, 0, off);
            if (sa === sb) for (const d of wide ? [30, 60, 90, 130, 180, 240] : [40, 90, 160]) consider(around(p, q, sa, a, b, d), A.fp, B.fp, 2, off);
            // Channel routes: step out, run along a lane of their own, step back in.
            if (wide && ra === 0.5 && rb === 0.5 && horizontal(sa) === horizontal(sb)) {
              const stub = 22;
              const [ox, oy] = outward(sa), [ix, iy] = outward(sb);
              const p1: Pt = [p[0] + ox * stub, p[1] + oy * stub], q1: Pt = [q[0] + ix * stub, q[1] + iy * stub];
              const mid = horizontal(sa) ? (p[1] + q[1]) / 2 : (p[0] + q[0]) / 2;
              for (let k = -10; k <= 10; k++) {
                const lane = mid + k * 16;
                const c: Pt[] = horizontal(sa)
                  ? [p, p1, [p1[0], lane], [q1[0], lane], q1, q]
                  : [p, p1, [lane, p1[1]], [lane, q1[1]], q1, q];
                if (sane(c, sa, sb)) consider(c, A.fp, B.fp, 4, off);
              }
            }
            if (horizontal(sa) === horizontal(sb) && sa !== sb) {
              for (const f of wide ? [0.1, 0.2, 0.3, 0.4, 0.6, 0.7, 0.8, 0.9] : [0.25, 0.75]) {
                const z = horizontal(sa)
                  ? [p, [p[0] + (q[0] - p[0]) * f, p[1]], [p[0] + (q[0] - p[0]) * f, q[1]], q] as Pt[]
                  : [p, [p[0], p[1] + (q[1] - p[1]) * f], [q[0], p[1] + (q[1] - p[1]) * f], q] as Pt[];
                if (sane(z, sa, sb)) consider(z, A.fp, B.fp, 2, off);
              }
            }
          }
        }
      }
    }
  };
  search([0.5], false);
  // Still colliding with something? Look wider.
  if (best && (best as { score: number }).score >= 300) search([0.5, 0.3, 0.7, 0.15, 0.85], true);

  if (!best) {
    const A = sideAt(a, "right", 0.5), B = sideAt(b, "left", 0.5);
    best = { pts: [A.pt, B.pt], fa: A.fp, fb: B.fp, score: 0, at: 0.5 };
  }
  const { pts, fa, fb, at } = best as { pts: Pt[]; fa: Pt; fb: Pt; at: number };
  const [x0, y0] = pts[0];
  return { x: x0, y: y0, points: pts.map(([x, y]) => [x - x0, y - y0] as Pt), fromPoint: fa, toPoint: fb, labelAt: at };
}

const LABEL_SPOTS = [0.5, 0.35, 0.65, 0.2, 0.8];

/**
 * The point `t` (0–1) of the way along a path by length. This is where
 * Excalidraw centres an arrow's label (its labelPosition; skeleton labels
 * default to 0.5 — half the path's length, not its middle point).
 */
export function pointAt(pts: Pt[], t = 0.5): Pt {
  const total = polylineLength(pts);
  let target = Math.max(0, Math.min(1, t)) * total;
  for (let i = 1; i < pts.length; i++) {
    const l = Math.hypot(pts[i][0] - pts[i - 1][0], pts[i][1] - pts[i - 1][1]);
    if (target <= l || i === pts.length - 1) {
      const f = l ? Math.min(1, target / l) : 0;
      return [pts[i - 1][0] + (pts[i][0] - pts[i - 1][0]) * f, pts[i - 1][1] + (pts[i][1] - pts[i - 1][1]) * f];
    }
    target -= l;
  }
  return pts[0];
}

/** The box a label of this size covers on a route (scene coordinates). */
export function labelBox(pts: Pt[], size: { w: number; h: number }, t = 0.5): Box {
  const [x, y] = pointAt(pts, t);
  return { x: x - size.w / 2, y: y - size.h / 2, w: size.w, h: size.h };
}

/** Puts each arrow's bound label at the spot its route chose. */
export function placeLabels(els: readonly { id: string; type: string }[], at: Map<string, { t: number; pts: Pt[] }>) {
  for (const el of els as any[]) {
    if (el.type !== "text" || !el.containerId) continue;
    const r = at.get(el.containerId);
    if (!r) continue;
    const [x, y] = pointAt(r.pts, r.t);
    el.labelPosition = r.t;
    el.x = x - el.width / 2;
    el.y = y - el.height / 2;
  }
}

/** Rough size of a label: hand-written text is about 11px a character and wraps near 240px. */
export function estimateLabel(text: string, perChar = 11, wrap = 240, lineHeight = 26): { w: number; h: number } {
  const raw = text.length * perChar;
  const lines = Math.max(1, Math.ceil(raw / wrap));
  return { w: Math.min(raw, wrap) + 8, h: lines * lineHeight + 4 };
}

/** Segments of existing arrows, in scene coordinates (things new shapes should avoid). */
export function arrowSegments(elements: readonly { type: string; isDeleted?: boolean; x: number; y: number; points?: readonly (readonly number[])[] }[]): [Pt, Pt][] {
  const out: [Pt, Pt][] = [];
  for (const e of elements) {
    if (e.isDeleted || e.type !== "arrow" || !e.points) continue;
    for (let i = 1; i < e.points.length; i++) {
      out.push([[e.x + e.points[i - 1][0], e.y + e.points[i - 1][1]], [e.x + e.points[i][0], e.y + e.points[i][1]]]);
    }
  }
  return out;
}

/**
 * Second pass: with every arrow's path known, put each label at the spot
 * along its own path that is clear of cards, other labels and every line.
 * Short arrows go first (they have the fewest spots). Returns labelAt per id.
 */
export function placeLabelsAvoiding(
  routes: { id: string; pts: Pt[]; size: { w: number; h: number }; alt?: { w: number; h: number }[] }[],
  cards: Box[],
  reserved: Box[] = [],
): Map<string, { t: number; variant: number }> {
  const lines = routes.map((r) => ({ id: r.id, segs: r.pts.slice(1).map((q, i) => [r.pts[i], q] as [Pt, Pt]) }));
  const placed: Box[] = [...reserved];
  const out = new Map<string, { t: number; variant: number }>();
  const spots: number[] = [];
  for (let k = 0; k <= 16; k++) spots.push(0.1 + k * 0.05);
  spots.sort((x, y) => Math.abs(x - 0.5) - Math.abs(y - 0.5));
  for (const r of [...routes].sort((x, y) => polylineLength(x.pts) - polylineLength(y.pts))) {
    // The label as wrapped first; if no spot is clear of cards and labels, a narrower wrapping.
    let best = Infinity;
    let bestT = 0.5;
    let bestV = 0;
    for (const [v, size] of [r.size, ...(r.alt ?? [])].entries()) {
      for (const t of spots) {
        const lb = labelBox(r.pts, size, t);
        let c = v * 5; // prefer the usual wrapping when it's just as clear
        for (const b of cards) if (boxesOverlap(lb, b, 2)) c += 1000;
        for (const b of placed) if (boxesOverlap(lb, b, 2)) c += 1000;
        for (const l of lines) if (l.id !== r.id) for (const [p, q] of l.segs) if (segmentHitsBox(p, q, lb)) c += 100;
        c += Math.abs(t - 0.5);
        if (c < best) { best = c; bestT = t; bestV = v; }
        if (c < 1 + v * 5) break; // free spot nearest the middle
      }
      if (best < 100) break; // clear of cards and labels: good enough
    }
    out.set(r.id, { t: bestT, variant: bestV });
    placed.push(labelBox(r.pts, [r.size, ...(r.alt ?? [])][bestV], bestT));
  }
  return out;
}

/**
 * Wraps a label onto lines of at most `max` characters, breaking at " · "
 * first, then at spaces. Excalidraw wraps an arrow's label only at 70% of the
 * arrow's width, so long arrows get very wide labels; wrapping them ourselves
 * keeps labels narrow, so the gaps between cards can be too.
 */
export function wrapLabel(text: string, max = 20): string {
  const parts = text.split(" · ");
  const lines: string[] = [];
  let cur = "";
  const push = (w: string, sep: string) => {
    if (!cur) cur = w;
    else if ((cur + sep + w).length <= max) cur += sep + w;
    else { lines.push(cur); cur = w; }
  };
  for (const [i, part] of parts.entries()) {
    const words = part.split(" ");
    if (part.length <= max) push(part, i ? " · " : "");
    else for (const [j, w] of words.entries()) push(w, j === 0 && i ? " · " : " ");
  }
  if (cur) lines.push(cur);
  return lines.join("\n");
}

/** Size of a (possibly multi-line) label in the Production font. */
export function labelSize(wrapped: string): { w: number; h: number } {
  const lines = wrapped.split("\n");
  return { w: Math.max(...lines.map((l) => l.length)) * 5.8 + 8, h: lines.length * 15 + 4 };
}
