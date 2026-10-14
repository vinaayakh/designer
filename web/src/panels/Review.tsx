// Reviewing a Proposal (ticket 010, revised): step through one change at a
// time. Only what you've accepted and the change under review are drawn.
// Rejecting a change asks the Assistant to rework the rest (it may ask you a
// question first). Full review shows everything with one Accept.
import { useState } from "react";
import type { Change, Pending } from "../types";
import type { OverlayStatus } from "../production/render";
import { Markdown } from "../Markdown";

/** index is a step (see reviewSteps); decisions are per change. */
export type ReviewState = { mode: "step" | "full"; index: number; decisions: Record<string, "accepted" | "skipped"> };

/** One review step: a change, plus the arrows that only make sense with it. */
export type Step = { id: string; changes: Change[] };

/**
 * Groups a Proposal into review steps: every change is its own step, except
 * an arrow that needs a new Node or Group, which joins the step of the last
 * thing it needs, so a new Node arrives with all its arrows.
 */
export function reviewSteps(changes: Change[]): Step[] {
  const steps: Step[] = [];
  const owner = new Map<string, number>();
  const joins = (c: Change) => c.op.endsWith("_edge") && (c.needs?.length ?? 0) > 0;
  for (const c of changes) {
    if (joins(c)) continue;
    owner.set(c.id, steps.length);
    steps.push({ id: c.id, changes: [c] });
  }
  for (const c of changes) {
    if (!joins(c)) continue;
    const at = Math.max(...c.needs!.map((n) => owner.get(n) ?? -1));
    if (at >= 0) steps[at].changes.push(c);
    else steps.push({ id: c.id, changes: [c] });
    owner.set(c.id, at >= 0 ? at : steps.length - 1);
  }
  return steps;
}

// A step is done once each of its changes is accepted, or skipped (Sim Mode Proposals: nobody revises them).
const stepDone = (s: Step, d: ReviewState["decisions"]) => s.changes.every((c) => d[c.id] === "accepted" || d[c.id] === "skipped");

/** Where review starts for a (new revision of a) Proposal: accepted changes carried over, first open step next. */
export function reviewStart(p: Pending): ReviewState {
  const decisions: ReviewState["decisions"] = {};
  for (const id of p.accepted ?? []) decisions[id] = "accepted";
  const steps = reviewSteps(p.payload.changes);
  const first = steps.findIndex((s) => !stepDone(s, decisions));
  return { mode: "step", index: first === -1 ? steps.length : first, decisions };
}

export function changeStatus(ch: Change): OverlayStatus {
  if (ch.op.startsWith("add_")) return "add";
  if (ch.op.startsWith("remove_")) return "remove";
  if (ch.op === "move_node") return "move";
  return "change";
}
const SIGN: Record<OverlayStatus, string> = { add: "+", remove: "−", change: "~", move: "↦" };

export function describe(ch: Change, label: (id?: string) => string): string {
  const setText = (s?: Record<string, unknown>) =>
    Object.entries(s ?? {}).map(([k, v]) => `${k}: ${v === null ? "removed" : Array.isArray(v) ? v.join(", ") : String(v)}`).join(", ");
  switch (ch.op) {
    case "add_node": {
      const facts = [ch.kind, ch.service ?? ch.service_name, ch.classes?.join(",")].filter(Boolean).join(" · ");
      return `Add “${ch.label}”${facts ? ` (${facts})` : ""}${ch.parentId ? ` in ${label(ch.parentId)}` : ""}`;
    }
    case "add_group":
      return `Add ${ch.kind ?? "group"}${ch.tier ? ` (${ch.tier})` : ""} “${ch.label}”${ch.parentId ? ` in ${label(ch.parentId)}` : ""}`;
    case "add_edge":
      return `Connect ${label(ch.fromId)} → ${label(ch.toId)}${ch.style ? ` (${[ch.style, ch.protocol].filter(Boolean).join(", ")})` : ""}`;
    case "update_node":
    case "update_group":
      return `Change ${label(ch.elementId)}: ${setText(ch.set)}${ch.parentId !== undefined ? `${ch.set && Object.keys(ch.set).length ? ", " : ""}moved into ${ch.parentId ? label(ch.parentId) : "the top level"}` : ""}`;
    case "update_edge":
      return `Change ${label(ch.fromId)} → ${label(ch.toId)}: ${setText(ch.set)}`;
    case "move_node":
      return `Move ${label(ch.elementId)} into ${ch.parentId ? label(ch.parentId) : "the top level"}`;
    case "remove_node":
    case "remove_group":
      return `Remove ${label(ch.elementId)}`;
    case "remove_edge":
      return `Remove ${label(ch.fromId)} → ${label(ch.toId)}`;
  }
  return ch.op;
}

