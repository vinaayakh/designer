import { useEffect, useRef, useState } from "react";
import { Markdown } from "../Markdown";
import type { AssistantStatus } from "../types";

export type ChatItem = { id: number; kind: "user" | "assistant" | "tool" | "notice" | "error"; text: string; retry?: boolean };

const TOOL_TEXT: Record<string, (i: any) => string> = {
  search_kb: (i) => `Searching the Knowledge Base: “${i?.query ?? ""}”`,
  get_record: (i) => `Reading ${i?.id ?? "a Record"}`,
  get_diagram: () => "Reading the whole diagram",
  get_nodes: (i) => `Looking at ${(i?.aliases ?? []).join(", ")}`,
  submit_proposal: () => "Submitting a Proposal",
  submit_gap_report: () => "Submitting the Gap Report",
  WebSearch: (i) => `Searching the web (${(i?.allowed_domains ?? []).join(", ")}): “${i?.query ?? ""}”`,
  WebFetch: (i) => `Reading ${i?.url ?? "a web page"}`,
};
export const toolText = (name: string, input: unknown) => (TOOL_TEXT[name] ?? (() => `Using ${name}`))(input);

const TASK: Record<string, string> = { chat: "Thinking", design: "Designing the Production Diagram", gap: "Running the Gap Check", fix: "Drafting a fix", reply: "Revising" };

export function Chat({ items, status, context, onRemoveContext, onSend, onStop, onRetry, onWarm, canSend, placeholder }: {
  items: ChatItem[];
  status: AssistantStatus;
  context: { id: string; label: string }[];
  onRemoveContext: (id: string) => void;
  onSend: (text: string) => void;
  onStop: () => void;
  onRetry: () => void;
  onWarm: () => void;
  canSend: boolean;
  placeholder: string;
}) {
  const [text, setText] = useState("");
  const log = useRef<HTMLDivElement>(null);
  const warmed = useRef(false);
  useEffect(() => {
    log.current?.scrollTo({ top: log.current.scrollHeight });
  }, [items]);
  const warm = () => {
    if (!warmed.current && !status.running) {
      warmed.current = true;
      onWarm();
    }
  };
  const send = () => {
    const t = text.trim();
    if (!t || status.busy || !canSend) return;
    onSend(t);
    setText("");
  };
  return (
    <div className="chat">
      <div className="status-line">
        <span className={`dot ${status.busy ? "busy" : status.running ? "on" : ""}`} />
        <span>{status.busy ? `${TASK[status.task ?? "chat"] ?? "Working"}…` : status.running ? "Ready" : "Idle"}</span>
        <span className="spacer" />
        {status.costUsd > 0 && <span className="cost" title="Estimated cost of this Assistant Session, as reported by Claude Code">${status.costUsd.toFixed(3)}</span>}
      </div>
      {status.warning && <div className="callout warn small" style={{ margin: "10px 16px 0" }}>{status.warning}</div>}
      <div className="chat-log" ref={log}>
        {items.length === 0 && (
          <div className="empty">
            Ask about the design, or ask for a change. Click Nodes on the canvas to add them to your message.
            <br />
            <span className="small">Changes always come as Proposals you review.</span>
          </div>
        )}
        {items.map((m) => (
          <div key={m.id} className={`msg ${m.kind}`}>
            {m.kind === "assistant" ? <Markdown text={m.text} className="md chat-md" /> : m.text}
            {m.retry && status.canRetry && !status.busy && (
              <div style={{ marginTop: 6 }}><button onClick={onRetry}>Retry</button></div>
            )}
          </div>
        ))}
      </div>
      <div className="composer">
        {context.length > 0 && (
          <div className="chips">
            {context.map((c) => (
              <span className="chip" key={c.id}>
                {c.label}
                <button onClick={() => onRemoveContext(c.id)} aria-label={`Remove ${c.label}`}>×</button>
              </span>
            ))}
          </div>
        )}
        <textarea
          rows={3}
          value={text}
          placeholder={placeholder}
          onFocus={warm}
          onChange={(e) => { setText(e.target.value); warm(); }}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              send();
            }
          }}
        />
        <div className="row">
          <span className="small muted">Enter to send · Shift+Enter for a new line</span>
          <span className="spacer" />
          {status.busy ? <button onClick={onStop}>Stop</button> : <button className="primary" disabled={!text.trim() || !canSend} onClick={send}>Send</button>}
        </div>
      </div>
    </div>
  );
}
