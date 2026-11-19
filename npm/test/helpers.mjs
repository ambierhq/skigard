import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import zlib from "node:zlib";

export const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
export const exe = process.platform === "win32" ? "skigard.exe" : "skigard";
export const tmp = () => fs.mkdtempSync(path.join(os.tmpdir(), "skigard-npm-"));

// The binary under test: SKIGARD_TEST_BINARY (e.g. from a GoReleaser snapshot), else built from this checkout.
let built;
export function binary() {
  if (process.env.SKIGARD_TEST_BINARY) return path.resolve(process.env.SKIGARD_TEST_BINARY);
  if (!built) {
    built = path.join(tmp(), exe);
    execFileSync("go", ["build", "-o", built, "./cmd/skigard"], { cwd: repo, stdio: "inherit" });
  }
  return built;
}

// Environment without agent markers (approve refuses inside an agent, correctly).
export function cleanEnv(extra = {}) {
  const drop = /^(PI_|CLAUDE|CODEX|GEMINI|OPENCODE|CURSOR|GOOSE|AMP_)/;
  const env = Object.fromEntries(Object.entries(process.env).filter(([k]) => !drop.test(k)));
  return { ...env, ...extra };
}

// A tree shaped like an installed package: <dir>/bin/skigard.js (the real shim) next to
// <dir>/node_modules/@ambierhq/skigard-<platform>/ holding the binary under test.
export function fakeInstall(bin = binary()) {
  const dir = tmp();
  fs.mkdirSync(path.join(dir, "bin"), { recursive: true });
  const shim = path.join(dir, "bin", "skigard.js");
  fs.copyFileSync(path.join(repo, "npm", "bin", "skigard.js"), shim);
  fs.chmodSync(shim, 0o755);
  const pdir = path.join(dir, "node_modules", "@ambierhq", `skigard-${process.platform}-${process.arch}`);
  fs.mkdirSync(path.join(pdir, "bin"), { recursive: true });
  fs.writeFileSync(path.join(pdir, "package.json"), JSON.stringify({ name: path.basename(pdir), version: "0.0.0" }));
  fs.copyFileSync(bin, path.join(pdir, "bin", exe));
  fs.chmodSync(path.join(pdir, "bin", exe), 0o755);
  return { dir, shim, pdir };
}

// --- tiny archive writers for npm-pack tests ---
export function makeTarGz(name, data) {
  const h = Buffer.alloc(512);
  h.write(name, 0);
  h.write("0000755\0", 100);
  h.write("0000000\0", 108);
  h.write("0000000\0", 116);
  h.write(data.length.toString(8).padStart(11, "0") + "\0", 124);
  h.write("00000000000\0", 136);
  h.write("        ", 148);
  h.write("0", 156);
  h.write("ustar\0" + "00", 257);
  let sum = 0;
  for (const b of h) sum += b;
  h.write(sum.toString(8).padStart(6, "0") + "\0 ", 148);
  const pad = Buffer.alloc((512 - (data.length % 512)) % 512);
  return zlib.gzipSync(Buffer.concat([h, data, pad, Buffer.alloc(1024)]));
}

export function makeZip(name, data) {
  const comp = zlib.deflateRawSync(data);
  const n = Buffer.from(name);
  const lh = Buffer.alloc(30);
  lh.writeUInt32LE(0x04034b50, 0); lh.writeUInt16LE(20, 4); lh.writeUInt16LE(8, 8);
  lh.writeUInt32LE(comp.length, 18); lh.writeUInt32LE(data.length, 22); lh.writeUInt16LE(n.length, 26);
  const cd = Buffer.alloc(46);
  cd.writeUInt32LE(0x02014b50, 0); cd.writeUInt16LE(20, 4); cd.writeUInt16LE(20, 6); cd.writeUInt16LE(8, 10);
  cd.writeUInt32LE(comp.length, 20); cd.writeUInt32LE(data.length, 24); cd.writeUInt16LE(n.length, 28); cd.writeUInt32LE(0, 42);
  const local = Buffer.concat([lh, n, comp]);
  const central = Buffer.concat([cd, n]);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0); end.writeUInt16LE(1, 8); end.writeUInt16LE(1, 10);
  end.writeUInt32LE(central.length, 12); end.writeUInt32LE(local.length, 16);
  return Buffer.concat([local, central, end]);
}
