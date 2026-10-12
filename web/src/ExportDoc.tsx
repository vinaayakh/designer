import { useEffect, useRef, useState } from "react";
import { api, download } from "./api";

export type DocFormat = "docx" | "pdf";

/** What the Workspace hears about the document task. */
export type DocSignal = { seq: number; version?: number; failed?: string };

type Step =
  | { s: "checking" }
  | { s: "ask"; stale: boolean }
  | { s: "writing" }
  | { s: "failed"; message: string }
  | { s: "building" }
  | { s: "error"; message: string };

/**
 * Exports the arc42 document of one Production version. The Assistant writes
 * the parts stored data can't (responsibilities, runtime scenarios...) once
 * per version; everything else is built from the diagram, Brief, Rationale
 * and Gap Report.
 */
export function ExportDoc({ version, format, signal, images, onClose, onDone }: {
  version: number;
  format: DocFormat;
  signal: DocSignal | null;
  images: () => Promise<{ svg?: string; png?: string }>;
  onClose: () => void;
  onDone: (text: string) => void;
}) {
  const [step, setStep] = useState<Step>({ s: "checking" });
  const seq0 = useRef(signal?.seq ?? 0);

  const build = async () => {
    setStep({ s: "building" });
    try {
      const img = await images();
      const name = await exportDoc(version, format, img);
      onDone(format === "pdf" ? "Choose “Save as PDF” in the print dialog." : `Saved ${name}.`);
      onClose();
    } catch (e) {
      setStep({ s: "error", message: (e as Error).message });
    }
  };

  const write = async () => {
    try {
      seq0.current = signal?.seq ?? 0;
      await api.writeDocument(version);
      setStep({ s: "writing" });
    } catch (e) {
      setStep({ s: "failed", message: (e as Error).message });
    }
  };

  useEffect(() => {
    api.documentStatus(version).then(
      (st) => (st.written && !st.stale ? void build() : setStep({ s: "ask", stale: st.written && st.stale })),
      (e) => setStep({ s: "error", message: (e as Error).message }),
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [version, format]);

  useEffect(() => {
    if (step.s !== "writing" || !signal || signal.seq <= seq0.current) return;
    if (signal.failed) setStep({ s: "failed", message: signal.failed });
    else if (signal.version === version) void build();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signal]);

  const label = format === "pdf" ? "PDF" : "Word";
  return (
    <div className="modal-back" onClick={step.s === "writing" || step.s === "building" ? undefined : onClose}>
      <div className="modal narrow" onClick={(e) => e.stopPropagation()} role="dialog" aria-label="Export arc42 document">
        <h2>arc42 document · {label}</h2>
        <p className="lede" style={{ marginTop: 6 }}>Production Diagram v{version}</p>

        {step.s === "checking" && <p className="muted">Checking…</p>}

        {step.s === "ask" && (
          <>
            {step.stale ? (
              <p>The diagram changed since the document text was written. The Assistant can rewrite it so the responsibilities and runtime scenarios match.</p>
            ) : (
              <p>
                The Assistant writes what the stored design can't tell: each component's responsibilities, the main runtime scenarios,
                crosscutting concepts and quality scenarios. It takes about a minute and is saved with this version.
              </p>
            )}
            <p className="muted small">Everything else comes from the Design Brief, the diagram, the Design Rationale, the Register (requirements, assumptions, decisions, risks) and the latest Gap Report.</p>
            <div className="row">
              <button onClick={onClose}>Cancel</button>
              <span className="spacer" />
              <button onClick={() => void build()}>{step.stale ? "Use the old text" : "Export without it"}</button>
              <button className="primary" onClick={() => void write()}>{step.stale ? "Rewrite and export" : "Write and export"}</button>
            </div>
          </>
        )}

        {step.s === "writing" && (
          <>
            <p><span className="spinner" aria-hidden /> The Assistant is writing the document text…</p>
            <p className="muted small">Its progress shows in the Assistant tab. The export starts as soon as it's saved.</p>
            <div className="row"><span className="spacer" /><button onClick={onClose}>Hide</button></div>
          </>
        )}

        {step.s === "failed" && (
          <>
            <p className="bad">{step.message}</p>
            <div className="row">
              <button onClick={onClose}>Cancel</button>
              <span className="spacer" />
              <button onClick={() => void build()}>Export without it</button>
              <button className="primary" onClick={() => void write()}>Try again</button>
            </div>
          </>
        )}

        {step.s === "building" && <p><span className="spinner" aria-hidden /> Building the document…</p>}

        {step.s === "error" && (
          <>
            <p className="bad">{step.message}</p>
            <div className="row"><span className="spacer" /><button onClick={onClose}>Close</button></div>
          </>
        )}
      </div>
    </div>
  );
}

async function exportDoc(version: number, format: DocFormat, img: { svg?: string; png?: string }): Promise<string> {
  if (format === "docx") {
    const { blob, name } = await download(`/api/export/${version}/docx`, { png: img.png });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    a.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 2000);
    return name;
  }
  const { blob, name } = await download(`/api/export/${version}/html`, { svg: img.svg });
  await printHTML(await blob.text(), name.replace(/\.html$/, ""));
  return name;
}

/**
 * Prints the document from a hidden frame; the browser's "Save as PDF" makes
 * the file. The frame is sandboxed without scripts: the page is static.
 */
function printHTML(html: string, title: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const frame = document.createElement("iframe");
    frame.setAttribute("sandbox", "allow-same-origin allow-modals");
    frame.style.cssText = "position:fixed;right:0;bottom:0;width:0;height:0;border:0;visibility:hidden";
    frame.srcdoc = html;
    frame.onload = () => {
      const w = frame.contentWindow;
      if (!w) return reject(new Error("couldn't open the print view"));
      const before = document.title;
      document.title = title; // the default file name in "Save as PDF"
      let done = false;
      const cleanup = () => {
        if (done) return;
        done = true;
        document.title = before;
        window.setTimeout(() => frame.remove(), 500);
        resolve();
      };
      w.addEventListener("afterprint", cleanup, { once: true });
      // Fonts and the inlined SVG need a moment to lay out.
      window.setTimeout(() => {
        w.focus();
        w.print();
        // Some browsers don't fire afterprint for sandboxed frames.
        window.setTimeout(cleanup, 60_000);
      }, 300);
    };
    document.body.appendChild(frame);
  });
}
