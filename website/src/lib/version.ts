import { readFileSync } from "node:fs";
import { resolve } from "node:path";

// Latest released version, read from CHANGELOG.md at build time so the site cannot go stale.
const FALLBACK_VERSION = "0.4.0";
export function latestVersion(): string {
  try {
    const log = readFileSync(resolve(process.cwd(), "../CHANGELOG.md"), "utf8");
    return log.match(/^## \[(\d+\.\d+\.\d+)\]/m)?.[1] ?? FALLBACK_VERSION;
  } catch {
    return FALLBACK_VERSION;
  }
}
