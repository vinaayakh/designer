// The Design Brief interview (ticket 009, variant B): a local wizard, one
// question per screen, with the Brief always visible as the AI will read it.
// The 3 required questions come first and gate the Rough Draft.
import { useEffect, useMemo, useState } from "react";
import { api } from "./api";
import type { Brief, BriefField, BriefSchema } from "./types";

type Screen = { type: "q"; key: string; fields: BriefField[] } | { type: "stop"; key: "stop" } | { type: "notes"; key: "notes" };

export function screensOf(schema: BriefSchema): Screen[] {
  const out: Screen[] = [];
  const seen = new Set<string>();
  const req = schema.fields.filter((f) => f.required);
  const opt = schema.fields.filter((f) => !f.required);
  for (const f of [...req, ...opt]) {
    if (seen.has(f.id)) continue;
    const together = f.screen ? schema.fields.filter((x) => x.screen === f.screen) : [f];
    together.forEach((x) => seen.add(x.id));
    out.push({ type: "q", key: f.screen ?? f.id, fields: together });
    if (f === req[req.length - 1]) out.push({ type: "stop", key: "stop" });
  }
  out.push({ type: "notes", key: "notes" });
  return out;
}

export function isAnswered(b: Brief, f: BriefField) {
  return f.multi ? (b.compliance?.length ?? 0) > 0 : !!b.answers[f.id] && b.answers[f.id] !== "unknown";
}

