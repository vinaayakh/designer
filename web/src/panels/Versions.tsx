import type { Version } from "../types";

const SOURCE: Record<string, string> = { conversion: "Conversion", proposal: "Proposal", edit: "Hand edits", tidy: "Tidy up", restore: "Restored" };

export function Versions({ versions, viewing, onView, onRestore, onLatest }: {
  versions: Version[];
  viewing: number | null;
  onView: (n: number) => void;
  onRestore: (n: number) => void;
  onLatest: () => void;
}) {
  if (!versions.length) return <div className="panel"><div className="empty">No Production versions yet. Conversion creates version 1.</div></div>;
  const latest = versions[versions.length - 1].n;
  return (
    <div className="panel">
      {viewing !== null && viewing !== latest && (
        <div className="callout">
          <div className="small">Viewing v{viewing} (read-only).</div>
          <div className="row" style={{ marginTop: 6 }}>
            <button className="primary" onClick={() => onRestore(viewing)}>Restore as new version</button>
            <button onClick={onLatest}>Back to latest</button>
          </div>
        </div>
      )}
      {[...versions].reverse().map((v) => (
        <div key={v.n} className={`card clickable ${(viewing ?? latest) === v.n ? "active" : ""}`} onClick={() => (v.n === latest ? onLatest() : onView(v.n))}>
          <div className="row">
            <span className="mono" style={{ fontWeight: 600 }}>v{v.n}</span>
            <span className="pill">{SOURCE[v.source] ?? v.source}</span>
            <span className="spacer" />
            <span className="small muted" style={{ fontVariantNumeric: "tabular-nums" }}>{new Date(v.created).toLocaleString()}</span>
          </div>
          {v.summary && <div className="small" style={{ marginTop: 4 }}>{v.summary}</div>}
        </div>
      ))}
      <p className="small muted">Hand edits within 10 minutes of each other are saved into one version.</p>
    </div>
  );
}
