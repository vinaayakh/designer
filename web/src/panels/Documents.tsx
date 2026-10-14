// The Documents tab: every document in a list with Create, and the Register beside it (the shared
// facts every document and the Assistant use). Create opens a wizard when the document still needs
// answers (CreateWizard); documents it builds on that aren't written yet are written first.
// Replaces the stepper of wayfinder/doc-pipeline ticket 010.
import { useEffect, useState } from "react";
import { api, download } from "../api";
import { Markdown } from "../Markdown";
import { CreateWizard, pagesFor } from "./CreateWizard";
import type { EntryType, PipelineStep, PipelineView, ProposedEntry, RegisterEntry } from "../types";

const TYPE_NAMES: Record<EntryType, string> = {
  REQ: "Requirements", CON: "Constraints", ASM: "Assumptions", NG: "Non-goals & out of scope", DEC: "Decisions", RISK: "Risks", Q: "Open questions",
};
const TYPES: EntryType[] = ["REQ", "CON", "ASM", "NG", "DEC", "RISK", "Q"];

export function Documents({ view, onError, onHighlight, onChanged, onArc42 }: {
  view: PipelineView | null;
  onArc42: (format: "docx" | "pdf") => void;
  onError: (e: unknown) => void;
  onHighlight: (ids: string[]) => void;
  onChanged: () => void;
}) {
  const [tab, setTab] = useState<"documents" | "register">("documents");
  const [wizard, setWizard] = useState<{ id: string; all: boolean } | null>(null);
  const [reading, setReading] = useState<{ title: string; markdown: string } | null>(null);
  useEffect(() => {
    if (!reading) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") setReading(null); };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [reading]);
  const [packing, setPacking] = useState(false);
  const downloadPack = () => {
    setPacking(true);
    download("/api/documents/pack", {}).then(({ blob, name }) => {
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = name;
      a.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 2000);
    }, onError).finally(() => setPacking(false));
  };
  if (!view) return <div className="docs"><p className="muted small">Loading…</p></div>;
  const entries = view.entries ?? [];
  const openCount = entries.filter((e) => e.status === "open").length;
  const act = (p: Promise<unknown>) => p.then(onChanged, onError);
  // One document at a time: the Assistant writes them one task each.
  const busy = view.steps.some((s) => s.status.kind === "writing") || !!view.queue?.length;
  const create = (id: string) => {
    if (pagesFor(view, id, false).length) setWizard({ id, all: false });
    else act(api.createDocument(id, []));
  };

  return (
    <div className="docs">
      <div className="docs-seg">
        <button className={tab === "documents" ? "on" : ""} onClick={() => setTab("documents")}>Documents</button>
        <button className={tab === "register" ? "on" : ""} onClick={() => setTab("register")}>
          Register {openCount > 0 && <span className="pill warn">{openCount} open</span>}
        </button>
      </div>
      <div className="docs-pack">
        <button className="small" disabled={packing || !view.steps.some((s) => s.versions?.length)} onClick={downloadPack}
          title="A zip of every current document, an ADR per decision, the Register with its history and raw log, and an index">
          {packing ? "Preparing…" : "Download document pack"}
        </button>
      </div>
      {view.integrity && <p className="docs-warn">The Register's history was edited outside Designer: {view.integrity}</p>}

      {tab === "documents" && (
        <div className="docs-list">
          {view.steps.map((s) => (
            <DocCard key={s.id} step={s} view={view} busy={busy}
              onCreate={() => create(s.id)} onCreateOther={create}
              onEditAnswers={() => setWizard({ id: s.id, all: true })}
              onRead={(n) => api.docVersion(s.id, n).then((r) => setReading({ title: `${s.title} v${n}`, markdown: r.markdown }), onError)} />
          ))}
          <div className="docs-card">
            <div className="docs-card-head">
              <span className="docs-step-title">arc42 architecture document</span>
              <span className={`pill ${!view.arc42 ? "" : view.arc42.stale ? "bad" : view.arc42.written ? "ok" : "accent"}`}>
                {!view.arc42 ? "Needs a Production diagram" : view.arc42.stale ? `Text out of date (v${view.arc42.version})` : view.arc42.written ? `Written for v${view.arc42.version}` : "Ready to export"}
              </span>
            </div>
            <p className="small">The whole architecture in the arc42 template, built from the diagram, the Gap Report and the same Register entries the other documents cite.</p>
            <div className="row">
              <button className="primary" disabled={!view.arc42} onClick={() => onArc42("docx")}>Export Word</button>
              <button disabled={!view.arc42} onClick={() => onArc42("pdf")}>Export PDF</button>
            </div>
          </div>
        </div>
      )}

      {tab === "register" && (
        <div className="docs-register">
          <p className="muted small">The shared facts: your answers, the Design Brief and what the documents decided. Every document cites them, and the Assistant works from them when it changes the diagram.</p>
          {!!view.proposed?.length && <ProposedEntries items={view.proposed} onError={onError} onChanged={onChanged} />}
          {entries.length === 0 && <p className="muted small">Nothing yet. Answers, the Design Brief and written documents add entries here.</p>}
          {TYPES.map((t) => {
            const es = entries.filter((e) => e.type === t);
            if (!es.length) return null;
            const opens = es.filter((e) => e.status === "open").length;
            return (
              <details key={t} open={opens > 0}>
                <summary>{TYPE_NAMES[t]} <span className="muted">{es.length}</span>{opens > 0 && <span className="pill warn">{opens} open</span>}</summary>
                {es.map((e) => <EntryCard key={e.id} e={e} onHighlight={onHighlight} act={act} />)}
              </details>
            );
          })}
        </div>
      )}

      {wizard && (
        <CreateWizard view={view} id={wizard.id} all={wizard.all} onError={onError}
          onClose={() => setWizard(null)} onDone={() => { setWizard(null); onChanged(); }} />
      )}

      {reading && (
        <div className="modal-back" onClick={() => setReading(null)}>
          <div className="modal docs-reader" onClick={(e) => e.stopPropagation()} role="dialog" aria-label={reading.title}>
            <Markdown text={reading.markdown} />
          </div>
        </div>
      )}
    </div>
  );
}

function StatusChip({ step, written }: { step: PipelineStep; written: boolean }) {
  const k = step.status.kind;
  const v = step.versions?.[step.versions.length - 1];
  // In a list nothing "waits": Create writes what a document builds on first.
  const [cls, text] =
    k === "writing" || k === "queued" ? ["accent", step.status.text]
    : k === "stale" ? ["bad", `v${v} · out of date`]
    : written ? ["ok", `v${v} · up to date`]
    : ["", "Not written"];
  return <span className={`pill ${cls}`} title={step.status.reasons?.join("\n")}>{text}</span>;
}

/**
 * What stops a document being created now: documents it requires that aren't written yet, and
 * the Production Diagram for documents built from it. Documents it only "uses" don't block.
 */
function blockersOf(view: PipelineView, step: PipelineStep) {
  const missing = (step.requires ?? []).map((id) => view.steps.find((s) => s.id === id)!).filter((s) => s && !s.versions?.length);
  const noDiagram = step.diagram === "production" && !view.production;
  return { missing, noDiagram, blocked: missing.length > 0 || noDiagram };
}

const andList = (xs: string[]) => (xs.length < 2 ? xs.join("") : `${xs.slice(0, -1).join(", ")} and ${xs[xs.length - 1]}`);

function DocCard({ step, view, busy, onCreate, onCreateOther, onEditAnswers, onRead }: {
  step: PipelineStep; view: PipelineView; busy: boolean; onCreate: () => void; onCreateOther: (id: string) => void; onEditAnswers: () => void; onRead: (n: number) => void;
}) {
  const versions = step.versions ?? [];
  const written = versions.length > 0;
  const kind = step.status.kind;
  const { missing, noDiagram, blocked } = blockersOf(view, step);
  // Optional inputs: used when they exist, so worth a hint but never a block.
  const soon = (step.uses ?? []).map((id) => view.steps.find((s) => s.id === id)!).filter((s) => s && !s.versions?.length);
  // Documents waiting on this one, to say what writing it unlocks.
  const unlocks = view.steps.filter((s) => s.requires?.includes(step.id) && !s.versions?.length);
  const asks = pagesFor(view, step.id, false).length;
  const action = !written ? "Create" : kind === "stale" ? "Update" : "Rewrite";
  const working = kind === "writing" || kind === "queued";
  const why = noDiagram
    ? "Needs a Production Diagram: convert the Rough Draft first"
    : `Needs the ${andList(missing.map((s) => s.title))} first`;
  return (
    <div className={`docs-card ${kind} ${blocked && !written ? "blocked" : ""}`}>
      <div className="docs-card-head">
        <span className="docs-step-title">{step.title}</span>
        {blocked && !written ? <span className="pill">Not available yet</span> : <StatusChip step={step} written={written} />}
      </div>
      {step.summary && <p className="small docs-card-summary">{step.summary}</p>}
      {blocked && (
        <div className="docs-blocked-msg" role="note">
          <strong>{why}.</strong>
          {!noDiagram && <> It builds on {missing.length === 1 ? "that document's" : "their"} facts, so it can be created once {missing.length === 1 ? "it is" : "they are"} written.</>}
          {!noDiagram && missing.length > 0 && (
            <div className="row">
              {missing.map((m) => (
                <button key={m.id} className="small" disabled={busy || blockersOf(view, m).blocked} onClick={() => onCreateOther(m.id)}>Create the {m.title}</button>
              ))}
            </div>
          )}
          {step.questions.length > 0 && <span className="muted small"> You can prepare its answers now.</span>}
        </div>
      )}
      {!blocked && soon.length > 0 && !written && (
        <p className="muted small">Uses the {andList(soon.map((s) => s.title))} when {soon.length === 1 ? "it is" : "they are"} written; you can create it now or after.</p>
      )}
      {unlocks.length > 0 && !written && <p className="muted small">Needed by the {andList(unlocks.map((s) => s.title))}.</p>}
      {kind === "writing" && <div className="docs-progress"><span className="spinner" aria-hidden /> The Assistant is writing it. Its progress shows in the Assistant tab.</div>}
      {kind === "stale" && <ul className="docs-reasons">{step.status.reasons?.map((r) => <li key={r}>{r}</li>)}</ul>}
      <div className="row">
        <button className={written && kind !== "stale" ? "" : "primary"} disabled={working || busy || blocked}
          title={blocked ? why : busy && !working ? "Another document is being written" : asks ? `Asks ${asks} question${asks === 1 ? "" : "s"} first` : ""} onClick={onCreate}>
          {action}{asks > 0 && !working && !blocked ? "…" : ""}
        </button>
        {written && <button onClick={() => onRead(versions[versions.length - 1])}>Read v{versions[versions.length - 1]}</button>}
        {versions.length > 1 && (
          <select className="docs-older" value="" onChange={(e) => e.target.value && onRead(+e.target.value)} aria-label="Older versions">
            <option value="">Older…</option>
            {versions.slice(0, -1).reverse().map((n) => <option key={n} value={n}>v{n}</option>)}
          </select>
        )}
        <span className="spacer" />
        {step.questions.length > 0 && <button className="link small" disabled={working} onClick={onEditAnswers}>Edit answers</button>}
      </div>
    </div>
  );
}

function EntryCard({ e, onHighlight, act }: { e: RegisterEntry; onHighlight: (ids: string[]) => void; act: (p: Promise<unknown>) => void }) {
  const cls = { open: "warn", confirmed: "ok", rejected: "bad", superseded: "" }[e.status];
  const src = [e.source.kind, e.source.ref].filter(Boolean).join(" · ");
  const reject = () => {
    const replacement = window.prompt(`Reject ${e.id}. Replace it with (optional; leave empty to just reject):`, "");
    if (replacement === null) return;
    act(api.entryStatus(e.id, "rejected", "", replacement));
  };
  return (
    <div className={`docs-entry ${e.status}`} onMouseEnter={() => e.links?.length && onHighlight(e.links)}>
      <div className="row">
        <span className="pill mono">{e.id}</span>
        <span className={`pill ${cls}`}>{e.status}{e.supersededBy ? ` → ${e.supersededBy}` : ""}</span>
      </div>
      <p>{e.statement}</p>
      {e.fields && Object.keys(e.fields).length > 0 && (
        <p className="muted small">{Object.entries(e.fields).map(([k, v]) => `${k}: ${v}`).join(" · ")}</p>
      )}
      <p className="muted small">{src}{e.citedBy?.length ? ` · cited by ${e.citedBy.join(", ")}` : ""}</p>
      <div className="row">
        {e.status === "open" && <button className="small ok-outline" onClick={() => act(api.entryStatus(e.id, "confirmed"))}>Confirm</button>}
        {(e.status === "open" || e.status === "confirmed") && <button className="small" onClick={reject}>Reject…</button>}
        {(e.type === "RISK" || e.type === "Q") && (e.status === "open" || e.status === "confirmed") && <button className="small" onClick={() => act(api.fixEntry(e.id))}>Draft a fix</button>}
      </div>
    </div>
  );
}

// Entries the Assistant proposed in chat: nothing reaches the Register until you accept it.
export function ProposedEntries({ items, onError, onChanged }: { items: ProposedEntry[]; onError: (e: unknown) => void; onChanged: () => void }) {
  return (
    <div className="docs-proposed" role="region" aria-label="Proposed Register entries">
      <p className="small"><strong>The Assistant proposes {items.length === 1 ? "a Register entry" : `${items.length} Register entries`}</strong> <span className="muted">from your chat. Nothing is recorded until you accept.</span></p>
      {items.map((p) => <ProposedCard key={p.id} p={p} onError={onError} onChanged={onChanged} />)}
    </div>
  );
}

function ProposedCard({ p, onError, onChanged }: { p: ProposedEntry; onError: (e: unknown) => void; onChanged: () => void }) {
  const [text, setText] = useState(p.entry.statement);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const decide = (accept: boolean) => {
    setBusy(true);
    api.decideProposed(p.id, accept, accept && text !== p.entry.statement ? text : "").then(onChanged, onError).finally(() => setBusy(false));
  };
  const fields = Object.entries(p.entry.fields ?? {});
  return (
    <div className="docs-entry open">
      <div className="row"><span className="pill">{TYPE_NAMES[p.entry.type]}</span>{p.replaces && <span className="pill warn">replaces {p.replaces}</span>}</div>
      {editing ? <textarea rows={4} value={text} onChange={(e) => setText(e.target.value)} aria-label="Statement" /> : <p className="docs-clamp" title={text}>{text}</p>}
      {fields.length > 0 && <p className="muted small">{fields.map(([k, v]) => `${k}: ${v}`).join(" · ")}</p>}
      {p.entry.source.text && <p className="muted small" title={p.entry.source.text}>From: “{p.entry.source.text.length > 120 ? p.entry.source.text.slice(0, 120) + "…" : p.entry.source.text}”</p>}
      <div className="row">
        <button className="small ok-outline" disabled={busy || !text.trim()} onClick={() => decide(true)}>Accept</button>
        {!editing && <button className="small" disabled={busy} onClick={() => setEditing(true)}>Edit</button>}
        <button className="small" disabled={busy} onClick={() => decide(false)}>Reject</button>
      </div>
    </div>
  );
}
