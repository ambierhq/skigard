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

// Pages synced from docs/, in sidebar order, with the description used for <meta>.
const DOCS = [
  ["threat-model", "Threat model", "What skigard promises, what it does not, and how an attacker could get past it."],
  ["review", "Agent review", "How skigard asks an agent you already use to review a quarantined skill, with tools off and the OS sandbox on."],
  ["lockfile", "Lockfile format", "skigard.lock: content hashes, hash versions, the executable set and how to upgrade a lockfile."],
  ["windows", "Windows notes", "File locations, console approval and what differs on Windows."],
  ["harnesses", "Supported harnesses", "Every agent harness skigard detects, with paths and verification status."],
  ["rules", "Scanner rules", "Every scanner rule, its severity and what it looks for."],
];

const outDocs = path.join(site, "src/content/docs/docs");

function rewriteLinks(md) {
  return md
    .replace(/\]\(([\w-]+)\.md(#[\w-]+)?\)/g, (_, n, h) => `](${BASE}/docs/${n}/${h ?? ""})`)
    .replace(/\]\((?:\.\.\/)+([^)#\s]+)(#[^)\s]*)?\)/g, (_, p, h) => `](${GH}/blob/main/${p}${h ?? ""})`);
}

const yamlStr = (s) => JSON.stringify(s);

async function syncDocs() {
  await fs.mkdir(outDocs, { recursive: true });
  for (const [slug] of DOCS) await fs.rm(path.join(outDocs, `${slug}.md`), { force: true });
  const full = [];
  for (const [slug, title, description] of DOCS) {
    const src = path.join(repo, "docs", `${slug}.md`);
    let md;
    try {
      md = await fs.readFile(src, "utf8");
    } catch {
      console.warn(`sync-docs: missing docs/${slug}.md, skipped`);
      continue;
    }
    md = md.replace(/^#\s+.*\n+/, ""); // Starlight renders the title as <h1>
    const body = rewriteLinks(md);
    const front = `---\ntitle: ${yamlStr(title)}\ndescription: ${yamlStr(description)}\neditUrl: ${GH}/edit/main/docs/${slug}.md\n---\n\n`;
    await fs.writeFile(path.join(outDocs, `${slug}.md`), front + body);
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
  const mark = await fs.readFile(path.join(repo, "assets/brand/provisional-mark-option-b.svg"), "utf8").catch(() => null);
  if (!mark) throw new Error("sync-docs: assets/brand mark not found");
  await fs.writeFile(path.join(site, "public/favicon.svg"), mark);
  await fs.writeFile(path.join(site, "public/logo.svg"), mark);
  await fs.mkdir(path.join(site, "src/assets"), { recursive: true });
  await fs.writeFile(path.join(site, "src/assets/mark.svg"), mark);
  // Social card: the mark on the brand background, as an SVG (rasterised by Task F's brand set when it lands).
  const inner = mark.replace(/^<svg[^>]*>/, "").replace(/<\/svg>\s*$/, "");
  const og = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 630" width="1200" height="630"><rect width="1200" height="630" fill="#16130E"/><g transform="translate(90 165) scale(1.2)">${inner}</g><text x="460" y="300" font-family="Georgia,serif" font-size="96" fill="#F4EEE2">skigard</text><text x="460" y="375" font-family="Helvetica,Arial,sans-serif" font-size="36" fill="#B7A98C">A lockfile and review gate for agent skills</text><text x="460" y="540" font-family="Helvetica,Arial,sans-serif" font-size="28" fill="#EFA226">ambierhq.com/skigard</text></svg>\n`;
  await fs.writeFile(path.join(site, "public/og.svg"), og);
}

async function llms(docs, harnesses) {
  const readme = await fs.readFile(path.join(repo, "README.md"), "utf8");
  const verified = harnesses.filter((h) => h.verified).map((h) => h.name).join(", ");
  const index = `# skigard

> A lockfile and review gate for agent skills. skigard fetches skills (folders with a SKILL.md) into quarantine, scans them, optionally has an agent you already use review them, makes a human approve the exact bytes in a real terminal, pins the approval to a content hash in a lockfile, and installs them into the agent harnesses it detects. A project by Ambier Oy, Finland. MIT licensed, Go, one static binary, no telemetry. Runs on macOS, Linux and Windows.

Agents cannot approve: \`approve\` reads the controlling terminal only and has no --yes flag. The scanner is heuristic and an agent review can be fooled, so a pass is not a safety certificate.

## Docs
- [Install](${SITE}${BASE}/docs/install/): shell, PowerShell, npx, apt, pacman, Homebrew, Nix, go install
- [Getting started](${SITE}${BASE}/docs/getting-started/): detect, add, review, approve, install, verify
- [Concepts](${SITE}${BASE}/docs/concepts/): quarantine, scan, review, approve, lockfile, harnesses
- [Using skigard in CI](${SITE}${BASE}/docs/ci/)
${docs.map((d) => `- [${d.title}](${SITE}${BASE}/docs/${d.slug}/): ${d.description}`).join("\n")}

## Optional
- [Full documentation in one file](${SITE}${BASE}/llms-full.txt)
- [Source code](${GH})
- [Releases](${GH}/releases)
- Harnesses verified against their own docs or a real machine: ${verified}
`;
  const fullDocs = docs.map((d) => `\n\n---\n\n# ${d.title}\n\nSource: ${SITE}${BASE}/docs/${d.slug}/\n\n${d.md}`).join("");
  await fs.writeFile(path.join(site, "public/llms.txt"), index);
  await fs.writeFile(path.join(site, "public/llms-full.txt"), `${index}\n---\n\n# README\n\n${readme}${fullDocs}\n`);
}

const docs = await syncDocs();
const harnesses = await syncHarnesses();
await syncBrand();
await llms(docs, harnesses);
console.log(`sync-docs: ${docs.length} docs, ${harnesses.length} harnesses`);
