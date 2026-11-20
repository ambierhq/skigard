import harnesses from "../data/harnesses.json"; // generated from internal/harness/registry.toml by scripts/sync-docs.mjs

export const harnessCount = harnesses.length;

export function breadcrumbs(items: [string, string][]) {
  return {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: items.map(([name, item], i) => ({ "@type": "ListItem", position: i + 1, name, item })),
  };
}

export const software = {
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "skigard",
  applicationCategory: "DeveloperApplication",
  operatingSystem: "Windows, macOS, Linux",
  description: "A safer way to manage agent skills. skigard scans and scores each agent skill from 1 to 10, pins the exact bytes in a lockfile and installs in one command; strict mode asks a human first.",
  url: "https://ambierhq.com/skigard/",
  downloadUrl: "https://github.com/ambierhq/skigard/releases",
  codeRepository: "https://github.com/ambierhq/skigard",
  license: "https://opensource.org/license/mit",
  programmingLanguage: "Go",
  offers: { "@type": "Offer", price: "0", priceCurrency: "EUR" },
  author: { "@type": "Organization", name: "Ambier Oy", url: "https://ambierhq.com/", address: { "@type": "PostalAddress", addressCountry: "FI" } },
};

export const faq: { q: string; a: string }[] = [
  { q: "Can an agent approve a skill?", a: "Not through any flag or pipe. approve reads the controlling terminal (/dev/tty on Unix, the console on Windows), has no --yes flag, no stdin or environment path, and refuses inside the agent harnesses it can detect; add --force-install and changing mode or threshold are refused there too. A process that creates its own pseudo-terminal can still type the answer, so the gate stops accidents and ordinary prompt injection, not a determined agent. In auto mode an agent can still run add and install a skill that clears the threshold you chose; the lock records it as mode = auto, and verify --require-human flags it. Use strict mode to always require a human." },
  { q: "Does skigard run anything from a skill?", a: "No. It never executes skill content and never invokes git on skill content (GitHub sources are downloaded as archives; the only git call is `init --git`, which runs git init in your own home). Skills are fetched, copied with hostile-input handling, hashed and scanned as data." },
  { q: "Does skigard send telemetry?", a: "No. The binary has no telemetry and one runtime dependency, a TOML parser. This website runs no analytics today." },
  { q: "How does skigard decide whether to install a skill?", a: "add fetches the skill to quarantine, scans it and gives it a safety score from 1 to 10 with reasons. In auto mode (the default) a skill at or above the threshold (default 7) with no hard stop is installed; anything else stays in quarantine with the reasons and the commands to continue. In strict mode a human always confirms in a real terminal. A scanner block or a failing review is a hard stop that no threshold installs; only a human with --force-install or approve --override can. --quarantine-only stops after the scan." },
  { q: "Is a passing scan or a high score a safety certificate?", a: "No. The scanner is heuristic and an agent review can be fooled. The score is a recommendation, and the review can only raise concern, never lower a scanner finding. Read the skill, especially bundled scripts, or use strict mode." },
  { q: "Which agents does it support?", a: `${harnessCount} agent harnesses are in the registry, including Claude Code, Pi, Codex, Cursor, Gemini CLI and OpenCode. Entries marked verified were checked against the harness itself; others need --include-unverified.` },
  { q: "What does the lockfile pin?", a: "skigard.lock records the source, commit, a versioned content hash of the approved bytes, the scan result, the review, the safety score and how it was approved (human, auto or forced). skigard verify fails when the store, the installs or the generated marketplace drift." },
  { q: "Does it work on Windows?", a: "Yes, including approve, which reads the console. Windows is covered by CI and has not been tested by hand on a real machine yet; the binary is not code-signed, so SmartScreen may warn. Automated agent review is disabled on Windows; use review --manual." },
  { q: "What is the licence?", a: "MIT. skigard is a project by Ambier Oy, Finland." },
];
