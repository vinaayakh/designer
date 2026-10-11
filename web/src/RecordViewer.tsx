import { useEffect, useState } from "react";
import { api } from "./api";
import { Markdown } from "./Markdown";
import type { KBRecord } from "./types";

const TIER: Record<string, string> = { "public-domain": "public domain", attribution: "CC BY, adapted", "facts-only": "facts only, in our own words" };

/** Shows one Knowledge Base Record with its sources and licence credits. */
export function RecordViewer({ id, onClose, onOpen }: { id: string; onClose: () => void; onOpen: (id: string) => void }) {
  const [r, setR] = useState<KBRecord | null>(null);
  const [err, setErr] = useState("");
  useEffect(() => {
    setR(null);
    api.record(id).then(setR, (e) => setErr(e.message));
  }, [id]);
  return (
    <div className="modal-back" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-label="Knowledge Base Record">
        <div className="row">
          <span className="eyebrow">{r?.type === "control" ? `${r.regime?.toUpperCase() ?? ""} control` : r?.type ?? "Record"}</span>
          <span className="small muted mono">{id}</span>
          <span className="spacer" />
          <button className="ghost" onClick={onClose} aria-label="Close">✕</button>
        </div>
        {err && <p className="error-text">{err}</p>}
        {r && (
          <>
            <h2 style={{ marginTop: 10 }}>{r.title}</h2>
            <p className="lede" style={{ fontSize: 15, marginTop: 6 }}>{r.summary}</p>
            <div className="row wrap small">
              {r.ref && <span className="pill mono">{r.ref}</span>}
              {r.obligation && <span className="pill">{r.obligation}</span>}
              <span className="pill">coverage: {r.coverage}</span>
              <span className="pill">revised {r.revised}</span>
            </div>
            <Markdown text={r.body} />
            {(r.satisfiedBy?.length || r.satisfies?.length) ? (
              <p className="small">
                {r.satisfiedBy?.length ? <>Satisfied by: {r.satisfiedBy.map((x) => <button key={x} className="link mono" style={{ marginRight: 8 }} onClick={() => onOpen(x)}>{x}</button>)}</> : null}
                {r.satisfies?.length ? <>Satisfies: {r.satisfies.map((x) => <button key={x} className="link mono" style={{ marginRight: 8 }} onClick={() => onOpen(x)}>{x}</button>)}</> : null}
              </p>
            ) : null}
            <h4>Sources</h4>
            {r.sources.map((s, i) => (
              <div key={i} className="small" style={{ marginBottom: 6 }}>
                <a href={s.url} target="_blank" rel="noreferrer">{s.title}</a> <span className="muted">· {TIER[s.licenceTier] ?? s.licenceTier} · as of {s.asOf}</span>
                {s.attribution && <div className="muted">{s.attribution}</div>}
              </div>
            ))}
          </>
        )}
      </div>
    </div>
  );
}
