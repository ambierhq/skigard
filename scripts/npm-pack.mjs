#!/usr/bin/env node
// Build the npm packages from GoReleaser archives (no network, no dependencies).
//   node scripts/npm-pack.mjs --dist dist --version 1.2.3 --out npm-dist [--pack]
// --version is the git tag without the leading "v" (a leading "v" is accepted and stripped).
// Output: <out>/skigard (main package) and <out>/platform/<os>-<cpu> (one package per platform).
// With --pack each package is also run through `npm pack` into <out>/tarballs.
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import zlib from "node:zlib";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const SCOPE = "@ambierhq";
// npm (os, cpu) -> GoReleaser (os, arch)
export const PLATFORMS = [
  { os: "linux", cpu: "x64", goos: "linux", goarch: "amd64" },
  { os: "linux", cpu: "arm64", goos: "linux", goarch: "arm64" },
  { os: "darwin", cpu: "x64", goos: "darwin", goarch: "amd64" },
  { os: "darwin", cpu: "arm64", goos: "darwin", goarch: "arm64" },
  { os: "win32", cpu: "x64", goos: "windows", goarch: "amd64" },
  { os: "win32", cpu: "arm64", goos: "windows", goarch: "arm64" },
];

// Extract one file (by base name, at the archive root) from a .tar.gz buffer.
export function untarFile(gz, name) {
  const buf = zlib.gunzipSync(gz);
  let off = 0;
  let longName = null;
  while (off + 512 <= buf.length) {
    const h = buf.subarray(off, off + 512);
    if (h.every((b) => b === 0)) break;
    const str = (a, n) => h.toString("utf8", a, a + n).replace(/\0.*$/s, "");
    let fname = longName ?? str(0, 100);
    const prefix = str(345, 155);
    if (longName === null && prefix) fname = `${prefix}/${fname}`;
    longName = null;
    const size = parseInt(str(124, 12).trim() || "0", 8);
    const type = String.fromCharCode(h[156] || 48);
    const data = buf.subarray(off + 512, off + 512 + size);
    off += 512 + Math.ceil(size / 512) * 512;
    if (type === "L") {
      longName = data.toString("utf8").replace(/\0.*$/s, "");
      continue;
    }
    if ((type === "0" || type === "\0") && path.posix.normalize(fname).replace(/^\.\//, "") === name) return Buffer.from(data);
  }
  throw new Error(`${name} not found in tar.gz`);
}

// Extract one file from a .zip buffer using the central directory.
export function unzipFile(zip, name) {
  let eocd = -1;
  for (let i = zip.length - 22; i >= Math.max(0, zip.length - 65557); i--) {
    if (zip.readUInt32LE(i) === 0x06054b50) { eocd = i; break; }
  }
  if (eocd < 0) throw new Error("not a zip file");
  const count = zip.readUInt16LE(eocd + 10);
  let p = zip.readUInt32LE(eocd + 16);
  for (let n = 0; n < count; n++) {
    if (zip.readUInt32LE(p) !== 0x02014b50) throw new Error("corrupt zip central directory");
    const method = zip.readUInt16LE(p + 10);
    const csize = zip.readUInt32LE(p + 20);
    const nlen = zip.readUInt16LE(p + 28);
    const xlen = zip.readUInt16LE(p + 30);
    const clen = zip.readUInt16LE(p + 32);
    const lho = zip.readUInt32LE(p + 42);
    const fname = zip.toString("utf8", p + 46, p + 46 + nlen);
    p += 46 + nlen + xlen + clen;
    if (fname !== name) continue;
    const dstart = lho + 30 + zip.readUInt16LE(lho + 26) + zip.readUInt16LE(lho + 28);
    const raw = zip.subarray(dstart, dstart + csize);
    if (method === 0) return Buffer.from(raw);
    if (method === 8) return zlib.inflateRawSync(raw);
    throw new Error(`unsupported zip method ${method}`);
  }
  throw new Error(`${name} not found in zip`);
}

export function normalizeVersion(v) {
  const s = String(v ?? "").replace(/^v/, "");
  if (!/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$/.test(s)) throw new Error(`invalid version ${JSON.stringify(v)}`);
  return s;
}

export function build({ dist, version, out, pack = false }) {
  version = normalizeVersion(version);
  fs.rmSync(out, { recursive: true, force: true });
  const license = fs.readFileSync(path.join(root, "LICENSE"));
  const main = JSON.parse(fs.readFileSync(path.join(root, "npm", "package.json"), "utf8"));
  const dirs = [];

  for (const p of PLATFORMS) {
    const win = p.os === "win32";
    const archive = path.join(dist, `skigard_${version}_${p.goos}_${p.goarch}.${win ? "zip" : "tar.gz"}`);
    if (!fs.existsSync(archive)) throw new Error(`missing ${archive}`);
    const exe = win ? "skigard.exe" : "skigard";
    const raw = fs.readFileSync(archive);
    const binary = win ? unzipFile(raw, exe) : untarFile(raw, exe);
    const dir = path.join(out, "platform", `${p.os}-${p.cpu}`);
    fs.mkdirSync(path.join(dir, "bin"), { recursive: true });
    fs.writeFileSync(path.join(dir, "bin", exe), binary, { mode: 0o755 });
    fs.writeFileSync(path.join(dir, "LICENSE"), license);
    fs.writeFileSync(path.join(dir, "README.md"), `# ${SCOPE}/skigard-${p.os}-${p.cpu}\n\nThe skigard ${p.os}/${p.cpu} binary. Install \`skigard\` instead: https://github.com/ambierhq/skigard\n`);
    fs.writeFileSync(
      path.join(dir, "package.json"),
      JSON.stringify(
        {
          name: `${SCOPE}/skigard-${p.os}-${p.cpu}`,
          version,
          description: `skigard binary for ${p.os}/${p.cpu}`,
          license: main.license,
          author: main.author,
          homepage: main.homepage,
          repository: { ...main.repository, directory: undefined },
          os: [p.os],
          cpu: [p.cpu],
          files: ["bin", "README.md", "LICENSE"],
          publishConfig: { access: "public" },
        },
        null,
        2,
      ) + "\n",
    );
    main.optionalDependencies[`${SCOPE}/skigard-${p.os}-${p.cpu}`] = version;
    dirs.push(dir);
  }

  main.version = version;
  const mdir = path.join(out, "skigard");
  fs.mkdirSync(path.join(mdir, "bin"), { recursive: true });
  fs.copyFileSync(path.join(root, "npm", "bin", "skigard.js"), path.join(mdir, "bin", "skigard.js"));
  fs.chmodSync(path.join(mdir, "bin", "skigard.js"), 0o755);
  fs.copyFileSync(path.join(root, "npm", "README.md"), path.join(mdir, "README.md"));
  fs.writeFileSync(path.join(mdir, "LICENSE"), license);
  delete main.scripts;
  fs.writeFileSync(path.join(mdir, "package.json"), JSON.stringify(main, null, 2) + "\n");
  dirs.push(mdir);

  if (pack) {
    const tb = path.join(out, "tarballs");
    fs.mkdirSync(tb, { recursive: true });
    for (const d of dirs) execFileSync("npm", ["pack", "--loglevel=warn", "--pack-destination", path.resolve(tb)], { cwd: d, stdio: ["ignore", "ignore", "inherit"], shell: process.platform === "win32" });
  }
  return { version, out, dirs };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const args = process.argv.slice(2);
  const opt = { dist: "dist", out: "npm-dist", pack: false };
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === "--pack") opt.pack = true;
    else if (["--dist", "--version", "--out"].includes(a)) opt[a.slice(2)] = args[++i];
    else { console.error(`unknown argument ${a}\nusage: npm-pack.mjs --dist DIR --version X.Y.Z [--out DIR] [--pack]`); process.exit(2); }
  }
  if (!opt.version) { console.error("--version is required"); process.exit(2); }
  try {
    const r = build(opt);
    console.log(`built ${r.dirs.length} packages at version ${r.version} in ${r.out}`);
  } catch (e) {
    console.error(`npm-pack: ${e.message}`);
    process.exit(1);
  }
}
