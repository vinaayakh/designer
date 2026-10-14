// Prototype: the Component Library panel (sim ticket 013) beside an Excalidraw canvas.
// Search, groups, quick inserts, click or drag to place. AWS shows official icons from the
// first-use download (public/icons/aws/); Azure falls back to generic Kind icons.
import React, { useEffect, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { Excalidraw, convertToExcalidrawElements, viewportCoordsToSceneCoords, CaptureUpdateAction } from "@excalidraw/excalidraw";
import "@excalidraw/excalidraw/index.css";
import { ICONS } from "../../../web/src/generated/icons";
import { LIBRARIES, search, type Entry, type Library } from "./library";

const SIM_LABEL = { full: "Simulated", hop: "Pass-through", "off-path": "Diagram only" } as const;
const CAT_TINT: Record<string, string> = { compute: "#fff4e6", data: "#ebfbee", network: "#e7f5ff", messaging: "#f3f0ff", security: "#fff5f5", observability: "#f8f9fa", external: "#f1f3f5" };

function iconURL(lib: Library, e: { icon: string | null; genericIcon: string }) {
  if (e.icon && lib.iconBase) return `${lib.iconBase}${e.icon}.svg`;
  const svg = (ICONS[e.genericIcon] ?? ICONS["box"] ?? "").replace(/currentColor/g, "#495057");
  return `data:image/svg+xml;base64,${btoa(svg)}`;
}

async function dataURL(url: string): Promise<string> {
  if (url.startsWith("data:")) return url;
  const svg = await (await fetch(url)).text();
  return `data:image/svg+xml;base64,${btoa(unescape(encodeURIComponent(svg)))}`;
}

function Row({ lib, e, active, onAdd }: { lib: Library; e: Entry; active: boolean; onAdd: (e: Entry) => void }) {
  return (
    <div
      className={`row ${active ? "active" : ""}`}
      draggable
      onDragStart={(ev) => ev.dataTransfer.setData("text/component", e.id)}
      onClick={() => onAdd(e)}
      title={`${e.name}${e.service ? ` · ${e.service}` : ""}`}
    >
      <img src={iconURL(lib, e)} width={28} height={28} alt="" />
      <div className="txt">
        <div className="name">{e.label}</div>
        <div className="sub">
          {e.place === "group" ? "Group · " : e.place === "source" ? "Traffic source · " : ""}
          {e.label !== e.name ? e.name : e.gloss}
        </div>
      </div>
      {e.place === "group" ? <span className="sim group">{e.groupKind === "cluster" ? "Capacity pool" : "Group"}</span> : <span className={`sim ${e.sim}`}>{SIM_LABEL[e.sim]}</span>}
      {e.hipaa === false && <span className="hipaa" title="Not on the provider's HIPAA-eligible list">not HIPAA</span>}
    </div>
  );
}

function LibraryPanel({ lib, onAdd, onProvider }: { lib: Library; onAdd: (e: Entry) => void; onProvider: (p: string) => void }) {
  const [q, setQ] = useState("");
  const [open, setOpen] = useState<Record<string, boolean>>({ lb: true, compute: true, ecs: true, eks: true });
  const [cursor, setCursor] = useState(0);
  const input = useRef<HTMLInputElement>(null);
  const t0 = performance.now();
  const results = useMemo(() => search(lib, q), [lib, q]);
  const searchMs = performance.now() - t0;
  (window as any).__lastSearchMs = searchMs;
  // Results stay grouped: groups in order of their best match, entries ranked inside.
  const grouped = useMemo(() => {
    const m = new Map<string, Entry[]>();
    for (const r of results) {
      if (!m.has(r.entry.group)) m.set(r.entry.group, []);
      m.get(r.entry.group)!.push(r.entry);
    }
    return [...m.entries()];
  }, [results]);
  const flat = grouped.flatMap(([, es]) => es);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "/" && document.activeElement !== input.current) {
        e.preventDefault();
        input.current?.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <aside className="lib">
      <div className="lib-head">
        <strong>Components</strong>
        <select value={lib.provider} onChange={(e) => onProvider(e.target.value)}>
          <option value="aws">AWS</option>
          <option value="azure">Azure (draft)</option>
        </select>
      </div>
      <input
        ref={input}
        className="search"
        placeholder="Search components  ( / )"
        value={q}
        onChange={(e) => { setQ(e.target.value); setCursor(0); }}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") { setCursor((c) => Math.min(flat.length - 1, c + 1)); e.preventDefault(); }
          if (e.key === "ArrowUp") { setCursor((c) => Math.max(0, c - 1)); e.preventDefault(); }
          if (e.key === "Enter" && flat[cursor]) onAdd(flat[cursor]);
          if (e.key === "Escape") setQ("");
        }}
      />
      {!q && (
        <div className="quick">
          {lib.quick.map((x) => <span key={x.id} className="chip" title={`Drop onto ${x.onto === "edge" ? "an Edge" : "a Node"} (${x.rule})`}>{x.title}</span>)}
        </div>
      )}
      <div className="list">
        {q ? (
          grouped.length ? grouped.map(([g, es]) => (
            <div key={g}>
              <div className="gh static">{lib.groups.find((x) => x.id === g)?.title}<span>{es.length}</span></div>
              {es.map((e) => <Row key={e.id} lib={lib} e={e} active={flat[cursor] === e} onAdd={onAdd} />)}
            </div>
          )) : <p className="muted">No component matches “{q}”.</p>
        ) : (
          lib.groups.filter((g) => g.id !== "quick").map((g) => {
            const es = lib.entries.filter((e) => e.group === g.id);
            if (!es.length) return null;
            const isOpen = open[g.id] ?? false;
            return (
              <div key={g.id}>
                <div className="gh" onClick={() => setOpen({ ...open, [g.id]: !isOpen })}>
                  <span className="caret">{isOpen ? "▾" : "▸"}</span>
                  {g.icon && lib.iconBase ? <img src={`${lib.iconBase}${g.icon}.svg`} width={16} height={16} alt="" /> : null}
                  {g.title}<span>{es.length}</span>
                </div>
                {isOpen && es.map((e) => <Row key={e.id} lib={lib} e={e} active={false} onAdd={onAdd} />)}
              </div>
            );
          })
        )}
      </div>
      <div className="foot muted">
        {lib.iconBase ? "AWS Architecture Icons 07312026, downloaded on first use under AWS's terms." : "Generic icons: no Azure icon set pinned yet."}
      </div>
    </aside>
  );
}

