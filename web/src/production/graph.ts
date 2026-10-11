// Reads a Production Diagram's meaning from the live scene (mirrors
// internal/scene for production; the Go side stays the source of truth for
// the AI and for validation).
import type { ExcalidrawElement } from "@excalidraw/excalidraw/element/types";
import type { GEdge, GGroup, GNode, Graph } from "../types";
import { metaOf } from "./render";

export function graphFromScene(elements: readonly ExcalidrawElement[]): Graph {
  const live = elements.filter((e) => !e.isDeleted);
  const text = new Map<string, string>();
  for (const el of live) {
    const m = metaOf(el);
    if (el.type === "text" && m?.role === "label" && m.node) text.set(m.node, ((el as any).originalText || (el as any).text || "").trim());
  }
  const labelOf = (id: string, stored?: string) => {
    const t = text.get(id);
    if (t === undefined) return stored ?? "";
    if (t.endsWith("…") && stored?.startsWith(t.slice(0, -1))) return stored;
    if (t === "(unnamed)" && !stored) return "";
    return t;
  };
  const nodes: GNode[] = [];
  const groups: GGroup[] = [];
  const edges: GEdge[] = [];
  for (const el of live) {
    const m = metaOf(el);
    if (!m) continue;
    if (m.role === "node") {
      nodes.push({ id: el.id, label: labelOf(el.id, m.label), kind: m.kind, service: m.service, serviceName: m.serviceName, classes: m.classes, attrs: m.attrs, parent: m.parent });
    } else if (m.role === "group") {
      groups.push({ id: el.id, label: labelOf(el.id, m.label), kind: m.kind, tier: m.tier, parent: m.parent });
    }
  }
  const ids = new Set([...nodes.map((n) => n.id), ...groups.map((g) => g.id)]);
  for (const el of live) {
    const m = metaOf(el);
    if (el.type !== "arrow" || m?.role !== "edge") continue;
    const from = (el as any).startBinding?.elementId as string | undefined;
    const to = (el as any).endBinding?.elementId as string | undefined;
    if (from && to && ids.has(from) && ids.has(to)) edges.push({ id: el.id, from, to, style: m.style, protocol: m.protocol, label: m.label });
  }
  return { production: true, nodes, groups, edges, notes: null, issues: null };
}

/** The smallest Group whose drawn box contains a box. */
export function drawnParent(groups: { id: string; x: number; y: number; w: number; h: number }[], b: { x: number; y: number; w: number; h: number }, self?: string): string | undefined {
  let best: string | undefined;
  let area = Infinity;
  for (const g of groups) {
    if (g.id === self) continue;
    if (b.x >= g.x && b.y >= g.y && b.x + b.w <= g.x + g.w && b.y + b.h <= g.y + g.h && g.w * g.h < area) {
      best = g.id;
      area = g.w * g.h;
    }
  }
  return best;
}

/** Designer elements whose owner is gone (e.g. the user deleted only part of a card). */
export function hasOrphans(elements: readonly ExcalidrawElement[]): boolean {
  const live = elements.filter((e) => !e.isDeleted);
  const owners = new Set(live.filter((e) => ["node", "group"].includes(metaOf(e)?.role ?? "")).map((e) => e.id));
  for (const el of live) {
    const m = metaOf(el);
    if (!m) continue;
    if ((m.role === "label" || m.role === "decor" || m.role === "icon") && m.node && !owners.has(m.node)) return true;
    if (m.role === "edge") {
      const a = (el as any).startBinding?.elementId;
      const b = (el as any).endBinding?.elementId;
      if (!a || !b || !owners.has(a) || !owners.has(b)) return true;
    }
    if ((m.role === "node" || m.role === "group") && m.parent && !owners.has(m.parent)) return true;
  }
  return false;
}
