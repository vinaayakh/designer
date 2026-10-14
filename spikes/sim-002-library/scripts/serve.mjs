// Minimal static server, standing in for the Go binary's embedded file server.
// No compression (localhost). Hashed /assets/* are immutable-cached; everything else is revalidated.
import { createServer } from "node:http";
import { readFile, stat } from "node:fs/promises";
import { extname, join, normalize } from "node:path";

const TYPES = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript",
  ".css": "text/css",
  ".woff2": "font/woff2",
  ".json": "application/json",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".woff": "font/woff",
  ".ttf": "font/ttf",
};

export function serve(dir, port) {
  const server = createServer(async (req, res) => {
    const url = new URL(req.url, "http://x");
    let p = normalize(decodeURIComponent(url.pathname)).replace(/^([/\\])+/, "");
    if (!p) p = "index.html";
    const file = join(dir, p);
    try {
      const s = await stat(file);
      if (!s.isFile()) throw new Error("not a file");
      const body = await readFile(file);
      const immutable = p.startsWith("assets") || p.startsWith("fonts");
      res.writeHead(200, {
        "content-type": TYPES[extname(file)] || "application/octet-stream",
        "cache-control": immutable ? "public, max-age=31536000, immutable" : "no-cache",
        "content-length": body.length,
      });
      res.end(body);
    } catch {
      res.writeHead(404);
      res.end("not found");
    }
  });
  return new Promise((resolve) => server.listen(port, "127.0.0.1", () => resolve(server)));
}

if (import.meta.url === `file:///${process.argv[1].replace(/\\/g, "/")}`) {
  const [dir, port] = process.argv.slice(2);
  await serve(dir, Number(port));
  console.log(`serving ${dir} on http://127.0.0.1:${port}`);
}
