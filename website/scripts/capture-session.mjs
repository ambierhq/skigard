// Records a real skigard session for the landing page terminal (website/src/data/session.json).
// Builds the current binary, runs examples/hello-skill in a throwaway HOME (local path form, no network) and
// drives `approve` through a real pty (scripts/pty_approve.py; approve is never run without one).
// Needs go and python3. The site build itself does not: it only reads the committed JSON.
// Usage: node scripts/capture-session.mjs [--check]   (--check fails when the committed file is out of date)
import { execFileSync } from "node:child_process";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, "../..");
const out = path.resolve(here, "../src/data/session.json");
const tmp = await fs.mkdtemp(path.join(os.tmpdir(), "skigard-session-"));
const home = path.join(tmp, "home");
const bin = path.join(tmp, process.platform === "win32" ? "skigard.exe" : "skigard");
await fs.mkdir(path.join(home, ".claude"), { recursive: true });
await fs.mkdir(path.join(home, ".pi"), { recursive: true });
await fs.mkdir(path.join(home, ".omp"), { recursive: true }); // detection also looks at PATH, so name every harness we expect

// Agent markers must not leak in: approve (correctly) refuses inside an agent harness.
// A fixed PATH so harness detection (which looks for binaries such as pi and omp) does not depend on this machine.
const env = { PATH: "/usr/bin:/bin", HOME: home, USERPROFILE: home, SKIGARD_HOME: path.join(home, "skigard"), TERM: "dumb", GOCACHE: process.env.GOCACHE ?? "" };
if (!env.GOCACHE) delete env.GOCACHE;
const run = (cmd, args, opts = {}) => execFileSync(cmd, args, { cwd: repo, env, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"], ...opts });

run("go", ["build", "-o", bin, "./cmd/skigard"], { env: { ...process.env } });
const version = run(bin, ["version"]).trim().replace(/^skigard\s+/, "").replace(/-dirty$/, "");

const await_realpath = { home: await fs.realpath(home) }; // macOS /var -> /private/var
const clean = (s) =>
  s
    .replaceAll("\r", "")
    .replaceAll(repo, "<repo>")
    .replaceAll(await_realpath.home, "~")
    .replaceAll(home, "~")
    .replaceAll(tmp, "~")
    .replace(/\x1b\[[0-9;?]*[A-Za-z]/g, "");

const lines = [];
const push = (t, c) => lines.push({ t, c });
const kind = (l) => (/^(approved|ok:|installed|quarantined)/.test(l) ? "ok" : /^Type \S+ to confirm/.test(l) ? "warn" : "out");
const add = (text, keep = () => true) => {
  for (const l of clean(text).split("\n")) if (l.trim() && keep(l)) push(l.trimEnd(), kind(l));
};

const skigard = (args) => run(bin, args);
// `add` installs by default; the landing page shows the fetch-and-scan step on its own (--quarantine-only), which is
// deterministic: it never prompts, never needs a terminal and does not depend on whether an agent runs it.
push("$ skigard add ./examples/hello-skill --quarantine-only", "cmd");
const addOut = skigard(["add", "./examples/hello-skill", "--quarantine-only"]);
add(addOut, (l) => !/^(next:|\s+skigard )/.test(l));

push("$ skigard approve hello-skill", "cmd");
const approve = execFileSync("python3", [path.join(repo, "scripts/pty_approve.py"), bin, env.SKIGARD_HOME, "hello-skill", "hello-skill"], { cwd: repo, env, encoding: "utf8" });
const [code, ...rest] = approve.split("\n");
if (code.trim() !== "0") throw new Error(`approve exited ${code}:\n${approve}`);
const text = clean(rest.join("\n"));
const keep = [];
let inChanges = false;
for (const l of text.split("\n")) {
  if (/^changes:/.test(l)) { inChanges = true; keep.push(l); continue; }
  if (inChanges && /^\s+\+\+\+ added:/.test(l)) { keep.push(l, "  … (file contents shown here, trimmed)"); continue; }
  if (inChanges && /^\s+\+/.test(l)) continue;
  if (/^(evidence:|next:|content hash:|note:)/.test(l)) continue; // the hash and note were shown by add
  if (l.trim()) keep.push(l);
}
for (const l of keep) push(l.trimEnd(), kind(l));

push("$ skigard install hello-skill", "cmd");
add(skigard(["install", "hello-skill"]), (l) => !/^info /.test(l));
push("$ skigard verify", "cmd");
add(skigard(["verify"]));

await fs.rm(tmp, { recursive: true, force: true });
const leaked = lines.filter((l) => l.t.includes(repo) || l.t.includes(os.homedir()) || l.t.includes("/var/folders") || l.t.includes(os.userInfo().username));
if (leaked.length) throw new Error(`session still contains local paths: ${JSON.stringify(leaked)}`);

const json = JSON.stringify({ version, lines }, null, 2) + "\n";
if (process.argv.includes("--check")) {
  const have = await fs.readFile(out, "utf8").catch(() => "");
  // The version string carries the commit; compare the transcript only so unrelated commits do not fail CI.
  const same = JSON.stringify(JSON.parse(have || "{}").lines) === JSON.stringify(lines);
  if (!same) {
    const want = JSON.parse(have || "{}").lines ?? [];
    for (let i = 0; i < Math.max(want.length, lines.length); i++) {
      if (want[i]?.t !== lines[i]?.t) console.error(`line ${i + 1}:\n  committed: ${want[i]?.t}\n  current:   ${lines[i]?.t}`);
    }
    console.error("src/data/session.json is out of date: run npm run capture");
    process.exit(1);
  }
  console.log("session.json matches the current binary");
} else {
  await fs.mkdir(path.dirname(out), { recursive: true });
  await fs.writeFile(out, json);
  console.log(`capture-session: ${lines.length} lines from skigard ${version}`);
}
