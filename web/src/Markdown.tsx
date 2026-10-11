// A small Markdown renderer for Knowledge Base Records and Assistant replies:
// headings, paragraphs, bullet and numbered lists, blockquotes, fenced code,
// tables, **bold**, *italic*, `code` and links. Everything renders as React
// text, so nothing in the input is treated as HTML.
import { Fragment, type ReactNode } from "react";

function inline(s: string, key: string): ReactNode[] {
  const out: ReactNode[] = [];
  const re = /(\*\*[^*]+\*\*|__[^_]+__|\*[^*\s][^*]*\*|`[^`]+`|\[[^\]]+\]\([^)\s]+\))/g;
  let last = 0;
  let m: RegExpExecArray | null;
  let i = 0;
  while ((m = re.exec(s))) {
    if (m.index > last) out.push(s.slice(last, m.index));
    const t = m[0];
    const k = `${key}-${i++}`;
    if (t.startsWith("**") || t.startsWith("__")) out.push(<b key={k}>{inline(t.slice(2, -2), k)}</b>);
    else if (t.startsWith("`")) out.push(<code key={k}>{t.slice(1, -1)}</code>);
    else if (t.startsWith("[")) {
      const [, text, href] = /\[([^\]]+)\]\(([^)\s]+)\)/.exec(t)!;
      const safe = /^https?:\/\//i.test(href) ? href : undefined;
      out.push(safe ? <a key={k} href={safe} target="_blank" rel="noreferrer">{text}</a> : <Fragment key={k}>{text}</Fragment>);
    } else out.push(<i key={k}>{t.slice(1, -1)}</i>);
    last = m.index + t.length;
  }
  if (last < s.length) out.push(s.slice(last));
  return out;
}

const isTableRow = (l: string) => /^\s*\|.*\|\s*$/.test(l);
const cells = (l: string) => l.trim().replace(/^\||\|$/g, "").split("|").map((c) => c.trim());
const BULLET = /^\s*[-*+] /;
const NUMBERED = /^\s*\d+[.)] /;

export function Markdown({ text, className = "md" }: { text: string; className?: string }) {
  const blocks: ReactNode[] = [];
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  let i = 0;
  let k = 0;
  while (i < lines.length) {
    const l = lines[i];
    if (!l.trim()) { i++; continue; }
    if (l.trimStart().startsWith("```")) {
      const code: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trimStart().startsWith("```")) code.push(lines[i++]);
      i++;
      blocks.push(<pre key={k++}><code>{code.join("\n")}</code></pre>);
      continue;
    }
    const h = /^(#{1,6}) (.*)$/.exec(l);
    if (h) {
      const level = h[1].length;
      blocks.push(level <= 2 ? <h2 key={k++}>{inline(h[2], `h${k}`)}</h2> : <h3 key={k++}>{inline(h[2], `h${k}`)}</h3>);
      i++;
      continue;
    }
    if (/^\s*(---|\*\*\*|___)\s*$/.test(l)) { blocks.push(<hr key={k++} />); i++; continue; }
    if (l.startsWith(">")) {
      const q: string[] = [];
      while (i < lines.length && lines[i].startsWith(">")) q.push(lines[i++].replace(/^> ?/, ""));
      blocks.push(<blockquote key={k++}>{q.map((x, j) => <Fragment key={j}>{inline(x, `q${k}${j}`)}<br /></Fragment>)}</blockquote>);
      continue;
    }
    if (isTableRow(l) && i + 1 < lines.length && /^\s*\|?[\s:|-]+\|?\s*$/.test(lines[i + 1]) && lines[i + 1].includes("-")) {
      const head = cells(l);
      i += 2;
      const rows: string[][] = [];
      while (i < lines.length && isTableRow(lines[i])) rows.push(cells(lines[i++]));
      blocks.push(
        <div className="table-wrap" key={k++}>
          <table>
            <thead><tr>{head.map((c, j) => <th key={j}>{inline(c, `th${k}${j}`)}</th>)}</tr></thead>
            <tbody>{rows.map((r, ri) => <tr key={ri}>{r.map((c, j) => <td key={j}>{inline(c, `td${k}${ri}${j}`)}</td>)}</tr>)}</tbody>
          </table>
        </div>,
      );
      continue;
    }
    if (BULLET.test(l) || NUMBERED.test(l)) {
      const ordered = NUMBERED.test(l);
      const re = ordered ? NUMBERED : BULLET;
      const items: string[] = [];
      while (i < lines.length && (re.test(lines[i]) || (/^\s{2,}\S/.test(lines[i]) && items.length))) {
        if (re.test(lines[i])) items.push(lines[i].replace(re, ""));
        else items[items.length - 1] += " " + lines[i].trim();
        i++;
      }
      const lis = items.map((x, j) => <li key={j}>{inline(x, `l${k}${j}`)}</li>);
      blocks.push(ordered ? <ol key={k++}>{lis}</ol> : <ul key={k++}>{lis}</ul>);
      continue;
    }
    const p: string[] = [];
    while (i < lines.length && lines[i].trim() && !/^(#{1,6} |>|```)/.test(lines[i].trimStart()) && !BULLET.test(lines[i]) && !NUMBERED.test(lines[i]) && !isTableRow(lines[i])) p.push(lines[i++]);
    blocks.push(<p key={k++}>{p.map((x, j) => <Fragment key={j}>{j > 0 && <br />}{inline(x, `p${k}${j}`)}</Fragment>)}</p>);
  }
  return <div className={className}>{blocks}</div>;
}