export function Wizard({ schema, initial, onDone }: { schema: BriefSchema; initial: Brief; onDone: (b: Brief) => void }) {
  const screens = useMemo(() => screensOf(schema), [schema]);
  const required = schema.fields.filter((f) => f.required);
  const [brief, setBrief] = useState<Brief>(initial);
  const [compact, setCompact] = useState("");
  const [error, setError] = useState("");
  const firstOpen = Math.max(0, screens.findIndex((s) => s.type === "q" && s.fields.some((f) => f.required && !isAnswered(initial, f))));
  const [i, setI] = useState(firstOpen);
  const [help, setHelp] = useState(false);

  const save = async (b: Brief) => {
    setBrief(b);
    try {
      const r = await api.saveBrief(b);
      setCompact(r.compact);
      setError("");
    } catch (e) {
      setError(String((e as Error).message));
    }
  };
  useEffect(() => {
    void save(initial);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const missing = required.filter((f) => !isAnswered(brief, f));
  const screen = screens[i];
  const next = () => setI((x) => Math.min(screens.length - 1, x + 1));
  const back = () => setI((x) => Math.max(0, x - 1));
  const finish = () => {
    if (missing.length === 0) onDone(brief);
  };

  const pick = (f: BriefField, v: string) => {
    if (f.multi) {
      let cur = brief.compliance ?? [];
      if (v === f.exclusive) cur = cur.includes(v) ? [] : [v];
      else cur = cur.includes(v) ? cur.filter((x) => x !== v) : [...cur.filter((x) => x !== f.exclusive), v];
      void save({ ...brief, compliance: cur });
      return;
    }
    void save({ ...brief, answers: { ...brief.answers, [f.id]: v } });
    if (!f.multi && screen.type === "q" && screen.fields.length === 1) setTimeout(next, 120);
  };

  const body = () => {
    if (screen.type === "stop") {
      return (
        <>
          <h1>That's enough to start.</h1>
          <p className="lede">The required answers are in. You can start sketching now, or answer a few more questions so the design fits better. Anything you skip stays <i>unknown</i>, and the AI will say what it assumed.</p>
          <div className="row" style={{ marginTop: 20 }}>
            <button className="primary" disabled={missing.length > 0} onClick={finish}>Start sketching</button>
            <button onClick={next}>Answer more</button>
          </div>
        </>
      );
    }
    if (screen.type === "notes") {
      return (
        <>
          <h1>{schema.notes.question}</h1>
          <textarea rows={4} maxLength={schema.notes.max} value={brief.notes} onChange={(e) => setBrief({ ...brief, notes: e.target.value })} onBlur={() => void save(brief)} placeholder="Optional" />
          <div className="small muted">{brief.notes.length}/{schema.notes.max}</div>
          <div className="row" style={{ marginTop: 20 }}>
            <button onClick={back}>Back</button>
            <button className="primary" disabled={missing.length > 0} onClick={() => save(brief).then(finish)}>Start sketching</button>
          </div>
        </>
      );
    }
    return (
      <>
        {screen.fields.map((f) => (
          <div key={f.id} style={{ marginBottom: 22 }}>
            <div className="step-label">{f.label}{f.required ? " · required" : ""}</div>
            <h1>{f.question}</h1>
            {f.help && (
              <p>
                <button className="link" onClick={() => setHelp((h) => !h)}>I'm not sure — explain</button>
                {help && <span className="muted"><br />{f.help}</span>}
              </p>
            )}
            <div className="choices">
              {f.values.map((v) => {
                const on = f.multi ? brief.compliance?.includes(v.id) : brief.answers[f.id] === v.id;
                return (
                  <button key={v.id} className={`choice ${f.multi ? "multi" : ""} ${on ? "on" : ""}`} onClick={() => pick(f, v.id)}>
                    <b>{v.label}</b>
                    {v.hint && <span className="muted small">{v.hint}</span>}
                  </button>
                );
              })}
            </div>
          </div>
        ))}
        <div className="row">
          {i > 0 && <button onClick={back}>Back</button>}
          {screen.fields.every((f) => f.required) ? (
            <button className="primary" disabled={!screen.fields.every((f) => isAnswered(brief, f))} onClick={next}>Next</button>
          ) : (
            <>
              <button onClick={() => { void save({ ...brief, answers: { ...brief.answers, ...Object.fromEntries(screen.fields.filter((f) => !isAnswered(brief, f)).map((f) => [f.id, "unknown"])) } }); next(); }}>
                Don't know — skip
              </button>
              {screen.fields.length > 1 && <button className="primary" onClick={next}>Next</button>}
              <span className="spacer" />
              <button className="link" disabled={missing.length > 0} onClick={finish}>Skip the rest</button>
            </>
          )}
        </div>
      </>
    );
  };

  return (
    <div className="wizard">
      <div className="wizard-main">
        <div className="bar"><div style={{ width: `${((i + 1) / screens.length) * 100}%` }} /></div>
        {body()}
        {error && <p className="error-text">{error}</p>}
      </div>
      <aside className="wizard-side">
        <div className="eyebrow">Design Brief</div>
        <p className="small muted" style={{ margin: "6px 0 14px" }}>Exactly what the AI will read. Unanswered questions stay <i>unknown</i>.</p>
        <div className="brief-pre">{compact || "…"}</div>
        {missing.length > 0 && <p className="small muted" style={{ marginTop: 12 }}>{missing.length} required answer{missing.length > 1 ? "s" : ""} missing: {missing.map((f) => f.label).join(", ")}</p>}
      </aside>
    </div>
  );
}

/** Editing the Brief later, from Project settings. */
export function BriefEditor({ schema, brief, onSaved, cloudLocked }: { schema: BriefSchema; brief: Brief; onSaved: (b: Brief) => void; cloudLocked: boolean }) {
  const [b, setB] = useState(brief);
  const [msg, setMsg] = useState("");
  const set = async (nb: Brief) => {
    setB(nb);
    try {
      await api.saveBrief(nb);
      onSaved(nb);
      setMsg("Saved. The Assistant gets the change with your next message.");
    } catch (e) {
      setMsg((e as Error).message);
    }
  };
  return (
    <div>
      {schema.fields.map((f) => (
        <div className="field" key={f.id}>
          <label>{f.label}{f.required ? " *" : ""}</label>
          {f.multi ? (
            <div className="row wrap">
              {f.values.map((v) => (
                <label key={v.id} className="check">
                  <input type="checkbox" checked={b.compliance?.includes(v.id) ?? false} onChange={() => {
                    let cur = b.compliance ?? [];
                    if (v.id === f.exclusive) cur = cur.includes(v.id) ? [] : [v.id];
                    else cur = cur.includes(v.id) ? cur.filter((x) => x !== v.id) : [...cur.filter((x) => x !== f.exclusive), v.id];
                    if (cur.length) void set({ ...b, compliance: cur });
                  }} />
                  {v.label}
                </label>
              ))}
            </div>
          ) : (
            <select value={b.answers[f.id] || ""} disabled={f.id === "cloud" && cloudLocked} title={f.id === "cloud" && cloudLocked ? "The target cloud can't change after Conversion" : undefined}
              onChange={(e) => void set({ ...b, answers: { ...b.answers, [f.id]: e.target.value } })}>
              {f.required ? <option value="" disabled>— choose —</option> : <option value="unknown">Unknown</option>}
              {f.values.map((v) => <option key={v.id} value={v.id}>{v.label}</option>)}
            </select>
          )}
        </div>
      ))}
      <div className="field">
        <label>{schema.notes.label}</label>
        <textarea rows={3} maxLength={schema.notes.max} value={b.notes} onChange={(e) => setB({ ...b, notes: e.target.value })} onBlur={() => void set(b)} />
      </div>
      {msg && <p className="small muted">{msg}</p>}
    </div>
  );
}
