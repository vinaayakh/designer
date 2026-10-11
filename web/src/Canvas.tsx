import { memo, useEffect, useRef } from "react";
import { Excalidraw, MainMenu, convertToExcalidrawElements, viewportCoordsToSceneCoords } from "@excalidraw/excalidraw";
import type { AppState, BinaryFiles, ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
import type { ExcalidrawElement } from "@excalidraw/excalidraw/element/types";
import "@excalidraw/excalidraw/index.css";
import { attachSnapshotPan, snapshotPanEnabled } from "./snapshotPan";

export interface CanvasProps {
  onApi: (api: ExcalidrawImperativeAPI) => void;
  onChange: (elements: readonly ExcalidrawElement[], appState: AppState, files: BinaryFiles) => void;
  viewMode: boolean;
  /** Clicks in view mode (the frozen Rough Draft, previews): scene coordinates. */
  onViewClick?: (x: number, y: number, additive: boolean) => void;
  onPointerUp?: () => void;
  onPointerDown?: () => void;
}

/** The Excalidraw canvas. Scenes are swapped by the parent through the API. */
export const Canvas = memo(function Canvas({ onApi, onChange, viewMode, onViewClick, onPointerUp, onPointerDown }: CanvasProps) {
  const wrap = useRef<HTMLDivElement>(null);
  const apiRef = useRef<ExcalidrawImperativeAPI | null>(null);
  const handlers = useRef({ onViewClick, onPointerUp, onPointerDown, viewMode });
  handlers.current = { onViewClick, onPointerUp, onPointerDown, viewMode };

  useEffect(() => {
    const el = wrap.current;
    if (!el) return;
    // In view mode Excalidraw doesn't report clicks (spike 015), so we hit-test ourselves.
    let down: { x: number; y: number } | null = null;
    const pd = (e: PointerEvent) => {
      down = { x: e.clientX, y: e.clientY };
      handlers.current.onPointerDown?.();
    };
    const pu = (e: PointerEvent) => {
      handlers.current.onPointerUp?.();
      const h = handlers.current;
      const api = apiRef.current;
      if (!h.viewMode || !h.onViewClick || !api || !down) return;
      if (Math.hypot(e.clientX - down.x, e.clientY - down.y) > 4) return; // a pan, not a click
      if (!(e.target as HTMLElement).closest("canvas")) return;
      const st = api.getAppState();
      const p = viewportCoordsToSceneCoords({ clientX: e.clientX, clientY: e.clientY }, st);
      h.onViewClick(p.x, p.y, e.shiftKey || e.ctrlKey || e.metaKey);
    };
    el.addEventListener("pointerdown", pd, true);
    window.addEventListener("pointerup", pu, true);
    return () => {
      el.removeEventListener("pointerdown", pd, true);
      window.removeEventListener("pointerup", pu, true);
    };
  }, []);

  // Wheel gestures: by default a moving snapshot (snapshotPan.ts). With it switched off,
  // the fallback below still saves work:
  // Excalidraw redraws the whole scene for every wheel event, and precision touchpads send about
  // two per frame. Wheel events are held and handed over as one at the next animation frame
  // (Excalidraw draws after that frame's paint anyway, so nothing shows later). Pans add up
  // exactly; zooms merge only while the total stays within the step Excalidraw allows per event,
  // so zoom speed doesn't change.
  useEffect(() => {
    const el = wrap.current;
    if (!el) return;
    if (snapshotPanEnabled()) return attachSnapshotPan(el, () => apiRef.current);
    const ZOOM_STEP_CAP = 10; // Excalidraw clamps a wheel zoom to ±10 per event
    const ours = new WeakSet<Event>();
    let acc: { dx: number; dy: number; last: WheelEvent; zoom: boolean } | null = null;
    let raf = 0;
    const isZoom = (e: WheelEvent) => e.ctrlKey || e.metaKey || (apiRef.current?.getAppState().inputDevice === "mouse" && !e.shiftKey && e.deltaY !== 0);
    const dispatch = (dx: number, dy: number, like: WheelEvent) => {
      const ev = new WheelEvent("wheel", {
        deltaX: dx, deltaY: dy, deltaMode: 0, clientX: like.clientX, clientY: like.clientY, screenX: like.screenX, screenY: like.screenY,
        ctrlKey: like.ctrlKey, metaKey: like.metaKey, shiftKey: like.shiftKey, altKey: like.altKey, buttons: like.buttons,
        bubbles: true, cancelable: true, composed: true,
      });
      ours.add(ev);
      like.target?.dispatchEvent(ev);
    };
    const flush = () => {
      if (!acc) return;
      const a = acc;
      acc = null;
      dispatch(a.dx, a.dy, a.last);
    };
    const frame = () => {
      raf = 0;
      flush();
    };
    const onWheel = (e: WheelEvent) => {
      if (ours.has(e) || !(e.target instanceof HTMLCanvasElement) || e.deltaMode !== 0) return;
      if (!raf) raf = requestAnimationFrame(frame);
      e.preventDefault();
      e.stopPropagation();
      const zoom = isZoom(e);
      const fits = acc && acc.zoom === zoom && acc.last.ctrlKey === e.ctrlKey && acc.last.metaKey === e.metaKey && acc.last.shiftKey === e.shiftKey &&
        (!zoom || (Math.sign(acc.dy) === Math.sign(e.deltaY) && Math.abs(acc.dy + e.deltaY) <= ZOOM_STEP_CAP));
      if (acc && !fits) flush();
      acc = acc ? { dx: acc.dx + e.deltaX, dy: acc.dy + e.deltaY, last: e, zoom } : { dx: e.deltaX, dy: e.deltaY, last: e, zoom };
    };
    el.addEventListener("wheel", onWheel, { capture: true, passive: false });
    return () => {
      el.removeEventListener("wheel", onWheel, { capture: true });
      cancelAnimationFrame(raf);
    };
  }, []);

  return (
    <div ref={wrap} style={{ position: "absolute", inset: 0 }}>
      <Excalidraw
        onExcalidrawAPI={(api) => {
          if (api && api !== apiRef.current) {
            apiRef.current = api;
            // For end-to-end checks.
            (window as any).__designerCanvas = api;
            (window as any).__designerConvert = convertToExcalidrawElements;
            onApi(api);
          }
        }}
        onChange={onChange}
        viewModeEnabled={viewMode}
        UIOptions={{ canvasActions: { loadScene: false, saveToActiveFile: false, export: false, saveAsImage: true, toggleTheme: false, changeViewBackgroundColor: false } }}
        initialData={{ appState: { currentItemFontFamily: 5 } }}
      >
        <MainMenu>
          <MainMenu.DefaultItems.SaveAsImage />
          <MainMenu.DefaultItems.SearchMenu />
          <MainMenu.DefaultItems.Help />
          <MainMenu.DefaultItems.ClearCanvas />
        </MainMenu>
      </Excalidraw>
    </div>
  );
});
