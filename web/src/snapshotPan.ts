// Smooth wheel panning and zooming for large diagrams.
//
// Excalidraw redraws every visible element on each pan or zoom step, so the
// cost grows with the diagram. While the wheel is moving, this shows a copy of
// the last frame instead and moves it with a CSS transform (the GPU does
// that), and only hands the final position to Excalidraw once the wheel stops.
// When the copy has drifted far (edges uncovered, or blurry after zooming), it
// refreshes: one real redraw, then a new copy.
//
// The viewport maths mirrors Excalidraw's own wheel handling (zoom step, cap
// per event, zoom around the cursor, normalisation), so where you end up is
// exactly where Excalidraw would have put you.

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
  // Excalidraw zooms around the last pointer position it saw (from pointer moves, starting at 0,0), not the wheel event's.
  const pointer = { x: 0, y: 0 };
  const onPointerMove = (e: PointerEvent) => { pointer.x = e.clientX; pointer.y = e.clientY; };

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
    snapshot(src);
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
    await commit(api, target, false);
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
    await commit(api, target, true);
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
    perf.mark("viewport.snapshot-gesture", performance.now() - started, { events, refreshes }, 60_000);
  };

  const onWheel = (e: WheelEvent) => {
    if (!(e.target instanceof HTMLCanvasElement) || e.deltaMode !== 0 || e.buttons !== 0) return;
    const api = getApi();
    if (!api) return;
    const st = api.getAppState();
    if (st.editingTextElement || st.newElement) return; // mid-edit: leave it to Excalidraw
    if (!overlay && !begin(api)) return;
    e.preventDefault();
    e.stopPropagation();
    events++;
    const v = cur!;
    if (e.deltaY !== 0 && isZoom(e)) {
      cur = zoomStep(v, e.deltaY, pointer.x - st.offsetLeft, pointer.y - st.offsetTop);
    } else if (e.shiftKey) {
      const s = e.deltaY || e.deltaX;
      cur = e.ctrlKey || e.metaKey ? { ...v, scrollY: v.scrollY - s / v.zoom } : { ...v, scrollX: v.scrollX - s / v.zoom };
    } else {
      cur = { ...v, scrollX: v.scrollX - e.deltaX / v.zoom, scrollY: v.scrollY - e.deltaY / v.zoom };
    }
    place();
    window.clearTimeout(settle);
    settle = window.setTimeout(() => void end(), SETTLE_MS);
  };

  // Anything else (a click, a key, a resize) ends the gesture first.
  const finishNow = () => { if (overlay) void end(); };
  root.addEventListener("wheel", onWheel, { capture: true, passive: false });
  window.addEventListener("pointermove", onPointerMove, { capture: true, passive: true });
  root.addEventListener("pointerdown", finishNow, true);
  window.addEventListener("keydown", finishNow, true);
  window.addEventListener("resize", finishNow);
  return () => {
    root.removeEventListener("wheel", onWheel, { capture: true });
    window.removeEventListener("pointermove", onPointerMove, { capture: true });
    root.removeEventListener("pointerdown", finishNow, true);
    window.removeEventListener("keydown", finishNow, true);
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