function App() {
  const [provider, setProvider] = useState("aws");
  const lib = LIBRARIES[provider];
  const api = useRef<any>(null);
  const stage = useRef<HTMLDivElement>(null);
  const placed = useRef(0);

  const place = async (e: Entry, at?: { x: number; y: number }) => {
    const a = api.current;
    if (!a) return;
    const st = a.getAppState();
    const p = at ?? { x: -st.scrollX + (st.width / 2) / st.zoom.value - 100 + (placed.current % 4) * 230, y: -st.scrollY + 120 / st.zoom.value + Math.floor(placed.current / 4) * 110 };
    placed.current++;
    const fileId = `icon-${provider}-${e.icon ?? e.genericIcon}`;
    a.addFiles([{ id: fileId, mimeType: "image/svg+xml", dataURL: await dataURL(iconURL(lib, e)), created: Date.now() }]);
    const gid = `g-${crypto.randomUUID()}`;
    const id = `n-${crypto.randomUUID().slice(0, 8)}`;
    let skel: any[];
    if (e.place === "group") {
      skel = [
        { type: "rectangle", id, x: p.x, y: p.y, width: 460, height: 260, strokeStyle: "dashed", strokeColor: "#7048e8", backgroundColor: "transparent", roughness: 0, customData: { groupKind: e.groupKind, service: e.service } },
        { type: "image", id: `${id}~i`, x: p.x + 10, y: p.y + 8, width: 24, height: 24, fileId, groupIds: [], locked: true },
        { type: "text", x: p.x + 40, y: p.y + 12, text: `${e.label}`, fontSize: 14, fontFamily: 6, strokeColor: "#5f3dc4" },
      ];
    } else {
      const tint = CAT_TINT[e.kind.split(".")[0]] ?? "#fff";
      skel = [
        { type: "rectangle", id, x: p.x, y: p.y, width: 200, height: 72, roughness: 0, roundness: { type: 3 }, fillStyle: "solid", backgroundColor: tint, strokeColor: "#495057", groupIds: [gid], customData: { kind: e.kind, service: e.service, component: e.id } },
        { type: "image", id: `${id}~i`, x: p.x + 12, y: p.y + 16, width: 40, height: 40, fileId, groupIds: [gid] },
        { type: "text", x: p.x + 62, y: p.y + 14, text: e.label.length > 22 ? e.label.slice(0, 21) + "…" : e.label, fontSize: 14, fontFamily: 6, groupIds: [gid] },
        { type: "text", x: p.x + 62, y: p.y + 38, text: (e.label !== e.name ? e.name : e.kind).slice(0, 26), fontSize: 11, fontFamily: 6, strokeColor: "#868e96", groupIds: [gid] },
      ];
    }
    const els = convertToExcalidrawElements(skel as any, { regenerateIds: false });
    a.updateScene({ elements: [...a.getSceneElements(), ...els], captureUpdate: CaptureUpdateAction.IMMEDIATELY });
  };

  return (
    <div className="app">
      <LibraryPanel lib={lib} onAdd={(e) => place(e)} onProvider={setProvider} />
      <div
        className="stage"
        ref={stage}
        onDragOver={(ev) => ev.preventDefault()}
        onDrop={(ev) => {
          ev.preventDefault();
          const e = lib.entries.find((x) => x.id === ev.dataTransfer.getData("text/component"));
          if (!e || !api.current) return;
          const p = viewportCoordsToSceneCoords({ clientX: ev.clientX, clientY: ev.clientY }, api.current.getAppState());
          place(e, { x: p.x - 100, y: p.y - 36 });
        }}
      >
        <Excalidraw onExcalidrawAPI={(a: any) => { api.current = a; (window as any).__api = a; (window as any).__place = (id: string) => place(LIBRARIES[provider].entries.find((x) => x.id === id)!); }} />
      </div>
    </div>
  );
}

