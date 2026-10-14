// The Component Library model: library YAML + Cloud Catalog → searchable entries.
// Provider-neutral: nothing here knows AWS or Azure beyond the files it is given.
import { parse } from "yaml";
import awsLib from "../../../internal/catalog/library/aws.yaml?raw";
import azureLib from "../../../internal/catalog/library/azure.yaml?raw";
import awsCat from "../../../internal/catalog/data/aws.yaml?raw";
import azureCat from "../../../internal/catalog/data/azure.yaml?raw";
import kindsRaw from "../../../internal/catalog/data/kinds.yaml?raw";

export type Sim = "full" | "hop" | "off-path";
export type Entry = {
  id: string;
  group: string;
  label: string; // what the card shows
  name: string; // full official name (always shown beside an official icon)
  kind: string;
  service?: string;
  icon: string | null;
  genericIcon: string;
  gloss: string;
  place: "node" | "group" | "source";
  groupKind?: string;
  sim: Sim;
  hipaa?: boolean;
  more?: boolean; // catalog Service without a curated component
  hay: { text: string; w: number }[];
};
export type Group = { id: string; title: string; role: string; icon?: string | null };
export type Quick = { id: string; title: string; component?: string; onto: string; rule: string };
export type Library = { provider: string; display: string; iconBase: string | null; groups: Group[]; quick: Quick[]; entries: Entry[] };

const kinds = parse(kindsRaw).kinds as { id: string; gloss: string; icon: string }[];
const gloss = (k: string) => kinds.find((x) => x.id === k)?.gloss ?? "";
const kindIcon = (k: string) => kinds.find((x) => x.id === k)?.icon ?? "box";

function load(libRaw: string, catRaw: string): Library {
  const lib = parse(libRaw);
  const cat = parse(catRaw);
  const svc = new Map<string, any>(cat.services.map((s: any) => [s.id, s]));
  const groups: Group[] = lib.groups;
  const entries: Entry[] = [];
  const used = new Set<string>();
  const groupTitle = (g: string) => groups.find((x) => x.id === g)?.title ?? g;
  for (const c of lib.components) {
    const s = c.service ? svc.get(c.service) : undefined;
    if (c.service) used.add(c.service);
    const kind = c.kind ?? s?.kind ?? "compute.container";
    const label = c.label ?? s?.name ?? c.id;
    entries.push({
      id: c.id, group: c.group, label, name: s?.name ?? label, kind, service: c.service, icon: c.icon ?? null,
      genericIcon: kindIcon(kind), gloss: gloss(kind), place: c.place ?? "node", groupKind: c.group_kind, sim: c.sim ?? (c.place === "group" || c.place === "source" ? "full" : "full"),
      hipaa: s?.hipaa,
      hay: [
        { text: label, w: 3 }, { text: s?.name ?? "", w: 3 },
        ...[...(c.keywords ?? []), ...(s?.aliases ?? [])].map((t: string) => ({ text: t, w: 2 })),
        { text: c.service ?? "", w: 2 }, { text: groupTitle(c.group), w: 1.5 }, { text: gloss(kind), w: 1 },
      ],
    });
  }
  // Every other catalog Service stays findable, under "More".
  for (const s of cat.services) {
    if (used.has(s.id)) continue;
    entries.push({
      id: s.id, group: "more", label: s.name, name: s.name, kind: s.kind, service: s.id, icon: null, genericIcon: kindIcon(s.kind), gloss: gloss(s.kind),
      place: "node", sim: "off-path", hipaa: s.hipaa, more: true,
      hay: [{ text: s.name, w: 3 }, ...(s.aliases ?? []).map((t: string) => ({ text: t, w: 2 })), { text: s.id, w: 2 }, { text: gloss(s.kind), w: 1 }],
    });
  }
  return {
    provider: lib.provider, display: cat.display, iconBase: lib.icons ? `/icons/${lib.icons}/` : null,
    groups: [...groups, { id: "more", title: "More services", role: "more" }], quick: lib.quick ?? [], entries,
  };
}

export const LIBRARIES: Record<string, Library> = { aws: load(awsLib, awsCat), azure: load(azureLib, azureCat) };

const words = (s: string) => s.toLowerCase().split(/[^a-z0-9]+/).filter(Boolean);

/**
 * Ranked search. Every query word must match some field (AND). A field scores by how the word
 * matches it — whole word 10, word prefix 6, substring 3 — times the field's weight. Ties go to
 * simulated components, then library order.
 */
export function search(lib: Library, q: string): { entry: Entry; score: number }[] {
  const qs = words(q);
  if (!qs.length) return [];
  const out: { entry: Entry; score: number; i: number }[] = [];
  lib.entries.forEach((e, i) => {
    let total = 0;
    for (const w of qs) {
      let best = 0;
      for (const f of e.hay) {
        if (!f.text) continue;
        const ws = words(f.text);
        const s = ws.includes(w) ? 10 : ws.some((x) => x.startsWith(w)) ? 6 : f.text.toLowerCase().includes(w) ? 3 : 0;
        best = Math.max(best, s * f.w);
      }
      if (!best) return;
      total += best;
    }
    if (e.more) total *= 0.6;
    if (e.sim === "full") total += 1;
    out.push({ entry: e, score: total, i });
  });
  return out.sort((a, b) => b.score - a.score || a.i - b.i);
}
