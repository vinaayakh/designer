import type { Brief, Graph, KBRecord, Pending, ReportRef, Settings, State, Version, GapReport, Issue } from "./types";
import { perf } from "./perf";

/** Identifies this tab, so it can ignore its own save notifications. */
export const CLIENT_ID = Math.random().toString(36).slice(2);

/** The route of a path, for grouping timings: /api/versions/3 → /api/versions/{n}. */
const route = (method: string, path: string) => `${method} ${path.split("?")[0].replace(/\/\d+(?=\/|$)/g, "/{n}")}`;

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const t0 = performance.now();
  const sent = body === undefined ? undefined : typeof body === "string" ? body : JSON.stringify(body);
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers: { "X-Designer": "1", "X-Client": CLIENT_ID, ...(body !== undefined ? { "Content-Type": "application/json" } : {}) },
      body: sent,
    });
  } catch (err) {
    perf.error(`request failed: ${route(method, path)}`, err, { ms: Math.round(performance.now() - t0) });
    throw err;
  }
  const text = await res.text();
  const ms = performance.now() - t0;
  const data = text ? JSON.parse(text) : null;
  perf.mark(`api ${route(method, path)}`, ms, { status: res.status, in_bytes: sent?.length ?? 0, out_bytes: text.length });
  if (!res.ok) {
    perf.log(res.status >= 500 ? "error" : "warn", `request ${res.status}: ${route(method, path)}`, { error: data?.error, ms: Math.round(ms) });
    throw new Error(data?.error ?? `${res.status} ${res.statusText}`);
  }
  return data as T;
}

export const api = {
  state: () => call<State>("GET", "/api/state"),
  saveBrief: (b: Brief) => call<{ complete: boolean; compact: string }>("PUT", "/api/brief", b),
  rename: (name: string) => call("PUT", "/api/project", { name }),

  rough: () => call<unknown>("GET", "/api/rough"),
  saveRough: (sceneJSON: string) => call("PUT", "/api/rough", sceneJSON),
  versions: () => call<Version[] | null>("GET", "/api/versions"),
  version: (n: number) => call<{ version: Version; scene: unknown }>("GET", `/api/versions/${n}`),
  saveVersion: (scene: unknown, source: "edit" | "tidy" | "restore", base: number, summary = "") =>
    call<Version>("POST", "/api/versions", { scene, source, base, summary }),
  graph: (view: string) => call<{ graph: Graph; version: number }>("GET", `/api/graph/${view}`),

  reports: () => call<ReportRef[] | null>("GET", "/api/reports"),
  report: (file: string) => call<GapReport>("GET", `/api/reports/${encodeURIComponent(file)}`),
  record: (id: string) => call<KBRecord>("GET", `/api/kb/records/${encodeURIComponent(id)}`),

  precheck: () => call<Issue[] | null>("GET", "/api/precheck"),
  convert: () => call("POST", "/api/convert", {}),
  gapCheck: (version: number) => call("POST", "/api/gapcheck", { version }),
  fix: (report: string, records: string[]) => call("POST", "/api/fix", { report, records }),

  warm: () => call("POST", "/api/assistant/warm", {}),
  chat: (text: string, view: string, selection: string[]) => call("POST", "/api/assistant/chat", { text, view, selection }),
  interrupt: () => call("POST", "/api/assistant/interrupt", {}),
  retry: () => call("POST", "/api/assistant/retry", {}),
  resetSession: () => call("POST", "/api/assistant/reset", {}),

  pending: () => call<Pending | null>("GET", "/api/proposal"),
  simulate: (accepted: string[] | null) => call<{ graph: Graph; base: Graph }>("POST", "/api/proposal/simulate", { accepted }),
  decide: (d: { id: string; revision: number; action: "accept" | "reject" | "reply" | "reject_change" | "clarify"; change?: string; accepted?: string[]; scene?: unknown; reply?: string }) =>
    call<{ version?: number }>("POST", "/api/proposal/decision", d),

  documentStatus: (n: number) => call<{ written: boolean; stale: boolean }>("GET", `/api/document/${n}`),
  writeDocument: (version: number) => call("POST", "/api/document", { version }),

  settings: () => call<Settings>("GET", "/api/settings"),
  saveSettings: (s: Settings) => call<Settings>("PUT", "/api/settings", s),
};

/** POSTs and returns the response as a file (Content-Disposition names it). */
export async function download(path: string, body: unknown): Promise<{ blob: Blob; name: string }> {
  return perf.timeAsync(`api ${route("POST", path)}`, () => fetchFile(path, body));
}

async function fetchFile(path: string, body: unknown): Promise<{ blob: Blob; name: string }> {
  const res = await fetch(path, {
    method: "POST",
    headers: { "X-Designer": "1", "X-Client": CLIENT_ID, "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new Error(data?.error ?? `${res.status} ${res.statusText}`);
  }
  const name = /filename="([^"]+)"/.exec(res.headers.get("Content-Disposition") ?? "")?.[1] ?? "document";
  return { blob: await res.blob(), name };
}

export type ServerEvent = { type: string; data: any };

/** Subscribes to the runtime's event stream, reconnecting automatically. */
export function subscribe(onEvent: (e: ServerEvent) => void, onConnection: (up: boolean) => void): () => void {
  let es: EventSource | null = null;
  let closed = false;
  let retry: number | undefined;
  const open = () => {
    es = new EventSource("/api/events");
    es.onopen = () => onConnection(true);
    es.onmessage = (m) => {
      try {
        onEvent(JSON.parse(m.data));
      } catch {
        /* ignore malformed */
      }
    };
    es.onerror = () => {
      onConnection(false);
      es?.close();
      if (!closed) retry = window.setTimeout(open, 1500);
    };
  };
  open();
  return () => {
    closed = true;
    window.clearTimeout(retry);
    es?.close();
  };
}
