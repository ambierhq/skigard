// Offline internal link and asset check over the built site (deploy/skigard). External links are checked by
// the repo's lychee workflow (links.yml), not here.
import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../deploy/skigard");
const BASE = "/skigard/";
const files = [];
async function walk(d) {
  for (const e of await fs.readdir(d, { withFileTypes: true })) {
    const p = path.join(d, e.name);
    if (e.isDirectory()) await walk(p);
    else files.push(p);
  }
}
await walk(root);
const redirects = (await fs.readFile(path.resolve(root, "../_redirects"), "utf8"))
  .split("\n").filter((l) => l && !l.startsWith("#")).map((l) => l.split(/\s+/)[0]);
const exists = async (p) => fs.stat(p).then(() => true, () => false);
let bad = 0;
for (const f of files.filter((f) => f.endsWith(".html"))) {
  const html = await fs.readFile(f, "utf8");
  for (const m of html.matchAll(/\b(?:href|src)="([^"#]+)(#[^"]*)?"/g)) {
    const url = m[1];
    if (/^(https?:|mailto:|data:)/.test(url) || !url.startsWith("/")) continue;
    if (!url.startsWith(BASE)) { console.error(`${path.relative(root, f)}: outside base: ${url}`); bad++; continue; }
    if (redirects.includes(url)) continue;
    const rel = url.slice(BASE.length);
    const target = path.join(root, rel);
    const ok = (await exists(target)) && (await fs.stat(target)).isFile() || (await exists(path.join(target, "index.html")));
    if (!ok) { console.error(`${path.relative(root, f)}: broken: ${url}`); bad++; }
  }
}
console.log(`check-links: ${files.length} files, ${bad} problems`);
process.exit(bad ? 1 : 0);
