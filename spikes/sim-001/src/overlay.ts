// Canvas 2D layer over Excalidraw: Node heat, instance badges and dots moving
// along Edges. Draws in scene coordinates with the same transform Excalidraw
// uses (screen = (scene + scroll) × zoom), so it stays aligned at any view.
// Dots are visual only: their count follows flow on a log scale and is capped
// per Edge, so drawing cost depends on what's on screen, not on traffic.
import { FN } from "./engine.ts";
import type { NodeSpec } from "./system.ts";

export type View = { scrollX: number; scrollY: number; zoom: number };
type Path = { pts: Float32Array; cum: Float32Array; len: number; minX: number; minY: number; maxX: number; maxY: number };

const MAX_DOTS_PER_EDGE = 12;
const DOT_SPEED = 0.35; // path lengths per second… scaled down for long paths below

export function pathFromArrow(el: { x: number; y: number; points: readonly (readonly [number, number])[] }): Path {
  const n = el.points.length;
  const pts = new Float32Array(n * 2);
  const cum = new Float32Array(n);
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  for (let i = 0; i < n; i++) {
    const x = el.x + el.points[i][0];
    const y = el.y + el.points[i][1];
    pts[i * 2] = x;
    pts[i * 2 + 1] = y;
    if (i) cum[i] = cum[i - 1] + Math.hypot(x - pts[i * 2 - 2], y - pts[i * 2 - 1]);
    minX = Math.min(minX, x); minY = Math.min(minY, y); maxX = Math.max(maxX, x); maxY = Math.max(maxY, y);
  }
  return { pts, cum, len: cum[n - 1] || 1, minX, minY, maxX, maxY };
}

const heat = (u: number) => (u < 0.6 ? "#2f9e44" : u < 0.85 ? "#f08c00" : "#e03131");

export class Overlay {
  canvas: HTMLCanvasElement;
  ctx: CanvasRenderingContext2D;
  view: View = { scrollX: 0, scrollY: 0, zoom: 1 };
  nodes: NodeSpec[];
  paths: (Path | null)[];
  frame: { nodes: Float32Array; edges: Float32Array } | null = null;
  selected = -1;
  dotsDrawn = 0;
  private dotX = new Float32Array(0);
  private dotY = new Float32Array(0);

  constructor(canvas: HTMLCanvasElement, nodes: NodeSpec[], paths: (Path | null)[]) {
    this.canvas = canvas;
    this.ctx = canvas.getContext("2d")!;
    this.nodes = nodes;
    this.paths = paths;
    const cap = paths.length * MAX_DOTS_PER_EDGE;
    this.dotX = new Float32Array(cap);
    this.dotY = new Float32Array(cap);
  }

  /** Scene → canvas CSS pixels, for tests. */
  toScreen(x: number, y: number) {
    const v = this.view;
    return { x: (x + v.scrollX) * v.zoom, y: (y + v.scrollY) * v.zoom };
  }

