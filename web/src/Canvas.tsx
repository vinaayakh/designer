import { memo, useEffect, useRef } from "react";
import { Excalidraw, MainMenu, convertToExcalidrawElements, viewportCoordsToSceneCoords } from "@excalidraw/excalidraw";
import type { AppState, BinaryFiles, ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
import type { ExcalidrawElement } from "@excalidraw/excalidraw/element/types";
import "@excalidraw/excalidraw/index.css";

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
