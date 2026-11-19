import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { PLATFORMS, build, normalizeVersion, unzipFile, untarFile } from "../../scripts/npm-pack.mjs";
import { makeTarGz, makeZip, tmp } from "./helpers.mjs";

test("normalizeVersion accepts tags and rejects junk", () => {
  assert.equal(normalizeVersion("v1.2.3"), "1.2.3");
  assert.equal(normalizeVersion("1.2.3-rc.1"), "1.2.3-rc.1");
  for (const bad of ["", "latest", "1.2", "v1.2.3;rm", "../1.2.3"]) assert.throws(() => normalizeVersion(bad));
});

test("archive readers extract the binary", () => {
  const data = Buffer.from("binary\0data".repeat(100));
  assert.deepEqual(untarFile(makeTarGz("skigard", data), "skigard"), data);
  assert.deepEqual(unzipFile(makeZip("skigard.exe", data), "skigard.exe"), data);
  assert.throws(() => untarFile(makeTarGz("other", data), "skigard"));
});

test("build produces six platform packages and a main package pinned to the tag version", () => {
  const dist = tmp();
  const out = path.join(tmp(), "out");
  for (const p of PLATFORMS) {
    const win = p.os === "win32";
    const data = Buffer.from(`fake ${p.goos}/${p.goarch}`);
    const file = `skigard_9.8.7_${p.goos}_${p.goarch}.${win ? "zip" : "tar.gz"}`;
    fs.writeFileSync(path.join(dist, file), win ? makeZip("skigard.exe", data) : makeTarGz("skigard", data));
  }
  build({ dist, version: "v9.8.7", out });
  const main = JSON.parse(fs.readFileSync(path.join(out, "skigard", "package.json"), "utf8"));
  assert.equal(main.version, "9.8.7");
  assert.equal(main.scripts, undefined);
  assert.equal(main.postinstall, undefined);
  assert.deepEqual(Object.keys(main.optionalDependencies).sort(), PLATFORMS.map((p) => `@ambierhq/skigard-${p.os}-${p.cpu}`).sort());
  for (const v of Object.values(main.optionalDependencies)) assert.equal(v, "9.8.7");
  for (const p of PLATFORMS) {
    const dir = path.join(out, "platform", `${p.os}-${p.cpu}`);
    const pj = JSON.parse(fs.readFileSync(path.join(dir, "package.json"), "utf8"));
    assert.equal(pj.version, "9.8.7");
    assert.deepEqual(pj.os, [p.os]);
    assert.deepEqual(pj.cpu, [p.cpu]);
    assert.equal(pj.scripts, undefined);
    const bin = path.join(dir, "bin", p.os === "win32" ? "skigard.exe" : "skigard");
    assert.equal(fs.readFileSync(bin, "utf8"), `fake ${p.goos}/${p.goarch}`);
  }
});

test("build fails when an archive is missing", () => {
  assert.throws(() => build({ dist: tmp(), version: "1.0.0", out: path.join(tmp(), "o") }), /missing/);
});