  draw(nowMs: number) {
    const { canvas, ctx, view } = this;
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth, h = canvas.clientHeight;
    if (canvas.width !== Math.round(w * dpr) || canvas.height !== Math.round(h * dpr)) {
      canvas.width = Math.round(w * dpr);
      canvas.height = Math.round(h * dpr);
    }
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    const f = this.frame;
    if (!f) return;
    const z = view.zoom;
    ctx.setTransform(dpr * z, 0, 0, dpr * z, dpr * z * view.scrollX, dpr * z * view.scrollY);
    const x0 = -view.scrollX, y0 = -view.scrollY, x1 = x0 + w / z, y1 = y0 + h / z;
    const px = 1 / z; // one CSS pixel in scene units

    // Node heat and badges.
    ctx.lineWidth = 3 * px;
    const showText = z > 0.45;
    if (showText) {
      ctx.font = `${11 * px}px system-ui, sans-serif`;
      ctx.textBaseline = "top";
    }
    for (let n = 0; n < this.nodes.length; n++) {
      const nd = this.nodes[n];
      if (nd.x > x1 || nd.y > y1 || nd.x + nd.w < x0 || nd.y + nd.h < y0) continue;
      const o = n * FN;
      const util = f.nodes[o], down = f.nodes[o + 5], err = f.nodes[o + 2];
      const col = down ? "#868e96" : heat(util);
      // No fill over the Node: it would blur the label. Outline colour + a utilisation bar below.
      if (down) {
        ctx.globalAlpha = 0.45;
        ctx.fillStyle = col;
        ctx.fillRect(nd.x, nd.y, nd.w, nd.h);
        ctx.globalAlpha = 1;
      }
      ctx.fillStyle = "#e9ecef";
      ctx.fillRect(nd.x, nd.y + nd.h + 4 * px, nd.w, 5 * px);
      ctx.fillStyle = col;
      ctx.fillRect(nd.x, nd.y + nd.h + 4 * px, nd.w * Math.min(1, util), 5 * px);
      ctx.strokeStyle = n === this.selected ? "#1c7ed6" : col;
      ctx.strokeRect(nd.x - 2 * px, nd.y - 2 * px, nd.w + 4 * px, nd.h + 4 * px);
      if (showText) {
        const inst = f.nodes[o + 3], warm = f.nodes[o + 4];
        const badge = `×${inst}${warm ? ` +${warm}` : ""} · ${Math.round(util * 100)}%${err > 0.005 ? ` · ${(err * 100).toFixed(err < 0.1 ? 1 : 0)}% err` : ""}`;
        const tw = ctx.measureText(badge).width;
        ctx.fillStyle = "rgba(255,255,255,0.9)";
        ctx.fillRect(nd.x + nd.w - tw - 8 * px, nd.y - 9 * px, tw + 6 * px, 15 * px);
        ctx.fillStyle = err > 0.01 || down ? "#c92a2a" : "#212529";
        ctx.fillText(badge, nd.x + nd.w - tw - 5 * px, nd.y - 7 * px);
      }
    }

    // Dots: positions first, then one fill pass per colour.
    let ok = 0;
    let bad = this.dotX.length;
    const t = nowMs / 1000;
    for (let e = 0; e < this.paths.length; e++) {
      const p = this.paths[e];
      const flow = f.edges[e * 2];
      if (!p || flow <= 0.01) continue;
      if (p.minX > x1 || p.minY > y1 || p.maxX < x0 || p.maxY < y0) continue;
      const count = Math.min(MAX_DOTS_PER_EDGE, Math.max(1, Math.round(Math.log10(flow + 1) * 3)));
      const errShare = f.edges[e * 2 + 1];
      const speed = (DOT_SPEED * 300) / Math.max(300, p.len);
      for (let i = 0; i < count; i++) {
        const u = (t * speed + i / count + e * 0.137) % 1;
        const d = u * p.len;
        let k = 1;
        while (k < p.cum.length - 1 && p.cum[k] < d) k++;
        const seg = p.cum[k] - p.cum[k - 1] || 1;
        const a = (d - p.cum[k - 1]) / seg;
        const x = p.pts[(k - 1) * 2] + (p.pts[k * 2] - p.pts[(k - 1) * 2]) * a;
        const y = p.pts[(k - 1) * 2 + 1] + (p.pts[k * 2 + 1] - p.pts[(k - 1) * 2 + 1]) * a;
        if (i / count < errShare) {
          bad--;
          this.dotX[bad] = x;
          this.dotY[bad] = y;
        } else {
          this.dotX[ok] = x;
          this.dotY[ok] = y;
          ok++;
        }
      }
    }
    const s = 5 * px, hs = s / 2;
    ctx.fillStyle = "#1971c2";
    for (let i = 0; i < ok; i++) ctx.fillRect(this.dotX[i] - hs, this.dotY[i] - hs, s, s);
    ctx.fillStyle = "#e03131";
    for (let i = bad; i < this.dotX.length; i++) ctx.fillRect(this.dotX[i] - hs, this.dotY[i] - hs, s, s);
    this.dotsDrawn = ok + (this.dotX.length - bad);
  }
}
