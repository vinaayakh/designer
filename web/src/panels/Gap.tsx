// The Gap Report (ticket 011): one item per Control that applies (met /
// unmet / outside), plus security recommendations. Fix and Fix all draft a
// Proposal on demand; a report is never edited after it's written.
import { useEffect, useState } from "react";
import { api } from "../api";
import type { GapItem, GapReport, ReportRef } from "../types";

const STATUS: Record<GapItem["status"], { label: string; cls: string }> = {
  unmet: { label: "unmet", cls: "bad" },
  met: { label: "met", cls: "ok" },
  outside: { label: "outside the diagram", cls: "" },
};

export function GapPanel({ reports, latest, label, onSelectNode, onOpenRecord, fixed, busy, pendingProposal, onCheck }: {
  reports: ReportRef[];
  latest: number;
  label: (id?: string) => string;
  onSelectNode: (id: string) => void;
  onOpenRecord: (id: string) => void;
  fixed: Record<string, number>; // record -> version where its fix was applied
  busy: boolean;
  pendingProposal: boolean;
  onCheck: () => void;
}) {
  const [file, setFile] = useState<string | null>(null);
  const [rep, setRep] = useState<GapReport | null>(null);
  const [sel, setSel] = useState<Set<string>>(new Set());
  const [err, setErr] = useState("");
  const current = file ?? reports[reports.length - 1]?.file ?? null;

  useEffect(() => {
    setFile(null);
  }, [reports.length]);
  useEffect(() => {
    if (!current) return setRep(null);
    api.report(current).then(setRep, (e) => setErr(e.message));
    setSel(new Set());
  }, [current]);

  const fix = async (records: string[]) => {
    setErr("");
    try {
      await api.fix(current!, records);
    } catch (e) {
      setErr((e as Error).message);
    }
  };

  if (!reports.length) {
    return (
      <div className="panel">
        <div className="empty">No Gap Report yet.<br /><br /><button className="primary" disabled={busy || latest === 0} onClick={onCheck}>Run Gap Check</button></div>
        {err && <p className="error-text">{err}</p>}
      </div>
    );
  }
  const unmet = rep?.items.filter((i) => i.status === "unmet") ?? [];
  const groups: GapItem["status"][] = ["unmet", "met", "outside"];
  return (
    <div className="panel">
      <div className="row">
        <select value={current ?? ""} onChange={(e) => setFile(e.target.value)} aria-label="Gap Report">
          {[...reports].reverse().map((r) => <option key={r.file} value={r.file}>v{r.version} · {new Date(r.created).toLocaleString()}</option>)}
        </select>
        <span className="spacer" />
        <button onClick={onCheck} disabled={busy}>Check again</button>
      </div>
      {rep && rep.version < latest && (
        <div className="callout warn small" style={{ marginTop: 12 }}>
          Checked against v{rep.version}; the current version is v{latest}.
        </div>
      )}
      {rep && (
        <>
          <p className="small muted" style={{ marginTop: 12 }}>
            {rep.regimes.length ? rep.regimes.map((r) => r.toUpperCase()).join(" + ") : "No compliance regime"} · Knowledge Base {rep.kbVersion} · {unmet.length} unmet
          </p>
          {unmet.length > 0 && (
            <div className="row" style={{ marginBottom: 8 }}>
              <button className="primary" disabled={busy || pendingProposal || sel.size === 0} onClick={() => fix([...sel])}>Fix all selected ({sel.size})</button>
              <button className="link" onClick={() => setSel(new Set(unmet.map((u) => u.record)))}>select all</button>
            </div>
          )}
          {groups.map((g) => {
            const items = rep.items.filter((i) => i.status === g);
            if (!items.length) return null;
            return (
              <div key={g}>
                <h4>{STATUS[g].label} ({items.length})</h4>
                {items.map((it) => (
                  <div key={it.record} className="card">
                    <div className="row" style={{ alignItems: "flex-start" }}>
                      {g === "unmet" && (
                        <input type="checkbox" checked={sel.has(it.record)} aria-label={`Select ${it.title}`}
                          onChange={() => setSel((s) => { const n = new Set(s); n.has(it.record) ? n.delete(it.record) : n.add(it.record); return n; })} />
                      )}
                      <div style={{ flex: 1 }}>
                        <div style={{ fontWeight: 600 }}>{it.title}</div>
                        <button className="link mono" onClick={() => onOpenRecord(it.record)}>{it.record}</button>
                      </div>
                      <span className={`pill ${STATUS[g].cls}`}>{STATUS[g].label}</span>
                    </div>
                    <div className="small" style={{ marginTop: 4 }}>{it.finding}</div>
                    {g === "met" && it.coverage === "partial" && <div className="small muted">The diagram shows the component, not its configuration.</div>}
                    {it.nodes?.length ? (
                      <div className="row wrap small" style={{ marginTop: 4 }}>
                        {it.nodes.map((n) => <button key={n} className="link" onClick={() => onSelectNode(n)}>{label(n)}</button>)}
                      </div>
                    ) : null}
                    {g === "unmet" && (
                      <div className="row" style={{ marginTop: 6 }}>
                        {it.fix?.map((f) => <button key={f} className="link mono" onClick={() => onOpenRecord(f)}>{f}</button>)}
                        <span className="spacer" />
                        {fixed[it.record] ? (
                          <span className="pill ok">fix applied in v{fixed[it.record]}, check again</span>
                        ) : (
                          <button disabled={busy || pendingProposal} onClick={() => fix([it.record])}>Fix</button>
                        )}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            );
          })}
          {rep.recommendations?.length ? (
            <>
              <h4>Security recommendations</h4>
              {rep.recommendations.map((r) => (
                <div key={r.practice} className="card">
                  <div style={{ fontWeight: 600 }}>{r.title}</div>
                  <button className="link mono" onClick={() => onOpenRecord(r.practice)}>{r.practice}</button>
                  <div className="small">{r.finding}</div>
                  {r.nodes?.length ? <div className="row wrap small">{r.nodes.map((n) => <button key={n} className="link" onClick={() => onSelectNode(n)}>{label(n)}</button>)}</div> : null}
                </div>
              ))}
            </>
          ) : null}
          <p className="small muted">Compliance checks report and suggest; they never change the diagram on their own.</p>
        </>
      )}
      {err && <p className="error-text">{err}</p>}
    </div>
  );
}
