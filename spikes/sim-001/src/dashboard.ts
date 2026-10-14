// Monitoring dashboard charts (uPlot, lazy-loaded with Sim Mode).
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import { NM, RM, SERIES_LEN } from "./engine.ts";

export type ChartSpec =
  | { kind: "latency"; title: string; route: number; sloMs: number }
  | { kind: "traffic"; title: string; route: number }
  | { kind: "node"; title: string; node: number };

export type Marker = { t: number; label: string };

const nul = (a: Float32Array, off: number, scale = 1) => {
  const out: (number | null)[] = new Array(SERIES_LEN);
  for (let i = 0; i < SERIES_LEN; i++) {
    const v = a[off + i];
    out[i] = Number.isFinite(v) ? v * scale : null;
  }
  return out;
};

function markersPlugin(get: () => Marker[]): uPlot.Plugin {
  return {
    hooks: {
      draw: (u) => {
        const { ctx } = u;
        const { top, height } = u.bbox;
        ctx.save();
        ctx.strokeStyle = "#7048e8";
        ctx.setLineDash([4, 3]);
        ctx.lineWidth = 1;
        for (const m of get()) {
          const x = Math.round(u.valToPos(m.t, "x", true));
          if (x < u.bbox.left || x > u.bbox.left + u.bbox.width) continue;
          ctx.beginPath();
          ctx.moveTo(x, top);
          ctx.lineTo(x, top + height);
          ctx.stroke();
        }
        ctx.restore();
      },
    },
  };
}

export function createDashboard(host: HTMLElement, specs: ChartSpec[], markers: () => Marker[], width = 300, height = 110) {
  const plots: uPlot[] = [];
  const axisFont = "10px system-ui";
  const x = { stroke: "#868e96", font: axisFont, size: 22, values: (_u: uPlot, v: number[]) => v.map((s) => `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, "0")}`) };
  for (const s of specs) {
    const el = document.createElement("div");
    el.className = "chart";
    host.appendChild(el);
    let series: uPlot.Series[];
    let axes: uPlot.Axis[];
    if (s.kind === "latency") {
      series = [{}, { label: "p50", stroke: "#74c0fc", width: 1.5 }, { label: "p95", stroke: "#1c7ed6", width: 1.5 }, { label: "p99", stroke: "#0b4f8a", width: 2 }, { label: "SLO", stroke: "#e03131", dash: [6, 4], width: 1, points: { show: false } }];
      axes = [x, { stroke: "#868e96", font: axisFont, size: 40, values: (_u, v) => v.map((ms) => (ms >= 1000 ? `${ms / 1000}s` : `${ms}ms`)) }];
    } else if (s.kind === "traffic") {
      series = [{}, { label: "rps", stroke: "#2f9e44", width: 1.5, scale: "rps" }, { label: "err %", stroke: "#e03131", width: 1.5, scale: "err" }];
      axes = [x, { scale: "rps", stroke: "#868e96", font: axisFont, size: 40 }, { scale: "err", side: 1, stroke: "#e03131", font: axisFont, size: 30, grid: { show: false } }];
    } else {
      series = [{}, { label: "util %", stroke: "#f08c00", width: 1.5, scale: "util" }, { label: "p99 ms", stroke: "#1c7ed6", width: 1.5, scale: "ms" }];
      axes = [x, { scale: "util", stroke: "#868e96", font: axisFont, size: 32 }, { scale: "ms", side: 1, stroke: "#1c7ed6", font: axisFont, size: 40, grid: { show: false } }];
    }
    const empty = series.map(() => new Array(SERIES_LEN).fill(null));
    plots.push(
      new uPlot(
        {
          title: s.title,
          width,
          height,
          series,
          axes,
          legend: { show: true, live: false },
          cursor: { show: false },
          scales: { x: { time: false }, util: { range: [0, 100] }, err: { range: [0, 100] } },
          plugins: [markersPlugin(markers)],
        },
        empty as any,
        el,
      ),
    );
  }

  return {
    /** Applies a "series" message from the worker. */
    update(msg: { seconds: number; subNodes: number[]; subRoutes: number[]; nodes: Float32Array; routes: Float32Array }) {
      const xs = new Array(SERIES_LEN);
      for (let i = 0; i < SERIES_LEN; i++) xs[i] = msg.seconds - SERIES_LEN + i;
      specs.forEach((s, i) => {
        let data: any[];
        if (s.kind === "node") {
          const k = msg.subNodes.indexOf(s.node);
          if (k < 0) return;
          const b = k * NM * SERIES_LEN;
          data = [xs, nul(msg.nodes, b + 2 * SERIES_LEN, 100), nul(msg.nodes, b + 7 * SERIES_LEN)];
        } else {
          const k = msg.subRoutes.indexOf(s.route);
          if (k < 0) return;
          const b = k * RM * SERIES_LEN;
          data =
            s.kind === "latency"
              ? [xs, nul(msg.routes, b + 2 * SERIES_LEN), nul(msg.routes, b + 3 * SERIES_LEN), nul(msg.routes, b + 4 * SERIES_LEN), new Array(SERIES_LEN).fill(s.sloMs)]
              : [xs, nul(msg.routes, b), nul(msg.routes, b + SERIES_LEN, 100)];
        }
        plots[i].setData(data);
      });
    },
    destroy() {
      plots.forEach((p) => p.destroy());
    },
  };
}
