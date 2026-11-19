#!/usr/bin/env node
// Launcher for the skigard binary. The binary ships in a per-platform package
// (@ambierhq/skigard-<os>-<cpu>, an optionalDependency); nothing is downloaded at install time.
// stdio is inherited, so `skigard approve` opens the controlling terminal itself, exactly as the
// native binary does. This file never passes anything to the binary through the environment.
"use strict";
const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const pkg = `@ambierhq/skigard-${process.platform}-${process.arch}`;
const exe = process.platform === "win32" ? "skigard.exe" : "skigard";

function fail(msg) {
  process.stderr.write(`skigard: ${msg}\n`);
  process.exit(2);
}

let binary;
try {
  binary = path.join(path.dirname(require.resolve(`${pkg}/package.json`)), "bin", exe);
} catch {
  fail(
    `no prebuilt binary for ${process.platform}-${process.arch} (expected package ${pkg}).\n` +
      "If you installed with --omit=optional or --no-optional, reinstall without it.\n" +
      "Other install options: https://github.com/ambierhq/skigard#install",
  );
}
if (!fs.existsSync(binary)) fail(`${pkg} is installed but ${binary} is missing; reinstall it`);
if (process.platform !== "win32") {
  try {
    fs.accessSync(binary, fs.constants.X_OK);
  } catch {
    try {
      fs.chmodSync(binary, 0o755); // some package managers drop the exec bit
    } catch (e) {
      fail(`cannot make ${binary} executable: ${e.message}`);
    }
  }
}

const r = spawnSync(binary, process.argv.slice(2), { stdio: "inherit", windowsHide: false });
if (r.error) fail(`cannot run ${binary}: ${r.error.message}`);
if (r.signal) {
  // Re-raise so the caller sees the same termination.
  process.kill(process.pid, r.signal);
  setTimeout(() => process.exit(128), 100);
} else {
  process.exit(r.status ?? 1);
}
