import { useMemo, useState } from "react";
import type { Catalog, GEdge, GGroup, GNode, Graph } from "../types";

export type NodePatch = Partial<Pick<GNode, "label" | "kind" | "service" | "serviceName" | "classes" | "attrs">>;

function KindSelect({ cat, value, onChange, allowNone }: { cat: Catalog; value?: string; onChange: (v: string) => void; allowNone?: boolean }) {
  return (
    <select value={value ?? ""} onChange={(e) => onChange(e.target.value)}>
      {allowNone && <option value="">— untyped —</option>}
      {cat.categories.map((c) => (
        <optgroup key={c} label={c}>
          {cat.kinds.filter((k) => k.id.startsWith(c + ".")).map((k) => <option key={k.id} value={k.id}>{k.id.split(".")[1]} — {k.gloss}</option>)}
        </optgroup>
      ))}
    </select>
  );
}

function ServiceSelect({ cat, cloud, kind, value, onChange }: { cat: Catalog; cloud: string; kind?: string; value?: string; onChange: (v: string) => void }) {
  const services = cat.clouds[cloud]?.services.filter((s) => s.kind === kind) ?? [];
  return (
    <select value={value ?? ""} onChange={(e) => onChange(e.target.value)}>
      <option value="">— none / off-catalog —</option>
      {services.map((s) => <option key={s.id} value={s.id}>{s.name}{s.hipaa ? "" : " (not HIPAA-eligible)"}</option>)}
    </select>
  );
}

