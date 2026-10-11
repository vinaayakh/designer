// Timings, slow work, long tasks, dropped frames, memory and errors from the
// browser. Entries are batched to /api/clientlog, so they land in the same
// designer.log as the server's (filtered there by its log level).

type Level = "debug" | "info" | "warn" | "error";
type Entry = { level: Level; msg: string; name?: string; ms?: number; slow?: boolean; data?: Record<string, unknown> };

const inBrowser = typeof window !== "undefined" && typeof fetch === "function";
const queue: Entry[] = [];
let timer: number | undefined;

/** Work slower than this is a warning: main-thread work blocks the UI, async work only waits. */
const SLOW_SYNC_MS = 100;
const SLOW_ASYNC_MS = 1500;
const slowFor: Record<string, number> = { "layout.elk": 3000, "export.images": 4000, "review.pan": 600 };

function push(e: Entry) {
  if (!inBrowser) return;
  queue.push(e);
  if (e.level === "error") flush();
  else if (timer === undefined) timer = window.setTimeout(flush, 4000);
}

/** Sends what's queued. keepalive lets it finish while the page unloads. */
export function flush() {
  if (!inBrowser) return;
  window.clearTimeout(timer);
  timer = undefined;
  if (!queue.length) return;
  const entries = queue.splice(0, 200);
  fetch("/api/clientlog", {
    method: "POST",
    keepalive: true,
    headers: { "X-Designer": "1", "Content-Type": "application/json", ...(clientId ? { "X-Client": clientId } : {}) },
    body: JSON.stringify({ entries }),
  }).catch(() => {});
  if (queue.length) timer = window.setTimeout(flush, 1000);
}

let clientId = "";
const round = (v: number) => Math.round(v * 10) / 10;

export const perf = {
  /** Records a duration measured elsewhere. */
  mark(name: string, ms: number, data?: Record<string, unknown>, slowMs = slowFor[name] ?? SLOW_ASYNC_MS) {
    const slow = ms > slowMs;
    push({ level: slow ? "warn" : "debug", msg: slow ? "slow work" : "work", name, ms: round(ms), slow, data: slow ? { ...data, slow_ms: slowMs } : data });
  },

  /** Records a measured event with its own verdict (several numbers decide whether it was slow). */
  record(name: string, ms: number, slow: boolean, data?: Record<string, unknown>) {
    push({ level: slow ? "warn" : "debug", msg: slow ? "slow work" : "work", name, ms: round(ms), slow, data });
  },

  /** Times synchronous (main-thread) work. */
  time<T>(name: string, fn: () => T, data?: Record<string, unknown>): T {
    const t0 = performance.now();
    try {
      const out = fn();
      perf.mark(name, performance.now() - t0, data, slowFor[name] ?? SLOW_SYNC_MS);
      return out;
    } catch (err) {
      perf.error(`${name} failed`, err, { ms: round(performance.now() - t0), ...data });
      throw err;
    }
  },

  /** Times asynchronous work. */
  async timeAsync<T>(name: string, fn: () => Promise<T>, data?: Record<string, unknown>): Promise<T> {
    const t0 = performance.now();
    try {
      const out = await fn();
      perf.mark(name, performance.now() - t0, data);
      return out;
    } catch (err) {
      perf.error(`${name} failed`, err, { ms: round(performance.now() - t0), ...data });
      throw err;
    }
  },

  log(level: Level, msg: string, data?: Record<string, unknown>) {
    push({ level, msg, data });
  },

  error(msg: string, err: unknown, data?: Record<string, unknown>) {
    const e = err as Error;
    push({ level: "error", msg, data: { ...data, error: String(e?.message ?? err), stack: e?.stack?.split("\n").slice(0, 6).join(" | ") } });
  },

  /**
   * Watches animation frames while fn runs, and logs how many were dropped
   * (gaps over 1.5 frames at 60 Hz).
   */
  async frames<T>(name: string, fn: () => Promise<T>, data?: Record<string, unknown>): Promise<T> {
    if (!inBrowser) return fn();
    let last = performance.now();
    const t0 = last;
    let frames = 0, dropped = 0, worst = 0, on = true;
    const tick = (now: number) => {
      const gap = now - last;
      last = now;
      frames++;
      worst = Math.max(worst, gap);
      if (gap > 25) dropped += Math.round(gap / 16.7) - 1;
      if (on) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
    try {
      return await fn();
    } finally {
      on = false;
      const ms = performance.now() - t0;
      const janky = dropped > 2 || worst > 50;
      push({ level: janky ? "warn" : "debug", msg: janky ? "dropped frames" : "frames", name, ms: round(ms), slow: janky,
        data: { ...data, frames, dropped, worst_frame_ms: round(worst), fps: round(frames / (ms / 1000)) } });
    }
  },
};

/** Long tasks, memory and uncaught errors, for the whole page. */
export function startPerf(client: string) {
  if (!inBrowser) return;
  clientId = client;
  let longCount = 0, longTotal = 0;
  try {
    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) {
        longCount++;
        longTotal += e.duration;
        if (e.duration > 200) push({ level: "warn", msg: "long task blocked the page", name: "longtask", ms: round(e.duration), slow: true });
      }
    }).observe({ type: "longtask", buffered: true });
  } catch {
    // not supported (Firefox, Safari)
  }
  window.setInterval(() => {
    const mem = (performance as unknown as { memory?: { usedJSHeapSize: number; totalJSHeapSize: number; jsHeapSizeLimit: number } }).memory;
    const data: Record<string, unknown> = { long_tasks: longCount, long_task_ms: round(longTotal) };
    if (mem) Object.assign(data, { heap_used_mb: round(mem.usedJSHeapSize / 2 ** 20), heap_total_mb: round(mem.totalJSHeapSize / 2 ** 20), heap_limit_mb: round(mem.jsHeapSizeLimit / 2 ** 20) });
    const heavy = mem && mem.usedJSHeapSize > 1024 * 2 ** 20;
    push({ level: heavy ? "warn" : "debug", msg: heavy ? "high browser memory" : "browser resources", data });
    longCount = 0;
    longTotal = 0;
  }, 30_000);
  window.addEventListener("error", (e) => perf.error("uncaught error", e.error ?? e.message, { source: e.filename, line: e.lineno }));
  window.addEventListener("unhandledrejection", (e) => perf.error("unhandled promise rejection", e.reason));
  window.addEventListener("pagehide", flush);
  document.addEventListener("visibilitychange", () => { if (document.visibilityState === "hidden") flush(); });
  const nav = performance.getEntriesByType("navigation")[0] as PerformanceNavigationTiming | undefined;
  if (nav) push({ level: "info", msg: "page loaded", name: "page.load", ms: round(nav.loadEventEnd || nav.domContentLoadedEventEnd), data: { dom_ready_ms: round(nav.domContentLoadedEventEnd), transfer_kb: round(nav.transferSize / 1024) } });
}
