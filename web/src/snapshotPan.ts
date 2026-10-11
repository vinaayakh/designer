// Smooth panning and zooming for large diagrams.
//
// Excalidraw redraws every visible element on each pan or zoom step, so the
// cost grows with the diagram. While a wheel or trackpad gesture or a drag-pan
// is under way, this shows a copy of the last frame instead and moves it with
// a CSS transform (the GPU does that), and only hands the final position to
// Excalidraw when the gesture ends. When the copy has drifted far (edges
// uncovered, or blurry after zooming), it refreshes: one real redraw, then a
// new copy.
//
// The viewport maths mirrors Excalidraw's own wheel and pan handling (zoom
// step, cap per event, zoom around the cursor, normalisation), so where you
// end up is exactly where Excalidraw would have put you.

import type { ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
import { perf } from "./perf";

type View = { scrollX: number; scrollY: number; zoom: number };

const SETTLE_MS = 140; // hand over to Excalidraw this long after the last wheel event
const MAX_SHIFT = 0.3; // refresh once the copy has moved this share of the viewport
const MAX_SCALE = 1.5; // …or been scaled this much either way
const MIN_ZOOM = 0.1, MAX_ZOOM = 30, ZOOM_CAP = 10; // Excalidraw's

const normalizeZoom = (z: number) => Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.round((z + Number.EPSILON) * 1e6) / 1e6));

/** Excalidraw's zoomBy for one wheel event, around a point in the canvas. */
function zoomStep(v: View, deltaY: number, px: number, py: number): View {
  const sign = Math.sign(deltaY), abs = Math.abs(deltaY);
  const d = abs > ZOOM_CAP ? ZOOM_CAP * sign : deltaY;
  let next = v.zoom - d / 100;
  next += Math.log10(Math.max(1, v.zoom)) * -sign * Math.min(1, abs / 20);
  next = normalizeZoom(Math.max(next, MIN_ZOOM));
  if (next === v.zoom) return v;
  // getStateForZoom: keep the point under the cursor still.
  return {
    scrollX: v.scrollX + (px - px / v.zoom) - (px - px / next),
    scrollY: v.scrollY + (py - py / v.zoom) - (py - py / next),
    zoom: next,
  };
}

