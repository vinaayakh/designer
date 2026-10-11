import { useState } from "react";
import { api } from "../api";
import { BriefEditor } from "../Wizard";
import type { Brief, BriefSchema, Settings } from "../types";

const MODELS = [
  ["", "Claude Code's default"],
  ["opus", "Opus"],
  ["sonnet", "Sonnet"],
  ["haiku", "Haiku"],
];

export function SettingsModal({ settings, schema, brief, name, trusted, kbVersion, cloudLocked, onClose, onSaved }: {
  settings: Settings;
  schema: BriefSchema;
  brief: Brief;
  name: string;
  trusted: string[];
  kbVersion: string;
  cloudLocked: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [s, setS] = useState(settings);
  const [n, setN] = useState(name);
  const [msg, setMsg] = useState("");
  const save = async (next: Settings) => {
    setS(next);
    try {
      await api.saveSettings(next);
      setMsg("Saved.");
      onSaved();
    } catch (e) {
      setMsg((e as Error).message);
    }
  };
  return (
    <div className="modal-back" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-label="Settings">
        <div className="row"><h2>Project settings</h2><span className="spacer" /><button className="ghost" onClick={onClose} aria-label="Close">✕</button></div>
        <div className="field">
          <label>Project name</label>
          <div className="row">
            <input type="text" value={n} onChange={(e) => setN(e.target.value)} style={{ flex: 1 }} />
            <button disabled={!n.trim() || n === name} onClick={() => api.rename(n.trim()).then(onSaved)}>Rename</button>
          </div>
        </div>
        <h4 className="muted small">Design Brief</h4>
        <BriefEditor schema={schema} brief={brief} onSaved={onSaved} cloudLocked={cloudLocked} />
        <h4 className="muted small">Assistant (this install)</h4>
        <div className="field">
          <label>Model</label>
          <select value={s.model} onChange={(e) => void save({ ...s, model: e.target.value })}>
            {MODELS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
          </select>
        </div>
        <div className="field">
          <label>Conversion model (Design and Gap Check)</label>
          <select value={s.conversionModel} onChange={(e) => void save({ ...s, conversionModel: e.target.value })}>
            {MODELS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
          </select>
        </div>
        <div className="row">
          <button onClick={() => api.resetSession().then(() => setMsg("The next message starts a new Assistant Session."))}>Start a new Assistant Session</button>
        </div>
        {msg && <p className="small muted">{msg}</p>}
        <h4 className="muted small">Knowledge Base and trusted sources</h4>
        <p className="small">Knowledge Base version {kbVersion}. When it has no answer, the Assistant may only use: {trusted.join(", ")}.</p>
      </div>
    </div>
  );
}
