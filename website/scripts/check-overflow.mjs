// Fails when a page scrolls horizontally at common widths. Serves the built site (deploy/) on a local port and
// drives the system Chrome/Chromium/Brave given by CHROME_PATH (puppeteer-core downloads no browser).
// Usage: CHROME_PATH=/path/to/chrome node scripts/check-overflow.mjs [--screenshots DIR]
import http from "node:http";
import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import puppeteer from "puppeteer-core";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../deploy");
const widths = [320, 390, 768, 1024, 1280];
// Landing hero: the install box must not overlap the terminal panel at any width.
const sweep = Array.from({ length: 24 }, (_, i) => 320 + i * 50).concat([1440]);
const pages = [
  "/skigard/",
  "/skigard/docs/getting-started/",
  "/skigard/docs/install/",
  "/skigard/guides/review-agent-skills-before-installing/",
];
const shotDir = process.argv.includes("--screenshots") ? process.argv[process.argv.indexOf("--screenshots") + 1] : "";
const chrome = process.env.CHROME_PATH;
if (!chrome) {
  console.error("set CHROME_PATH to a Chrome, Chromium or Brave binary");
  process.exit(2);
}

const types = { ".html": "text/html", ".css": "text/css", ".js": "text/javascript", ".svg": "image/svg+xml", ".woff2": "font/woff2", ".png": "image/png", ".json": "application/json" };
const server = http.createServer(async (req, res) => {
  let p = path.join(root, decodeURIComponent(new URL(req.url, "http://x").pathname));
  if (!p.startsWith(root)) { res.writeHead(403).end(); return; }
  try {
    if ((await fs.stat(p)).isDirectory()) p = path.join(p, "index.html");
    res.writeHead(200, { "content-type": types[path.extname(p)] ?? "application/octet-stream" });
    res.end(await fs.readFile(p));
  } catch {
    res.writeHead(404).end("not found");
  }
});
await new Promise((r) => server.listen(0, "127.0.0.1", r));
const origin = `http://127.0.0.1:${server.address().port}`;

const browser = await puppeteer.launch({ executablePath: chrome, headless: true, args: ["--no-sandbox"] });
let bad = 0;
try {
  for (const url of pages) {
    for (const w of widths) {
      const page = await browser.newPage();
      await page.setViewport({ width: w, height: 900, deviceScaleFactor: 1 });
      await page.goto(origin + url, { waitUntil: "networkidle0" });
      const r = await page.evaluate(() => {
        const de = document.documentElement;
        const iw = innerWidth;
        const offenders = [...document.querySelectorAll("body *")]
          .filter((e) => e.getBoundingClientRect().right > iw + 1 && !e.closest("pre, table, .sr, [aria-hidden=true]"))
          .slice(0, 5).map((e) => `${e.tagName.toLowerCase()}.${e.className}`);
        return { sw: de.scrollWidth, iw, offenders };
      });
      const ok = r.sw <= r.iw;
      if (!ok) bad++;
      console.log(`${ok ? "ok  " : "FAIL"} ${String(w).padStart(4)}px ${url} scrollWidth=${r.sw}${ok ? "" : " offenders: " + r.offenders.join(", ")}`);
      if (shotDir && url === "/skigard/") {
        await fs.mkdir(shotDir, { recursive: true });
        await page.screenshot({ path: path.join(shotDir, `landing-${w}.png`), fullPage: false });
      }
      await page.close();
    }
  }
  const page = await browser.newPage();
  for (const w of sweep) {
    await page.setViewport({ width: w, height: 900 });
    await page.goto(`${origin}/skigard/`, { waitUntil: "networkidle0" });
    const hit = await page.evaluate(() => {
      const box = (s) => [...document.querySelectorAll(s)].find((e) => e.offsetParent)?.getBoundingClientRect();
      const t = box(".term");
      const parts = [".tabs", ".panel", ".copy"].map((s) => [s, box(s)]);
      return parts.filter(([, r]) => r && t && r.left < t.right && r.right > t.left && r.top < t.bottom && r.bottom > t.top).map(([s]) => s);
    });
    if (hit.length) {
      bad++;
      console.log(`FAIL ${String(w).padStart(4)}px hero: ${hit.join(", ")} overlaps the terminal panel`);
    }
    if (shotDir && (w === 1020 || w === 1070)) await page.screenshot({ path: path.join(shotDir, `landing-${w}.png`) });
  }
  console.log(`hero overlap sweep done (${sweep.length} widths)`);
} finally {
  await browser.close();
  server.close();
}
if (bad) {
  console.error(`${bad} overflow failure(s)`);
  process.exit(1);
}
