// The Create wizard (Documents panel): walks through every question a document still
// needs, one per screen, then a review. Finish records all the answers in the Register at
// once (the shared facts every document and the Assistant use) and writes the document.
// A document whose prerequisites aren't written is greyed out in the list, so it never
// gets here.
import { useEffect, useMemo, useState } from "react";
import { api } from "../api";
import type { EntryType, PipelineStep, PipelineView, StepQuestion } from "../types";

const ENTRY_WORD: Record<EntryType, string> = {
  REQ: "a requirement", CON: "a constraint", ASM: "an assumption", NG: "a non-goal", DEC: "a decision", RISK: "a risk", Q: "an open question",
};

type Page = { step: PipelineStep; q: StepQuestion };
type Draft = { values: string[]; unknown: boolean };

/** The questions Create asks: unanswered ones (or, when editing, all of one document's). */
export function pagesFor(view: PipelineView, id: string, all: boolean): Page[] {
  return view.steps.filter((s) => s.id === id).flatMap((step) => step.questions.filter((q) => all || !step.answers?.[q.id]).map((q) => ({ step, q })));
}

export function CreateWizard({ view, id, all, onClose, onDone, onError }: {
  view: PipelineView; id: string; all: boolean;
  onClose: () => void; onDone: () => void; onError: (e: unknown) => void;
}) {
  const target = view.steps.find((s) => s.id === id)!;
  const chain = [target];
  const pages = useMemo(() => pagesFor(view, id, all), []); // eslint-disable-line react-hooks/exhaustive-deps
  const key = (p: Page) => `${p.step.id}·${p.q.id}`;
  const [drafts, setDrafts] = useState<Record<string, Draft>>(() => {
    const d: Record<string, Draft> = {};
    for (const p of pages) {
      const prev = p.step.answers?.[p.q.id];
      if (prev) d[key(p)] = { values: prev.values ?? [], unknown: !!prev.unknown };
    }
    return d;
  });
  const [i, setI] = useState(0);
  const [busy, setBusy] = useState(false);
  const [dirty, setDirty] = useState(false);
  const review = i >= pages.length;
  const page = pages[Math.min(i, pages.length - 1)];

  const close = () => {
    if (dirty && !window.confirm("Close without saving your answers?")) return;
    onClose();
  };
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && close();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  const set = (p: Page, d: Draft) => {
    setDrafts((x) => ({ ...x, [key(p)]: d }));
    setDirty(true);
  };
  const filled = (p: Page) => {
    const d = drafts[key(p)];
    return !!d && (d.unknown || d.values.some((v) => v.trim()));
  };
  const missingRequired = pages.filter((p) => p.q.blocking && (!filled(p) || drafts[key(p)]?.unknown));

  const finish = (write: boolean) => {
    setBusy(true);
    const answers = pages.filter(filled).map((p) => {
      const d = drafts[key(p)];
      const values = d.unknown ? [] : d.values.map((v) => v.trim()).filter(Boolean);
      return { step: p.step.id, question: p.q.id, values, unknown: d.unknown };
    });
    const done = write
      ? api.createDocument(id, answers)
      : api.saveAnswers(id, answers);
    done.then(onDone, (e) => { onError(e); setBusy(false); });
  };

  const title = all ? `Answers for the ${target.title}` : `${target.versions?.length ? "Update" : "Create"} the ${target.title}`;

  return (
    <div className="modal-back" onClick={close}>
      <div className="modal docs-create" onClick={(e) => e.stopPropagation()} role="dialog" aria-label={title}>
        <div className="docs-create-head">
          <h2>{title}</h2>
          <button className="ghost" onClick={close} aria-label="Close">✕</button>
        </div>
        {pages.length > 0 && (
          <div className="docs-dots" aria-hidden>
            {pages.map((p, k) => <button key={key(p)} className={`docs-dot ${k === i ? "on" : ""} ${filled(p) ? "done" : ""}`} onClick={() => setI(k)} tabIndex={-1} />)}
            <button className={`docs-dot review ${review ? "on" : ""}`} onClick={() => setI(pages.length)} tabIndex={-1} />
            <span className="muted small">{review ? "Review" : `${i + 1} of ${pages.length}`}</span>
          </div>
        )}

        {!review && page && (
          <Question key={key(page)} page={page} draft={drafts[key(page)]} onChange={(d) => set(page, d)} />
        )}

        {review && (
          <div className="docs-review">
            {pages.length === 0 && <p>Everything this document needs is already answered.</p>}
            {chain.map((s) => {
              const ps = pages.filter((p) => p.step.id === s.id);
              if (!ps.length) return null;
              return (
                <section key={s.id}>
                  <h4>For the {s.title}</h4>
                  {ps.map((p) => {
                    const d = drafts[key(p)];
                    return (
                      <button key={key(p)} className="docs-review-row" onClick={() => setI(pages.indexOf(p))}>
                        <span className="docs-review-q">{p.q.label}{p.q.blocking && <span className="pill warn">required</span>}</span>
                        <span className={d && (d.unknown || d.values.length) ? "" : "muted"}>
                          {!d || (!d.unknown && !d.values.some((v) => v.trim())) ? "Not answered" : d.unknown ? "Unknown: the document states what it assumes" : d.values.join(", ")}
                        </span>
                      </button>
                    );
                  })}
                </section>
              );
            })}
            <p className="small muted">Answers go into the Register: the shared facts every document, and the Assistant when it changes the diagram, work from.</p>
            {missingRequired.length > 0 && <p className="docs-warn">Still needed: {missingRequired.map((p) => p.q.label).join(", ")}.</p>}
          </div>
        )}

        <div className="row docs-create-foot">
          <button disabled={i === 0 || busy} onClick={() => setI(i - 1)}>Back</button>
          <span className="spacer" />
          {!review && page && !page.q.blocking && (
            <button disabled={busy} onClick={() => { set(page, { values: [], unknown: true }); setI(i + 1); }}
              title="The document states what it assumes instead, as an open assumption">I don't know</button>
          )}
          {!review && <button className="primary" disabled={busy || (page?.q.blocking && !filled(page))} onClick={() => setI(i + 1)}>Next</button>}
          {review && all && <button disabled={busy} onClick={() => finish(false)}>Save answers</button>}
          {review && (
            <button className="primary" disabled={busy || missingRequired.length > 0} onClick={() => finish(true)}>
              {busy ? "Starting…" : `Finish and ${target.versions?.length ? "update" : "write"} it`}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

function Question({ page, draft, onChange }: { page: Page; draft?: Draft; onChange: (d: Draft) => void }) {
  const { q } = page;
  const values = draft && !draft.unknown ? draft.values : [];
  const text = values.join("\n");
  const pick = (v: string) => {
    const on = values.includes(v);
    onChange({ unknown: false, values: q.kind === "choice" ? [v] : on ? values.filter((x) => x !== v) : [...values, v] });
  };
  return (
    <div className="docs-q docs-create-q">
      <label>
        {q.label}
        {q.blocking && <span className="pill warn">required</span>}
        {draft?.unknown && <span className="pill">unknown</span>}
      </label>
      <p>{q.question}</p>
      {(q.kind === "text" || q.kind === "list") && (
        <textarea autoFocus rows={q.kind === "list" ? 5 : 3} placeholder={q.kind === "list" ? "One per line" : ""} value={text}
          onChange={(e) => onChange({ unknown: false, values: q.kind === "list" ? e.target.value.split("\n") : [e.target.value] })} />
      )}
      {(q.kind === "choice" || q.kind === "multi") && (
        <div className="docs-choices">
          {q.values!.map((v) => <button key={v} className={values.includes(v) ? "on" : ""} onClick={() => pick(v)}>{v}</button>)}
        </div>
      )}
      <span className="muted small">{q.creates ? `Recorded in the Register as ${ENTRY_WORD[q.creates]}${q.kind === "list" ? " per line" : ""}.` : "Used for the document's details only."}</span>
    </div>
  );
}
