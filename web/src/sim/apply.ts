// Scenario → Proposal (sim ticket 009). Sizing becomes readable attributes on the
// Node (instances, autoscaling, instance), a self-hosted service gets
// simulate_as, and an added cache becomes a new Node with an Edge from its
// caller. Edge settings (calls per request, timeouts) stay in the Scenario.
import type { Change, Graph } from "../types.ts";
import { sizingAttrs, sizingFromAttrs, type Resolved, type Scenario } from "./resolve.ts";

export type Applied = { changes: Change[]; summary: string };

export function scenarioChanges(g: Graph, sc: Scenario, r: Resolved): Applied {
  const changes: Change[] = [];
  const lines: string[] = [];
  const byId = new Map(g.nodes.map((n) => [n.id, n]));

  for (const [id, ov] of Object.entries(sc.sizing)) {
    const n = byId.get(id);
    const s = r.sizing[id];
    if (!n || !s) continue;
    const now = sizingFromAttrs(n.attrs);
    const set: Record<string, unknown> = {};
    // Only a size the user chose, or one the diagram already names: not a Profile default.
    const want = sizingAttrs({ count: s.count, size: ov.size || now.size ? s.size : undefined, scale: s.scale });
    const sameCount = now.count === s.count;
    const sameSize = (now.size ?? s.size) === s.size;
    const sameScale = JSON.stringify(now.scale ?? null) === JSON.stringify(s.scale ?? null);
    if ((ov.count != null || ov.scale !== undefined || ov.size) && !(sameCount && sameSize && sameScale)) {
      Object.assign(set, want);
      if (!s.scale && n.attrs?.autoscaling) set.autoscaling = null;
      // The old free-text key (e.g. tasks) is replaced by the normalised ones.
      for (const k of now.keys) if (!(k in want)) set[k] = null;
      lines.push(`${n.label}: ${s.count}${s.scale ? ` (auto ${s.scale.min}-${s.scale.max})` : ""}${want.instance ? ` × ${want.instance}` : ""}`);
    }
    if (ov.simAs && n.attrs?.simulate_as !== ov.simAs) {
      set.simulate_as = ov.simAs;
      lines.push(`${n.label} simulated as ${ov.simAs}`);
    }
    if (Object.keys(set).length) changes.push({ id: `c${changes.length + 1}`, op: "update_node", ref: id, set, why: whyFrom(sc) });
  }

  for (const a of sc.added) {
    const to = byId.get(a.to);
    changes.push({
      id: `c${changes.length + 1}`, op: "add_node", ref: a.ref, label: a.label, kind: "data.cache", service: r.cacheService,
      parent: to?.parent ?? null, set: { hit_ratio: `${Math.round(a.hit * 100)}%` }, why: whyFrom(sc),
    } as Change);
    changes.push({ id: `c${changes.length + 1}`, op: "add_edge", from: a.from, to: a.ref, style: "sync", protocol: "TCP" });
    lines.push(`add ${a.label}`);
  }
  return { changes, summary: lines.join("; ") };
}

function whyFrom(sc: Scenario) {
  const b = sc.baseline, l = sc.last;
  if (!b || !l) return `Tested in Sim Mode (Scenario ${sc.name}).`;
  const br = (x: number | null) => (x == null ? "no break" : `${x} rps`);
  return `Sim Mode, Scenario ${sc.name}: breaking point ${br(b.breakRps)} → ${br(l.breakRps)}; p99 at ${l.trafficRps} rps ${Math.round(b.p99)} → ${Math.round(l.p99)} ms.`;
}
