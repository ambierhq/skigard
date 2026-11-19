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
  description: "A lockfile and review gate for agent skills. A human approves the exact bytes before anything reaches your agents.",
  url: "https://ambierhq.com/skigard/",
  downloadUrl: "https://github.com/ambierhq/skigard/releases",
  codeRepository: "https://github.com/ambierhq/skigard",
  license: "https://opensource.org/license/mit",
  programmingLanguage: "Go",
  offers: { "@type": "Offer", price: "0", priceCurrency: "EUR" },
  author: { "@type": "Organization", name: "Ambier Oy", url: "https://ambierhq.com/", address: { "@type": "PostalAddress", addressCountry: "FI" } },
};

export const faq: { q: string; a: string }[] = [
  { q: "Can an agent approve a skill?", a: "No. approve reads the controlling terminal (/dev/tty on Unix, the console on Windows), has no --yes flag, no stdin path and no environment bypass, and refuses to run inside a detected agent harness." },
  { q: "Does skigard run anything from a skill?", a: "No. It never executes skill content and never invokes git. Skills are fetched, copied with hostile-input handling, hashed and scanned as data." },
  { q: "Does skigard send telemetry?", a: "No. The binary has no telemetry and one runtime dependency, a TOML parser. This website uses privacy-friendly analytics only when enabled." },
  { q: "Is a passing scan a safety certificate?", a: "No. The scanner is heuristic and an agent review can be fooled. The review can only raise concern, never lower a scanner finding. Read the skill, especially bundled scripts." },
  { q: "Which agents does it support?", a: "About 75 agent harnesses are detected, including Claude Code, Pi, Codex, Cursor, Gemini CLI and OpenCode. Entries marked verified were checked against the harness itself; others need --include-unverified." },
  { q: "What does the lockfile pin?", a: "skigard.lock records the source, commit, a versioned content hash of the approved bytes, the scan result, the review and the approver. skigard verify fails when the store, the installs or the generated marketplace drift." },
  { q: "Does it work on Windows?", a: "Yes, including approve, which reads the console. Automated agent review is disabled on Windows; use review --manual." },
  { q: "What is the licence?", a: "MIT. skigard is a project by Ambier Oy, Finland." },
];