function ChangeRow({ c, label, status, active, onClick }: { c: Change; label: (id?: string) => string; status?: string; active?: boolean; onClick?: () => void }) {
  const st = changeStatus(c);
  return (
    <div className={`card ${onClick ? "clickable" : ""} ${active ? "active" : ""}`} onClick={onClick}>
      <div className="change">
        <span className={`op ${st}`}>{SIGN[st]}</span>
        <div>
          <div className="small">{describe(c, label)}</div>
          {c.why && <div className="why">{c.why}</div>}
        </div>
        {status !== undefined && <span className={`pill ${status === "accepted" ? "ok" : ""}`}>{status}</span>}
      </div>
    </div>
  );
}

export function Review({ pending, state, setState, label, onFocus, onAccept, onReject, onReply, onRejectChange, onAnswer, busy }: {
  pending: Pending;
  state: ReviewState;
  setState: (s: ReviewState) => void;
  label: (id?: string) => string;
  onFocus: (ch: Change) => void;
  onAccept: (accepted: string[]) => void;
  onReject: () => void;
  onReply: (text: string) => void;
  onRejectChange: (change: string, accepted: string[], reason: string) => void;
  onAnswer: (text: string) => void;
  busy: boolean;
}) {
  const [reply, setReply] = useState("");
  const [replying, setReplying] = useState(false);
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState("");
  const [answer, setAnswer] = useState("");
  const changes = pending.payload.changes;
  const steps = reviewSteps(changes);
  const { mode, index, decisions } = state;
  const accepted = changes.filter((c) => decisions[c.id] === "accepted").map((c) => c.id);
  const step = index < steps.length ? steps[index] : undefined;
  const cur = step?.changes[0];
  const doneSteps = steps.filter((s) => stepDone(s, decisions)).length;
  const rejections = pending.rejections ?? [];
  const fromSim = pending.author === "user";

  const header = (
    <>
      <div className="eyebrow">{fromSim ? (pending.source?.startsWith("Scenario") ? "Your changes from Sim Mode · " : "Your changes · ") : ""}Proposal {pending.id}{pending.revision > 1 ? ` · revision ${pending.revision}` : ""} · {pending.view === "rough" ? "Rough Draft" : `Production v${pending.baseVersion}`}</div>
      <h3 style={{ marginTop: 8 }}>{pending.payload.title}</h3>
      {pending.payload.summary && <p className="lede">{pending.payload.summary}</p>}
      {fromSim && pending.source && <p className="small muted">From {pending.source}. Skip a change to leave it out{pending.source.startsWith("Scenario") ? "; it stays in the Scenario" : ""}.</p>}
      {pending.payload.addresses?.length ? <p className="small muted">Fixes <span className="mono">{pending.payload.addresses.map((a) => a.replace(/^gap:/, "")).join(", ")}</span></p> : null}
    </>
  );
  const rejectedList = rejections.length > 0 && (
    <>
      <h4>Rejected</h4>
      {rejections.map((r, i) => (
        <div key={i} className="card" style={{ opacity: 0.7 }}>
          <div className="small" style={{ textDecoration: "line-through", textDecorationColor: "var(--ink-4)" }}>{describe(r.change, label)}</div>
          {r.reason && <div className="why">“{r.reason}”</div>}
        </div>
      ))}
    </>
  );
  const footer = (
    <div className="row" style={{ marginTop: 16 }}>
      {!fromSim && <button onClick={() => setReplying((r) => !r)} disabled={busy}>Reply…</button>}
      <span className="spacer" />
      <button className="danger" onClick={onReject} disabled={busy}>Reject Proposal</button>
    </div>
  );

  // The Assistant is reworking the rest of the Proposal.
  if (pending.awaitingRevision) {
    return (
      <div className="panel">
        {header}
        <div className="callout">
          <div className="row"><span className="dot busy" /><b className="small">Revising the remaining changes…</b></div>
          {pending.reply && <div className="small muted" style={{ marginTop: 6 }}>You replied: “{pending.reply}”</div>}
          {accepted.length > 0 && <div className="small muted" style={{ marginTop: 6 }}>Your {accepted.length} accepted change{accepted.length === 1 ? "" : "s"} will be kept.</div>}
        </div>
        {rejectedList}
      </div>
    );
  }

  // The Assistant needs more information before revising.
  if (pending.question) {
    return (
      <div className="panel">
        {header}
        <h4>The Assistant asks</h4>
        <Markdown text={pending.question} className="md chat-md" />
        <textarea rows={3} value={answer} onChange={(e) => setAnswer(e.target.value)} placeholder="Your answer" style={{ marginTop: 10 }} />
        <div className="row" style={{ marginTop: 8 }}>
          <span className="spacer" />
          <button className="primary" disabled={!answer.trim() || busy} onClick={() => { onAnswer(answer.trim()); setAnswer(""); }}>Answer</button>
        </div>
        {rejectedList}
        {footer}
      </div>
    );
  }

  return (
    <div className="panel">
      {header}
      <div className="tabs" style={{ marginBottom: 12 }}>
        <button className={mode === "step" ? "on" : ""} onClick={() => { setState({ ...state, mode: "step" }); if (cur) onFocus(cur); }}>Step through</button>
        <button className={mode === "full" ? "on" : ""} onClick={() => setState({ ...state, mode: "full" })}>Full review</button>
      </div>

      {mode === "step" && cur && (
        <>
          <div className="row small muted"><span>Step {index + 1} of {steps.length}</span><span className="spacer" /><span>{accepted.length} of {changes.length} changes accepted</span></div>
          <div className="progress"><div style={{ width: `${(doneSteps / steps.length) * 100}%` }} /></div>
          <ChangeRow c={cur} label={label} active />
          {step!.changes.length > 1 && (
            <div style={{ marginLeft: 26 }}>
              <div className="eyebrow" style={{ margin: "8px 0 0" }}>With its connections</div>
              {step!.changes.slice(1).map((c) => <ChangeRow key={c.id} c={c} label={label} />)}
            </div>
          )}
          {!rejecting ? (
            <div className="row" style={{ marginTop: 10 }}>
              <span className="spacer" />
              {fromSim ? (
                <button disabled={busy} onClick={() => {
                  const nd = { ...decisions };
                  for (const c of step!.changes) nd[c.id] = "skipped";
                  const next = steps.findIndex((s) => !stepDone(s, nd));
                  setState({ ...state, decisions: nd, index: next === -1 ? steps.length : next });
                  if (next !== -1) onFocus(steps[next].changes[0]);
                }}>Skip</button>
              ) : (
                <button onClick={() => setRejecting(true)} disabled={busy}>Reject</button>
              )}
              <button className="primary" disabled={busy} onClick={() => {
                const nd = { ...decisions };
                for (const c of step!.changes) nd[c.id] = "accepted";
                const next = steps.findIndex((s) => !stepDone(s, nd));
                setState({ ...state, decisions: nd, index: next === -1 ? steps.length : next });
                if (next !== -1) onFocus(steps[next].changes[0]);
              }}>{step!.changes.length > 1 ? `Accept all ${step!.changes.length}` : "Accept"}</button>
            </div>
          ) : (
            <div className="callout" style={{ marginTop: 10 }}>
              <div className="small" style={{ fontWeight: 600 }}>Why reject it? <span className="muted" style={{ fontWeight: 400 }}>Optional — it helps the Assistant rework the rest.{step!.changes.length > 1 ? " Its connections go with it." : ""}</span></div>
              <textarea rows={2} value={reason} onChange={(e) => setReason(e.target.value)} placeholder="e.g. we already have a cache in front of this" style={{ marginTop: 8 }} />
              <div className="row" style={{ marginTop: 8 }}>
                <span className="spacer" />
                <button className="ghost" onClick={() => { setRejecting(false); setReason(""); }}>Cancel</button>
                <button className="primary" disabled={busy} onClick={() => { onRejectChange(cur.id, accepted, reason.trim()); setRejecting(false); setReason(""); }}>Reject and revise</button>
              </div>
            </div>
          )}
          {accepted.length > 0 && (
            <>
              <h4>Accepted, drawn on the diagram</h4>
              {changes.filter((c) => decisions[c.id] === "accepted").map((c) => <ChangeRow key={c.id} c={c} label={label} />)}
            </>
          )}
          <p className="small muted" style={{ marginTop: 14 }}>{steps.length - index - 1 > 0 ? `${steps.length - index - 1} more step${steps.length - index - 1 === 1 ? "" : "s"} after this one.` : "This is the last step."}</p>
        </>
      )}

      {mode === "step" && !cur && (
        <div className="callout">
          <b>{accepted.length} of {changes.length} changes accepted.</b>
          <div className="row" style={{ marginTop: 10 }}>
            <button className="primary" disabled={!accepted.length || busy} onClick={() => onAccept(accepted)}>Apply {accepted.length} change{accepted.length === 1 ? "" : "s"}</button>
            <button onClick={() => setState(reviewStart({ ...pending, accepted: [] }))}>Start over</button>
          </div>
        </div>
      )}

      {mode === "full" && (
        <>
          {changes.map((c) => (
            <ChangeRow key={c.id} c={c} label={label} status={decisions[c.id] ?? "to review"}
              onClick={() => { const i = steps.findIndex((s) => s.changes.includes(c)); setState({ ...state, mode: "step", index: i }); onFocus(c); }} />
          ))}
          <div className="row" style={{ marginTop: 12 }}>
            <button className="primary" disabled={busy} onClick={() => onAccept(changes.map((c) => c.id))}>Accept all {changes.length}</button>
            <span className="small muted">Or click a change to review it on its own.</span>
          </div>
        </>
      )}

      {rejectedList}
      {footer}
      {replying && (
        <div style={{ marginTop: 10 }}>
          <textarea rows={3} value={reply} onChange={(e) => setReply(e.target.value)} placeholder="What should change? The Assistant will send a new revision." />
          <div className="row" style={{ marginTop: 6 }}>
            <span className="spacer" />
            <button className="primary" disabled={!reply.trim() || busy} onClick={() => { onReply(reply.trim()); setReply(""); setReplying(false); }}>Send reply</button>
          </div>
        </div>
      )}
    </div>
  );
}