/** Attaches snapshot panning to the element holding the Excalidraw canvas. Returns a cleanup. */
export function attachSnapshotPan(root: HTMLElement, getApi: () => ExcalidrawImperativeAPI | null): () => void {
  let overlay: HTMLDivElement | null = null; // covers both Excalidraw canvases while active
  let copy: HTMLCanvasElement | null = null; // the frame being moved
  let base: View | null = null; // the viewport the copy shows
  let cur: View | null = null; // where the wheel has taken us
  let settle = 0;
  let refreshing = false;
  let ending = false;
  let started = 0, events = 0, refreshes = 0;
  // What each gesture cost, for the log (see report()).
  let stats = { kind: "", firstInput: 0, firstFrameMs: -1, copyMs: 0, refreshMs: [] as number[], handoverMs: 0, worstFrameMs: 0 };
  let frameWatch = 0;
  const skipped: Record<string, number> = {}; // wheel events left to Excalidraw, by reason
  const watchFrames = () => {
    let last = performance.now();
    const tick = (t: number) => {
      stats.worstFrameMs = Math.max(stats.worstFrameMs, t - last);
      last = t;
      if (frameWatch) frameWatch = requestAnimationFrame(tick);
    };
    frameWatch = requestAnimationFrame(tick);
  };
  // Excalidraw zooms around the last pointer position it saw (from pointer moves, starting at 0,0), not the wheel event's.
  const pointer = { x: 0, y: 0 };

  // Drag pans (hand tool, Space + drag, middle button, any drag in read-only views): Excalidraw
  // starts its pan as usual (cursor, state, clean-up stay its own). It shows the "grabbing"
  // cursor exactly when it does, so the first move checks that, and only then do the moves go to
  // the snapshot instead (scroll changes by the movement over zoom, as in Excalidraw). The release
  // hands the final viewport over.
  let drag: { x: number; y: number; confirmed: boolean } | null = null;
  const interactiveCanvas = () => root.querySelector<HTMLCanvasElement>("canvas.excalidraw__canvas.interactive");
  const onPointerDown = (e: PointerEvent) => {
    const st = getApi()?.getAppState();
    const candidate = e.isPrimary && (e.button === 0 || e.button === 1) && e.target instanceof HTMLCanvasElement && st && !st.editingTextElement;
    if (!candidate) {
      finishNow();
      return;
    }
    window.clearTimeout(settle); // a wheel gesture still showing carries on if this turns out to be a pan
    drag = { x: e.clientX, y: e.clientY, confirmed: false };
  };
  const onPointerMove = (e: PointerEvent) => {
    pointer.x = e.clientX;
    pointer.y = e.clientY;
    if (!drag || !e.isPrimary) return;
    if (!drag.confirmed) {
      const api = getApi();
      if (!api || interactiveCanvas()?.style.cursor !== "grabbing" || (!overlay && !begin(api))) {
        drag = null; // not a pan (selecting, dragging a shape…): Excalidraw's as usual
        finishNow();
        return;
      }
      drag.confirmed = true;
    }
    e.stopPropagation(); // Excalidraw's own pan move would redraw everything
    noteInput("drag", e.timeStamp);
    const dx = e.clientX - drag.x, dy = e.clientY - drag.y;
    drag.x = e.clientX;
    drag.y = e.clientY;
    events++;
    cur = { ...cur!, scrollX: cur!.scrollX + dx / cur!.zoom, scrollY: cur!.scrollY + dy / cur!.zoom };
    place();
  };
  const onPointerUp = (e: PointerEvent) => {
    if (!drag || !e.isPrimary) return;
    const was = drag;
    drag = null;
    if (was.confirmed) void end();
    else finishNow();
  };


  const staticCanvas = () => root.querySelector<HTMLCanvasElement>("canvas.excalidraw__canvas.static");
  const viewOf = (api: ExcalidrawImperativeAPI): View => {
    const s = api.getAppState();
    return { scrollX: s.scrollX, scrollY: s.scrollY, zoom: s.zoom.value };
  };
  const isZoom = (e: WheelEvent) => e.ctrlKey || e.metaKey || (getApi()?.getAppState().inputDevice === "mouse" && !e.shiftKey && e.deltaY !== 0);

  /** Copies what Excalidraw last drew. */
  const snapshot = (src: HTMLCanvasElement) => {
    if (!copy) return;
    if (copy.width !== src.width || copy.height !== src.height) {
      copy.width = src.width;
      copy.height = src.height;
    }
    const ctx = copy.getContext("2d")!;
    ctx.clearRect(0, 0, copy.width, copy.height);
    ctx.drawImage(src, 0, 0);
  };

  /** Notes the gesture's first input, and when the frame showing it starts. */
  const noteInput = (kind: string, at: number) => {
    if (stats.firstInput) return;
    stats.kind = kind;
    stats.firstInput = at;
    requestAnimationFrame((t) => { stats.firstFrameMs = t - at; });
  };

  /** Places the copy so it shows `cur`: screen = (scene + scroll) * zoom. */
  const place = () => {
    if (!copy || !base || !cur) return;
    const k = cur.zoom / base.zoom;
    const tx = (cur.scrollX - base.scrollX) * cur.zoom;
    const ty = (cur.scrollY - base.scrollY) * cur.zoom;
    copy.style.transform = `matrix(${k},0,0,${k},${tx},${ty})`;
    const r = overlay!.getBoundingClientRect();
    const drift = Math.max(Math.abs(tx) / r.width, Math.abs(ty) / r.height);
    if (!refreshing && !ending && (drift > MAX_SHIFT || k > MAX_SCALE || k < 1 / MAX_SCALE)) void refresh();
  };

  const begin = (api: ExcalidrawImperativeAPI): boolean => {
    const src = staticCanvas();
    if (!src || !src.parentElement) return false;
    const box = src.getBoundingClientRect(), parentBox = src.parentElement.getBoundingClientRect();
    overlay = document.createElement("div");
    overlay.className = "snapshot-pan";
    overlay.style.cssText = `position:absolute;left:${box.left - parentBox.left}px;top:${box.top - parentBox.top}px;width:${box.width}px;height:${box.height}px;overflow:hidden;pointer-events:none;z-index:2;background:${api.getAppState().viewBackgroundColor || "#fff"}`;
    copy = document.createElement("canvas");
    copy.style.cssText = `position:absolute;left:0;top:0;width:${box.width}px;height:${box.height}px;transform-origin:0 0;will-change:transform`;
    overlay.appendChild(copy);
    const c0 = performance.now();
    snapshot(src);
    stats = { kind: "", firstInput: 0, firstFrameMs: -1, copyMs: performance.now() - c0, refreshMs: [], handoverMs: 0, worstFrameMs: 0 };
    watchFrames();
    // Above both canvases, below Excalidraw's toolbars.
    const interactive = root.querySelector<HTMLCanvasElement>("canvas.excalidraw__canvas.interactive");
    (interactive ?? src).after(overlay);
    base = viewOf(api);
    cur = { ...base };
    started = performance.now();
    events = refreshes = 0;
    return true;
  };

  /**
   * Lets Excalidraw draw `v`, and resolves once it has. Mid-gesture it keeps its element bitmaps
   * at their old zoom (as Excalidraw's own zooming does) instead of rebuilding every one; the
   * final hand-over rebuilds them once, sharp.
   */
  const commit = (api: ExcalidrawImperativeAPI, v: View, final: boolean) =>
    new Promise<void>((resolve) => {
      api.updateScene({ appState: { scrollX: v.scrollX, scrollY: v.scrollY, zoom: { value: v.zoom }, shouldCacheIgnoreZoom: !final } as any });
      let tries = 0;
      const wait = () => {
        const s = api.getAppState();
        const there = s.scrollX === v.scrollX && s.scrollY === v.scrollY && s.zoom.value === v.zoom;
        // Excalidraw draws in an effect after its commit: give it a frame after the state lands.
        if ((there && tries >= 1) || tries > 30) resolve();
        else { tries++; requestAnimationFrame(wait); }
      };
      requestAnimationFrame(wait);
    });

  /** One real redraw mid-gesture, then a fresh copy of it. */
  const refresh = async () => {
    const api = getApi();
    if (!api || !cur || refreshing) return;
    refreshing = true;
    const target = { ...cur };
    const r0 = performance.now();
    await commit(api, target, false);
    stats.refreshMs.push(performance.now() - r0);
    const src = staticCanvas();
    if (overlay && src) {
      snapshot(src);
      base = target;
      refreshes++;
      place();
    }
    refreshing = false;
  };

  const end = async () => {
    window.clearTimeout(settle);
    const api = getApi();
    if (!overlay || !api || !cur || ending) return;
    ending = true;
    while (refreshing) await new Promise((r) => requestAnimationFrame(r));
    const target = { ...cur! };
    const h0 = performance.now();
    await commit(api, target, true);
    stats.handoverMs = performance.now() - h0;
    ending = false;
    const src = staticCanvas();
    if (cur && (cur.scrollX !== target.scrollX || cur.scrollY !== target.scrollY || cur.zoom !== target.zoom) && src) {
      // The wheel moved again while Excalidraw caught up: carry on from what it just drew.
      snapshot(src);
      base = target;
      place();
      settle = window.setTimeout(() => void end(), SETTLE_MS);
      return;
    }
    overlay?.remove();
    overlay = copy = null;
    base = cur = null;
    cancelAnimationFrame(frameWatch);
    frameWatch = 0;
    report();
  };

  /**
   * Logs what the gesture cost: input to first moved frame, copying the snapshot, each mid-gesture
   * redraw, the final hand-over redraw (the sharp picture appears after it), and the worst frame.
   * Any of those noticeably slow makes it a warning.
   */
  const report = () => {
    const s = stats;
    const r = (v: number) => Math.round(v * 10) / 10;
    const data: Record<string, unknown> = {
      gesture: s.kind || "unknown", events, refreshes, settle_ms: SETTLE_MS,
      first_frame_ms: r(s.firstFrameMs), copy_ms: r(s.copyMs), refresh_ms_max: r(Math.max(0, ...s.refreshMs)), handover_ms: r(s.handoverMs),
      worst_frame_ms: r(s.worstFrameMs),
    };
    const reasons = Object.entries(skipped).filter(([, n]) => n);
    if (reasons.length) {
      data.wheel_left_to_excalidraw = Object.fromEntries(reasons);
      for (const k of Object.keys(skipped)) skipped[k] = 0;
    }
    const slow = s.firstFrameMs > 34 || s.copyMs > 16 || s.handoverMs > 150 || s.worstFrameMs > 50;
    perf.record("viewport.gesture", performance.now() - started, slow, data);
  };

  const skip = (why: string) => { skipped[why] = (skipped[why] ?? 0) + 1; };
  const onWheel = (e: WheelEvent) => {
    // Read the deltas before deltaMode: Firefox reports a mouse wheel in lines (about 3 instead of
    // 100 px) to pages that look at deltaMode first, which would also shrink Excalidraw's steps.
    const deltaX = e.deltaX, deltaY = e.deltaY;
    if (!(e.target instanceof HTMLCanvasElement)) return;
    if (e.deltaMode !== 0) return skip(`deltaMode ${e.deltaMode}`);
    if (e.buttons !== 0) return skip("button held");
    const api = getApi();
    if (!api) return;
    const st = api.getAppState();
    if (st.editingTextElement || st.newElement) return skip("editing"); // mid-edit: leave it to Excalidraw
    if (!overlay && !begin(api)) return;
    e.preventDefault();
    e.stopPropagation();
    events++;
    const v = cur!;
    const zooming = deltaY !== 0 && isZoom(e);
    noteInput(zooming ? "wheel-zoom" : "wheel-pan", e.timeStamp);
    if (zooming) {
      cur = zoomStep(v, deltaY, pointer.x - st.offsetLeft, pointer.y - st.offsetTop);
    } else if (e.shiftKey) {
      const s = deltaY || deltaX;
      cur = e.ctrlKey || e.metaKey ? { ...v, scrollY: v.scrollY - s / v.zoom } : { ...v, scrollX: v.scrollX - s / v.zoom };
    } else {
      cur = { ...v, scrollX: v.scrollX - deltaX / v.zoom, scrollY: v.scrollY - deltaY / v.zoom };
    }
    place();
    window.clearTimeout(settle);
    settle = window.setTimeout(() => void end(), SETTLE_MS);
  };

  // Anything else (a click, a key, a resize) ends the gesture first.
  const finishNow = () => { if (overlay && !drag?.confirmed) void end(); };
  const onKeyDown = (e: KeyboardEvent) => { if (!e.repeat && e.code !== "Space") finishNow(); };
  const onBlur = () => { if (drag) { const was = drag; drag = null; if (was.confirmed) void end(); } };
  root.addEventListener("wheel", onWheel, { capture: true, passive: false });
  window.addEventListener("pointermove", onPointerMove, { capture: true });
  root.addEventListener("pointerdown", onPointerDown, true);
  window.addEventListener("pointerup", onPointerUp, true);
  window.addEventListener("keydown", onKeyDown, true);
  window.addEventListener("blur", onBlur);
  window.addEventListener("resize", finishNow);
  return () => {
    root.removeEventListener("wheel", onWheel, { capture: true });
    window.removeEventListener("pointermove", onPointerMove, { capture: true });
    root.removeEventListener("pointerdown", onPointerDown, true);
    window.removeEventListener("pointerup", onPointerUp, true);
    window.removeEventListener("keydown", onKeyDown, true);
    window.removeEventListener("blur", onBlur);
    window.removeEventListener("resize", finishNow);
    window.clearTimeout(settle);
    overlay?.remove();
  };
}

/** On unless turned off with localStorage "designer.snapshotPan" = "off" (for comparisons). */
export function snapshotPanEnabled(): boolean {
  try {
    return localStorage.getItem("designer.snapshotPan") !== "off";
  } catch {
    return true;
  }
}