const css = `
* { box-sizing: border-box; }
html, body, #root { margin: 0; height: 100%; font: 13px system-ui, sans-serif; color: #212529; }
.app { display: flex; height: 100%; }
.lib { width: 320px; border-right: 1px solid #dee2e6; display: flex; flex-direction: column; background: #fff; }
.lib-head { display: flex; justify-content: space-between; align-items: center; padding: 10px 12px 6px; }
.lib-head select { font: inherit; }
.search { margin: 0 12px 8px; padding: 7px 10px; border: 1px solid #ced4da; border-radius: 6px; font: inherit; }
.search:focus { outline: 2px solid #4dabf7; border-color: #4dabf7; }
.quick { display: flex; flex-wrap: wrap; gap: 6px; padding: 0 12px 8px; }
.chip { border: 1px dashed #adb5bd; border-radius: 12px; padding: 2px 9px; font-size: 12px; color: #495057; cursor: grab; }
.list { flex: 1; overflow: auto; padding-bottom: 8px; }
.gh { display: flex; align-items: center; gap: 6px; padding: 7px 12px; font-weight: 600; cursor: pointer; user-select: none; border-top: 1px solid #f1f3f5; }
.gh.static { cursor: default; color: #495057; font-size: 12px; }
.gh span:last-child { margin-left: auto; color: #adb5bd; font-weight: 400; }
.caret { width: 10px; color: #868e96; }
.row { display: flex; align-items: center; gap: 9px; padding: 5px 12px 5px 22px; cursor: grab; }
.row:hover, .row.active { background: #f1f3f5; }
.row .txt { flex: 1; min-width: 0; }
.row .name { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.row .sub { color: #868e96; font-size: 11px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.sim { font-size: 10px; padding: 1px 6px; border-radius: 8px; white-space: nowrap; }
.sim.full { background: #d3f9d8; color: #2b8a3e; } .sim.hop { background: #e7f5ff; color: #1971c2; } .sim.off-path { background: #f1f3f5; color: #868e96; }
.sim.group { background: #f3f0ff; color: #6741d9; }
.hipaa { font-size: 10px; color: #c92a2a; }
.foot { padding: 8px 12px; border-top: 1px solid #f1f3f5; font-size: 11px; }
.muted { color: #868e96; padding: 0 12px; }
.stage { flex: 1; position: relative; }
`;
const style = document.createElement("style");
style.textContent = css;
document.head.appendChild(style);
createRoot(document.getElementById("root")!).render(<App />);