export function Inspector({ g, cat, cloud, selected, view, locked, onNode, onGroup, onEdge, onAddNode, onTidy }: {
  g: Graph | null;
  cat: Catalog;
  cloud: string;
  selected: string | null;
  view: "rough" | "production";
  locked: boolean;
  onNode: (id: string, p: NodePatch) => void;
  onGroup: (id: string, p: Partial<GGroup>) => void;
  onEdge: (id: string, p: Partial<GEdge>) => void;
  onAddNode: (kind: string, service?: string) => void;
  onTidy?: (scope?: string) => void;
}) {
  const node = g?.nodes.find((n) => n.id === selected);
  const group = g?.groups.find((x) => x.id === selected);
  const edge = g?.edges.find((x) => x.id === selected);
  const [newKind, setNewKind] = useState("compute.container");
  const [newService, setNewService] = useState("");
  const kind = useMemo(() => cat.kinds.find((k) => k.id === node?.kind), [cat, node?.kind]);
  const [attrKey, setAttrKey] = useState("");

  if (locked) return <div className="panel"><div className="empty">The canvas is locked while a Proposal waits for review.</div></div>;

  return (
    <div className="panel">
      {view === "production" && (
        <div className="card boxed">
          <div className="eyebrow">Add a component</div>
          <div className="field" style={{ marginTop: 6 }}>
            <KindSelect cat={cat} value={newKind} onChange={(v) => { setNewKind(v); setNewService(""); }} />
          </div>
          {cloud !== "generic" && <div className="field"><ServiceSelect cat={cat} cloud={cloud} kind={newKind} value={newService} onChange={setNewService} /></div>}
          <div className="row">
            <button onClick={() => onAddNode(newKind, newService || undefined)}>Add{group ? ` to ${group.label || "group"}` : node?.parent ? " beside it" : ""}</button>
            <span className="spacer" />
            {onTidy && <button onClick={() => onTidy(group?.id)} title="Re-run automatic layout">Tidy {group ? "group" : "up"}</button>}
          </div>
        </div>
      )}

      {!node && !group && !edge && <div className="empty">Select a component, group or arrow to edit it.</div>}

      {node && (
        <>
          <h3>Component</h3>
          <div className="field"><label>Label</label><input type="text" value={node.label} onChange={(e) => onNode(node.id, { label: e.target.value })} /></div>
          <div className="field">
            <label>Component Kind{view === "rough" ? " (optional on the Rough Draft)" : ""}</label>
            <KindSelect cat={cat} value={node.kind} allowNone={view === "rough"} onChange={(v) => {
              const holds = cat.kinds.find((k) => k.id === v)?.holdsData;
              onNode(node.id, { kind: v || undefined, service: undefined, ...(holds ? {} : { classes: undefined }) });
            }} />
          </div>
          {view === "production" && cloud !== "generic" && (
            <>
              <div className="field"><label>Service</label><ServiceSelect cat={cat} cloud={cloud} kind={node.kind} value={node.service} onChange={(v) => onNode(node.id, { service: v || undefined, serviceName: undefined })} /></div>
              {!node.service && <div className="field"><label>Off-catalog service name</label><input type="text" value={node.serviceName ?? ""} placeholder="e.g. Stripe" onChange={(e) => onNode(node.id, { serviceName: e.target.value || undefined })} /></div>}
            </>
          )}
          {view === "production" && kind?.holdsData && (
            <div className="field">
              <label>Data Classification</label>
              <div className="row wrap">
                {cat.dataClasses.map((c) => (
                  <label key={c} className="check">
                    <input type="checkbox" checked={node.classes?.includes(c) ?? false} onChange={() => {
                      const cur = node.classes ?? [];
                      onNode(node.id, { classes: cur.includes(c) ? cur.filter((x) => x !== c) : [...cur, c] });
                    }} />
                    {c}
                  </label>
                ))}
              </div>
            </div>
          )}
          {view === "production" && (
            <div className="field">
              <label>Attributes</label>
              {Object.entries(node.attrs ?? {}).map(([k, v]) => (
                <div className="row small" key={k}>
                  <span>{k}{v !== "true" ? ` = ${v}` : ""}</span>
                  <span className="spacer" />
                  <button className="ghost" onClick={() => { const a = { ...node.attrs }; delete a[k]; onNode(node.id, { attrs: a }); }} aria-label={`Remove ${k}`}>✕</button>
                </div>
              ))}
              <div className="row">
                <input type="text" value={attrKey} placeholder="e.g. encrypted" onChange={(e) => setAttrKey(e.target.value)} style={{ flex: 1 }} />
                <button disabled={!attrKey.trim()} onClick={() => { const [k, v] = attrKey.split("="); onNode(node.id, { attrs: { ...node.attrs, [k.trim()]: (v ?? "true").trim() } }); setAttrKey(""); }}>Add</button>
              </div>
            </div>
          )}
        </>
      )}

      {group && (
        <>
          <h3>Group</h3>
          <div className="field"><label>Label</label><input type="text" value={group.label} onChange={(e) => onGroup(group.id, { label: e.target.value })} /></div>
          {view === "production" && (
            <>
              <div className="field">
                <label>Group kind</label>
                <select value={group.kind ?? ""} onChange={(e) => onGroup(group.id, { kind: e.target.value, tier: e.target.value === "subnet" ? group.tier ?? "private" : undefined })}>
                  {cat.groupKinds.map((k) => <option key={k.id} value={k.id}>{k.id} — {k.gloss}</option>)}
                </select>
              </div>
              {group.kind === "subnet" && (
                <div className="field">
                  <label>Tier</label>
                  <select value={group.tier ?? "private"} onChange={(e) => onGroup(group.id, { tier: e.target.value })}>
                    {cat.groupKinds.find((k) => k.id === "subnet")?.tiers?.map((t) => <option key={t}>{t}</option>)}
                  </select>
                </div>
              )}
            </>
          )}
        </>
      )}

      {edge && view === "production" && (
        <>
          <h3>Connection</h3>
          <div className="field">
            <label>Interaction</label>
            <select value={edge.style ?? "sync"} onChange={(e) => onEdge(edge.id, { style: e.target.value })}>
              <option value="sync">sync (solid)</option>
              <option value="async">async (dashed)</option>
              <option value="stream">stream (dotted)</option>
            </select>
          </div>
          <div className="field"><label>Protocol</label><input type="text" value={edge.protocol ?? ""} placeholder="e.g. HTTPS" onChange={(e) => onEdge(edge.id, { protocol: e.target.value || undefined })} /></div>
          <div className="field"><label>Label</label><input type="text" value={edge.label ?? ""} onChange={(e) => onEdge(edge.id, { label: e.target.value || undefined })} /></div>
        </>
      )}
    </div>
  );
}
