// The Documents tab (wayfinder/doc-pipeline ticket 010, variant A): the Document Pipeline as a
// stepper of Steps, each with its wizard and document, and the Register beside it.
import { useEffect, useState } from "react";
import { api, download } from "../api";
import { Markdown } from "../Markdown";
import type { EntryType, PipelineStep, PipelineView, ProposedEntry, RegisterEntry, StepQuestion } from "../types";

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
  const [tab, setTab] = useState<"pipeline" | "register">("pipeline");
  const [open, setOpen] = useState<string | null>(null);
  // Expand the first unfinished Step once; after that the expanded Step only changes when you choose.
  useEffect(() => {
    if (open === null && view) setOpen(view.steps.find((s) => s.status.kind !== "current" && s.status.kind !== "waiting")?.id ?? view.steps[0].id);
  }, [view, open]);
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

  return (
    <div className="docs">
      <div className="docs-seg">
        <button className={tab === "pipeline" ? "on" : ""} onClick={() => setTab("pipeline")}>Pipeline</button>
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

      {tab === "pipeline" && (
        <ol className="docs-stepper">
          {view.steps.map((s) => {
            const isOpen = open === s.id;
            return (
              <li key={s.id} className={`docs-step ${s.status.kind}`}>
                <button className="docs-step-head" onClick={() => setOpen(isOpen ? "" : s.id)} aria-expanded={isOpen}>
                  <span className="docs-step-title">{s.title}</span>
                  <StatusChip step={s} />
                </button>
                {isOpen && (
                  <StepBody step={s} view={view} onError={onError} onChanged={onChanged}
                    onRead={(n) => api.docVersion(s.id, n).then((r) => setReading({ title: `${s.title} v${n}`, markdown: r.markdown }), onError)}
                    onRun={() => act(api.runStep(s.id))} />
                )}
              </li>
            );
          })}
          <li className={`docs-step ${view.arc42?.written && !view.arc42.stale ? "current" : "waiting"}`}>
            <button className="docs-step-head" onClick={() => setOpen(open === "arc42" ? "" : "arc42")} aria-expanded={open === "arc42"}>
              <span className="docs-step-title">arc42 architecture document</span>
              <span className={`pill ${!view.arc42 ? "" : view.arc42.stale ? "bad" : view.arc42.written ? "ok" : "accent"}`}>
                {!view.arc42 ? "Needs a Production diagram" : view.arc42.stale ? `Text out of date (v${view.arc42.version})` : view.arc42.written ? `Written for v${view.arc42.version}` : "Ready to export"}
              </span>
            </button>
            {open === "arc42" && (
              <div className="docs-step-body">
                <p className="muted small">arc42 template (our wording, credited) · from the Production diagram, Gap Report and the Register: its requirements, assumptions, decisions (as ADRs) and risks are the same entries the other documents cite.</p>
                <div className="row">
                  <button className="primary" disabled={!view.arc42} onClick={() => onArc42("docx")}>Export Word</button>
                  <button disabled={!view.arc42} onClick={() => onArc42("pdf")}>Export PDF</button>
                </div>
              </div>
            )}
          </li>
        </ol>
      )}

      {tab === "register" && (
        <div className="docs-register">
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

function StatusChip({ step }: { step: PipelineStep }) {
  const cls = { waiting: "", needs: "warn", ready: "accent", writing: "accent", current: "ok", stale: "bad" }[step.status.kind];
  return <span className={`pill ${cls}`} title={step.status.reasons?.join("\n")}>{step.status.text}</span>;
}

function StepBody({ step, view, onError, onChanged, onRead, onRun }: {
  step: PipelineStep; view: PipelineView; onError: (e: unknown) => void; onChanged: () => void; onRead: (n: number) => void; onRun: () => void;
}) {
  const answers = step.answers ?? {};
  const firstOpen = Math.max(0, step.questions.findIndex((q) => !answers[q.id]));
  const [i, setI] = useState(firstOpen);
  const [editing, setEditing] = useState(step.status.kind === "needs");
  useEffect(() => setI(firstOpen), [step.id]); // eslint-disable-line react-hooks/exhaustive-deps
  const versions = step.versions ?? [];
  const blocked = (step.status.blocking?.length ?? 0) > 0;
  const deps = [...(step.requires ?? []), ...(step.uses ?? [])].map((id) => view.steps.find((s) => s.id === id)?.title ?? id);
  const q = step.questions[Math.min(i, step.questions.length - 1)];
  return (
    <div className="docs-step-body">
      <p className="muted small">{step.template}{deps.length ? ` · builds on ${deps.join(", ")}` : ""} · from the {step.diagram === "rough" ? "Rough Draft" : "Production diagram"}</p>
      {step.status.kind === "writing" && <div className="docs-progress"><span className="spinner" aria-hidden /> The Assistant is writing it. Its progress shows in the Assistant tab.</div>}
      {step.status.kind === "stale" && <ul className="docs-reasons">{step.status.reasons?.map((r) => <li key={r}>{r}</li>)}</ul>}
      {step.questions.length > 0 && step.status.kind !== "waiting" && (editing ? (
        <div className="docs-wizard">
          <div className="docs-dots">
            {step.questions.map((x, k) => <button key={x.id} className={`docs-dot ${k === i ? "on" : ""} ${answers[x.id] ? "done" : ""}`} onClick={() => setI(k)} aria-label={x.label} />)}
            <span className="muted small">{i + 1} of {step.questions.length}</span>
            <span className="spacer" />
            <button className="ghost small" onClick={() => setEditing(false)}>Hide</button>
          </div>
          <QuestionForm key={`${step.id}:${q.id}`} step={step} q={q} onError={onError}
            onSaved={() => { onChanged(); if (i + 1 < step.questions.length) setI(i + 1); else setEditing(false); }} />
        </div>
      ) : (
        <button className="ghost small docs-answers-link" onClick={() => setEditing(true)}>
          {step.status.unanswered > 0 ? `Answer ${step.status.unanswered} question${step.status.unanswered === 1 ? "" : "s"}` : "Review answers"}
        </button>
      ))}
      <div className="row">
        <button className="primary" disabled={step.status.kind === "waiting" || step.status.kind === "writing" || blocked} onClick={onRun}
          title={blocked ? `Answer ${step.status.blocking!.join(" and ")} first` : ""}>
          {versions.length ? "Rewrite it" : "Write it"}
        </button>
        {versions.length > 0 && <button onClick={() => onRead(versions[versions.length - 1])}>Read v{versions[versions.length - 1]}</button>}
        {versions.length > 1 && (
          <select className="docs-older" value="" onChange={(e) => e.target.value && onRead(+e.target.value)} aria-label="Older versions">
            <option value="">Older…</option>
            {versions.slice(0, -1).reverse().map((n) => <option key={n} value={n}>v{n}</option>)}
          </select>
        )}
      </div>
    </div>
  );
}

function QuestionForm({ step, q, onSaved, onError }: { step: PipelineStep; q: StepQuestion; onSaved: () => void; onError: (e: unknown) => void }) {
  const prev = step.answers?.[q.id];
  const [text, setText] = useState(prev?.values?.join("\n") ?? "");
  const [picked, setPicked] = useState<string[]>(prev?.values ?? []);
  const [busy, setBusy] = useState(false);
  const values = q.kind === "choice" || q.kind === "multi" ? picked : [text];
  const filled = q.kind === "choice" || q.kind === "multi" ? picked.length > 0 : text.trim().length > 0;
  const save = (unknown: boolean) => {
    setBusy(true);
    api.answer(step.id, q.id, unknown ? [] : values, unknown).then(onSaved, onError).finally(() => setBusy(false));
  };
  return (
    <div className="docs-q">
      <label>
        {q.label}
        {q.blocking && <span className="pill warn">required</span>}
        {prev && <span className="pill ok">{prev.unknown ? "unknown → assumption" : "answered"}</span>}
      </label>
      <p>{q.question}</p>
      {(q.kind === "text" || q.kind === "list") && (
        <textarea rows={q.kind === "list" ? 4 : 3} placeholder={q.kind === "list" ? "One per line" : ""} value={text} onChange={(e) => setText(e.target.value)} />
      )}
      {(q.kind === "choice" || q.kind === "multi") && (
        <div className="docs-choices">
          {q.values!.map((v) => {
            const on = picked.includes(v);
            return <button key={v} className={on ? "on" : ""} onClick={() => setPicked(q.kind === "choice" ? [v] : on ? picked.filter((x) => x !== v) : [...picked, v])}>{v}</button>;
          })}
        </div>
      )}
      <div className="row">
        <button className="primary" disabled={!filled || busy} onClick={() => save(false)}>Save and next</button>
        {!q.blocking && <button disabled={busy} onClick={() => save(true)} title="The Assistant states what it assumes instead, as an open assumption">Unknown</button>}
        <span className="muted small">{q.creates ? `Becomes ${q.creates} entries (confirmed)` : "For document control only"}</span>
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
