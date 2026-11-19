// Build-time sync: the repo's docs/ markdown, registry.toml and brand mark are the single source of truth.
// Nothing here is committed; see .gitignore.
import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const site = path.resolve(here, "..");
const repo = path.resolve(site, "..");
const BASE = "/skigard";
const GH = "https://github.com/ambierhq/skigard";
const SITE = "https://ambierhq.com";

const outDocs = path.join(site, "src/content/docs/docs");
const EXCLUDE = new Set(["ops-log.md"]); // docs/maintenance is a directory and is skipped too

async function listDocs() {
  const out = [];
  const top = await fs.readdir(path.join(repo, "docs"), { withFileTypes: true });
  for (const e of top) {
    if (e.isFile() && e.name.endsWith(".md") && !EXCLUDE.has(e.name)) out.push(e.name);
  }
  const inst = await fs.readdir(path.join(repo, "docs/install")).catch(() => []);
  for (const n of inst) if (n.endsWith(".md")) out.push(`install/${n}`);
  return out.sort();
}

// "install/README.md" -> slug "install", "install/macos.md" -> "install/macos", "faq.md" -> "faq"
const slugOf = (rel) => rel.replace(/\.md$/, "").replace(/(^|\/)README$/, "").replace(/\/$/, "");

function rewriteLinks(md, rel) {
  const dir = path.posix.dirname(rel);
  return md.replace(/\]\(([^)\s#]+)(#[^)\s]*)?\)/g, (all, target, hash = "") => {
    if (/^[a-z][a-z0-9+.-]*:/i.test(target)) return all;
    const resolved = path.posix.normalize(path.posix.join("docs", dir, target)); // relative to the repo root
    if (resolved.startsWith("docs/") && resolved.endsWith(".md")) {
      const slug = slugOf(resolved.slice("docs/".length));
      return `](${BASE}/docs/${slug}${slug ? "/" : ""}${hash})`;
    }
    return `](${GH}/blob/main/${resolved}${hash})`;
  });
}

const yamlStr = (s) => JSON.stringify(s);

function describe(md, title) {
  for (const para of md.split(/\n\s*\n/)) {
    const t = para.trim();
    if (!t || /^(#|```|\||[-*>]|\d+\.)/.test(t)) continue;
    const plain = t.replace(/\[([^\]]+)\]\([^)]*\)/g, "$1").replace(/[`*_]/g, "").replace(/\s+/g, " ");
    return plain.length <= 155 ? plain : plain.slice(0, 152).replace(/\s+\S*$/, "") + "...";
  }
  return title;
}

async function syncDocs() {
  await fs.rm(outDocs, { recursive: true, force: true });
  const full = [];
  for (const rel of await listDocs()) {
    let md = await fs.readFile(path.join(repo, "docs", rel), "utf8");
    const h1 = md.match(/^#\s+(.*)\n+/);
    const title = h1 ? h1[1].replace(/[`*]/g, "").trim() : slugOf(rel);
    md = md.replace(/^#\s+.*\n+/, ""); // Starlight renders the title as <h1>
    const description = describe(md, title);
    const slug = slugOf(rel);
    const file = rel.endsWith("README.md") ? path.join(path.dirname(rel), "index.md") : rel;
    const dest = path.join(outDocs, file);
    await fs.mkdir(path.dirname(dest), { recursive: true });
    const front = `---\ntitle: ${yamlStr(title)}\ndescription: ${yamlStr(description)}\neditUrl: ${GH}/edit/main/docs/${rel}\n---\n\n`;
    await fs.writeFile(dest, front + rewriteLinks(md, rel));
    full.push({ slug, title, description, md });
  }
  return full;
}

// Minimal reader for the registry's one-line inline tables (no new dependency, see AGENTS.md).
function parseRegistry(text) {
  const out = [];
  for (const line of text.split("\n")) {
    const m = line.match(/^\s*\{(.*)\},?\s*$/);
    if (!m) continue;
    const entry = {};
    for (const kv of m[1].matchAll(/(\w+)\s*=\s*("(?:[^"\\]|\\.)*"|\[[^\]]*\]|true|false)/g)) {
      entry[kv[1]] = JSON.parse(kv[2]);
    }
    if (entry.id && entry.name) out.push(entry);
  }
  return out;
}

async function syncHarnesses() {
  const text = await fs.readFile(path.join(repo, "internal/harness/registry.toml"), "utf8");
  const list = parseRegistry(text).map((h) => ({
    id: h.id,
    name: h.name,
    verified: h.verified === true,
    global: h.global ?? "",
    project: h.project ?? "",
  }));
  if (list.length < 10) throw new Error(`sync-docs: parsed only ${list.length} harnesses from registry.toml`);
  await fs.writeFile(path.join(site, "src/data/harnesses.json"), JSON.stringify(list, null, 1) + "\n");
  return list;
}

async function syncBrand() {
  const brand = path.join(repo, "assets/brand");
  const copy = async (from, to) => fs.copyFile(path.join(brand, from), path.join(site, to));
  await fs.mkdir(path.join(site, "src/assets"), { recursive: true });
  await copy("mark.svg", "src/assets/mark.svg");
  await copy("mark.svg", "public/logo.svg");
  await copy("favicon.svg", "public/favicon.svg");
  await copy("favicon.ico", "public/favicon.ico");
  await copy("apple-touch-icon.png", "public/apple-touch-icon.png");
  await copy("social-preview.png", "public/og.png");
}

async function llms(docs, harnesses) {
  const readme = await fs.readFile(path.join(repo, "README.md"), "utf8");
  const verified = harnesses.filter((h) => h.verified).map((h) => h.name).join(", ");
  const index = `# skigard

> A lockfile and review gate for agent skills. skigard fetches skills (folders with a SKILL.md) into quarantine, scans them, optionally has an agent you already use review them, makes a human approve the exact bytes in a real terminal, pins the approval to a content hash in a lockfile, and installs them into the agent harnesses it detects. A project by Ambier Oy, Finland. MIT licensed, Go, one static binary, no telemetry. Runs on macOS, Linux and Windows.

Agents cannot approve: \`approve\` reads the controlling terminal only and has no --yes flag. The scanner is heuristic and an agent review can be fooled, so a pass is not a safety certificate.

## Docs
${docs.map((d) => `- [${d.title}](${SITE}${BASE}/docs/${d.slug}${d.slug ? "/" : ""}): ${d.description}`).join("\n")}

## Optional
- [Full documentation in one file](${SITE}${BASE}/llms-full.txt)
- [Source code](${GH})
- [Releases](${GH}/releases)
- Harnesses verified against their own docs or a real machine: ${verified}
`;
  const fullDocs = docs.map((d) => `\n\n---\n\n# ${d.title}\n\nSource: ${SITE}${BASE}/docs/${d.slug}${d.slug ? "/" : ""}\n\n${d.md}`).join("");
  await fs.writeFile(path.join(site, "public/llms.txt"), index);
  await fs.writeFile(path.join(site, "public/llms-full.txt"), `${index}\n---\n\n# README\n\n${readme}${fullDocs}\n`);
}

const docs = await syncDocs();
const harnesses = await syncHarnesses();
await syncBrand();
await llms(docs, harnesses);
console.log(`sync-docs: ${docs.length} docs, ${harnesses.length} harnesses`);
