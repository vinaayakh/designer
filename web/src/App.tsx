import { useCallback, useEffect, useState } from "react";
import { api } from "./api";
import { Wizard } from "./Wizard";
import { Workspace } from "./Workspace";
import type { State } from "./types";

export function App() {
  const [st, setSt] = useState<State | null>(null);
  const [err, setErr] = useState("");
  const reload = useCallback(async () => {
    const s = await api.state();
    setSt(s);
    return s;
  }, []);
  useEffect(() => {
    reload().catch((e) => setErr(e.message));
  }, [reload]);

  useEffect(() => {
    if (st) document.title = `${st.project.name} · Designer`;
  }, [st?.project.name]);

  if (err) return <div className="empty">Can't reach the Designer runtime: {err}. Is <code>designer</code> still running?</div>;
  if (!st) return <div className="empty">Loading…</div>;
  if (!st.briefDone) return <Wizard schema={st.briefSchema} initial={st.brief} onDone={() => reload()} />;
  return <Workspace initial={st} reload={reload} />;
}
