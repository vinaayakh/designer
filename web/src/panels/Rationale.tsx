import { useState } from "react";
import type { Citation, Pending, Rationale } from "../types";

function Cite({ c, onOpenRecord }: { c: Citation; onOpenRecord: (id: string) => void }) {
  if (c.record) return <button className="link mono" onClick={() => onOpenRecord(c.record!)}>{c.record}</button>;
  return <a className="small" href={c.url} target="_blank" rel="noreferrer">{c.title || c.url} <span className="pill">web</span></a>;
}

/** The Design Rationale as a document; clicking a Node shows its entry. */
export function RationaleView({ r, label, selected, onSelect, onOpenRecord }: {
  r: Rationale | undefined;
  label: (id?: string) => string;
  selected?: string | null;
  onSelect: (id: string) => void;
  onOpenRecord: (id: string) => void;
}) {
  if (!r) return <div className="empty">No Design Rationale yet.</div>;
  const nodes = Object.entries(r.nodes ?? {});
  const sel = selected && r.nodes?.[selected] ? selected : null;
  return (
    <div>
      <p className="lede" style={{ fontSize: 14 }}>{r.summary}</p>
      {sel && (
        <div className="card active">
          <div style={{ fontWeight: 600 }}>{label(sel)}</div>
          <NodeEntry e={r.nodes![sel]} onOpenRecord={onOpenRecord} />
        </div>
      )}
      {r.assumptions?.length ? (
        <>
          <h4>Assumptions</h4>
          <ul style={{ paddingLeft: 18, margin: 0 }}>{r.assumptions.map((a, i) => <li key={i} style={{ marginBottom: 4 }}>{a}</li>)}</ul>
          <p className="small muted">Disagree? Reply in the Assistant and it will propose a change.</p>
        </>
      ) : null}
      {r.tradeoffs?.length ? (
        <>
          <h4>Trade-offs</h4>
          {r.tradeoffs.map((t, i) => (
            <div key={i} className="card small">
              <b>{t.decision}</b>: chose {t.chose} over {t.over}, because {t.because}
            </div>
          ))}
        </>
      ) : null}
      <h4>Why each component exists</h4>
      {nodes.map(([id, e]) => (
        <div key={id} className={`card clickable ${id === sel ? "active" : ""}`} onClick={() => onSelect(id)}>
          <div style={{ fontWeight: 600 }}>{label(id)}</div>
          <NodeEntry e={e} onOpenRecord={onOpenRecord} />
        </div>
      ))}
    </div>
  );
}

function NodeEntry({ e, onOpenRecord }: { e: NonNullable<Rationale["nodes"]>[string]; onOpenRecord: (id: string) => void }) {
  return (
    <>
      <div className="small">{e.why}</div>
      {(e.practices?.length || e.cites?.length) ? (
        <div className="row wrap" style={{ marginTop: 4 }}>
          {e.practices?.filter((p) => !e.cites?.some((c) => c.record === p)).map((p) => <button key={p} className="link mono" onClick={() => onOpenRecord(p)}>{p}</button>)}
          {e.cites?.map((c, i) => <Cite key={i} c={c} onOpenRecord={onOpenRecord} />)}
        </div>
      ) : null}
    </>
  );
}

/** The Conversion Preview: review the whole Design, all or nothing (ticket 011). */
export function ConversionPanel({ pending, label, selected, onSelect, onOpenRecord, onAccept, onReject, onReply, busy, laidOut, layoutError, onRetryLayout }: {
  pending: Pending;
  label: (id?: string) => string;
  selected?: string | null;
  onSelect: (id: string) => void;
  onOpenRecord: (id: string) => void;
  onAccept: () => void;
  onReject: () => void;
  onReply: (text: string) => void;
  busy: boolean;
  laidOut: boolean;
  layoutError: string;
  onRetryLayout: () => void;
}) {
  const [reply, setReply] = useState("");
  if (pending.awaitingRevision) {
    return (
      <div className="panel">
        <h3>Conversion Preview</h3>
        <div className="callout"><div className="small muted">You replied:</div>{pending.reply}</div>
        <p className="muted">The Assistant is revising the design…</p>
      </div>
    );
  }
  return (
    <div className="panel">
      <div className="eyebrow">Conversion Preview{pending.revision > 1 ? ` · revision ${pending.revision}` : ""}</div>
      <h3 style={{ marginTop: 8 }}>{pending.payload.title}</h3>
      <div className="callout">
        <div className="small">Accepting creates Production version 1 and <b>freezes the Rough Draft</b>. A Gap Check runs right after.</div>
        {layoutError && (
          <div className="callout warn small" style={{ marginTop: 10 }}>
            Couldn't lay out the design: {layoutError}
            <div style={{ marginTop: 6 }}><button onClick={onRetryLayout}>Try again</button></div>
          </div>
        )}
        <div className="row" style={{ marginTop: 8 }}>
          <button className="primary" onClick={onAccept} disabled={busy || !laidOut}>{laidOut ? "Accept" : layoutError ? "Not laid out" : "Laying out…"}</button>
          <button className="danger" onClick={onReject} disabled={busy}>Reject</button>
        </div>
      </div>
      <RationaleView r={pending.payload.rationale} label={label} selected={selected} onSelect={onSelect} onOpenRecord={onOpenRecord} />
      <h4>Reply with changes</h4>
      <textarea rows={3} value={reply} onChange={(e) => setReply(e.target.value)} placeholder="e.g. “The DB is Postgres, not DynamoDB” — you'll get a new revision." />
      <div className="row" style={{ marginTop: 6 }}>
        <span className="spacer" />
        <button disabled={!reply.trim() || busy} onClick={() => { onReply(reply.trim()); setReply(""); }}>Send reply</button>
      </div>
    </div>
  );
}
