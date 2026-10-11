// Loads mermaid-to-excalidraw on first use instead of in the eager bundle (spike 015).
// @ts-expect-error resolved by a Vite alias
export const parseMermaidToExcalidraw = async (...args: unknown[]) => (await import("mermaid-to-excalidraw-real")).parseMermaidToExcalidraw(...args);
