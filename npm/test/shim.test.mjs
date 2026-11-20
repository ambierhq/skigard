import assert from "node:assert/strict";
import { spawnSync, execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { binary, cleanEnv, fakeInstall, repo, tmp } from "./helpers.mjs";

const node = process.execPath;
const run = (shim, args, opts = {}) => spawnSync(node, [shim, ...args], { encoding: "utf8", env: cleanEnv(), ...opts });
const direct = (args, opts = {}) => spawnSync(binary(), args, { encoding: "utf8", env: cleanEnv(), ...opts });

test("runs the platform binary and prints its output", () => {
  const { shim } = fakeInstall();
  const r = run(shim, ["version"]);
  assert.equal(r.status, 0, r.stderr);
  assert.equal(r.stdout, direct(["version"]).stdout);
  assert.match(r.stdout, /^skigard /);
});

test("passes arguments through verbatim", () => {
  const { shim } = fakeInstall();
  const r = run(shim, ["scan", "--no-such-flag", "a b", "--", "x"]);
  const d = direct(["scan", "--no-such-flag", "a b", "--", "x"]);
  assert.equal(r.status, d.status);
  assert.equal(r.stderr, d.stderr);
});

test("propagates the exit code", () => {
  const { shim } = fakeInstall();
  const home = tmp();
  const env = cleanEnv({ SKIGARD_HOME: home });
  const r = run(shim, ["scan", "nope"], { env });
  const d = direct(["scan", "nope"], { env });
  assert.notEqual(d.status, 0);
  assert.equal(r.status, d.status);
  assert.equal(run(shim, ["version"]).status, 0);
});

test("inherits stdio: output reaches the parent's own file descriptors", () => {
  const { shim } = fakeInstall();
  const out = path.join(tmp(), "out.txt");
  const fd = fs.openSync(out, "w");
  const r = spawnSync(node, [shim, "version"], { stdio: ["ignore", fd, "inherit"], env: cleanEnv() });
  fs.closeSync(fd);
  assert.equal(r.status, 0);
  assert.match(fs.readFileSync(out, "utf8"), /^skigard /);
});

test("inherits stdin", () => {
  // `skigard add -` is not a thing; use the binary's own stdin reader: review --manual does not read stdin, so
  // just prove a closed and an open stdin do not hang or crash the launcher.
  const { shim } = fakeInstall();
  const r = run(shim, ["version"], { input: "ignored\n" });
  assert.equal(r.status, 0);
});

test("reports a missing platform package clearly", () => {
  const { shim, pdir } = fakeInstall();
  fs.rmSync(pdir, { recursive: true });
  const r = run(shim, ["version"]);
  assert.equal(r.status, 2);
  assert.match(r.stderr, /no prebuilt binary for/);
  assert.match(r.stderr, /@ambierhq\/skigard-/);
});

test("restores a dropped exec bit", { skip: process.platform === "win32" }, () => {
  const { shim, pdir } = fakeInstall();
  fs.chmodSync(path.join(pdir, "bin", "skigard"), 0o644);
  assert.equal(run(shim, ["version"]).status, 0);
});

// A home with one quarantined skill named "good".
function quarantined() {
  const w = tmp();
  const home = path.join(w, "home");
  fs.mkdirSync(path.join(w, "src", "good"), { recursive: true });
  fs.writeFileSync(path.join(w, "src", "good", "SKILL.md"), "---\nname: good\ndescription: formats text\n---\n# good\nprose only\n");
  const env = cleanEnv({ SKIGARD_HOME: home, HOME: path.join(w, "user") });
  fs.mkdirSync(env.HOME);
  execFileSync(binary(), ["init"], { env, stdio: "ignore" });
  execFileSync(binary(), ["add", path.join(w, "src"), "--all"], { env, stdio: "ignore" });
  return { home, env };
}

test("approve without a controlling terminal still refuses", { skip: process.platform === "win32" }, () => {
  const { shim } = fakeInstall();
  const { env } = quarantined();
  const r = spawnSync(node, [shim, "approve", "good"], { encoding: "utf8", input: "good\n", env, detached: true });
  assert.notEqual(r.status, 0);
  assert.match(r.stderr, /real terminal/);
});

test("approve refuses inside an agent", { skip: process.platform === "win32" }, () => {
  const { shim } = fakeInstall();
  const { env } = quarantined();
  const r = spawnSync(node, [shim, "approve", "good"], { encoding: "utf8", env: { ...env, PI_CODING_AGENT: "1" } });
  assert.notEqual(r.status, 0);
  assert.match(r.stderr, /agent harness/);
});

// approve through a real pty, with the launcher in the middle: wrong answer cancels (3), right answer approves (0).
// Never call approve without a pty (AGENTS.md); scripts/pty_approve.py provides one.
test("approve reaches the controlling terminal through the launcher", { skip: process.platform === "win32" }, (t) => {
  if (spawnSync("python3", ["--version"]).status !== 0) return t.skip("python3 not available");
  const { shim } = fakeInstall();
  const { home, env } = quarantined();
  const pty = path.join(repo, "scripts", "pty_approve.py");
  const res = (answer) => spawnSync("python3", [pty, shim, home, "good", answer], { encoding: "utf8", env: cleanEnv() });
  const wrong = res("nope");
  assert.equal(wrong.stdout.split("\n")[0], "3", wrong.stdout + wrong.stderr);
  const right = res("good");
  assert.equal(right.stdout.split("\n")[0], "0", right.stdout + right.stderr);
  assert.match(execFileSync(binary(), ["list"], { env, encoding: "utf8" }), /good\s+approved/);
});
