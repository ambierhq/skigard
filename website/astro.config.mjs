import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

const SITE = "https://ambierhq.com";
const BASE = "/skigard";
// Cloudflare Web Analytics is off until the owner sets the token (see docs/ops-log.md). No token, no beacon.
const cfToken = process.env.CF_WEB_ANALYTICS_TOKEN;

export default defineConfig({
  site: SITE,
  base: BASE,
  trailingSlash: "always",
  build: { format: "directory" },
  integrations: [
    starlight({
      title: "skigard",
      description: "A lockfile and review gate for agent skills. A human approves the exact bytes before anything reaches your agents.",
      logo: { src: "./src/assets/mark.svg", alt: "skigard" },
      favicon: "/favicon.svg",
      social: [{ icon: "github", label: "GitHub", href: "https://github.com/ambierhq/skigard" }],
      editLink: { baseUrl: "https://github.com/ambierhq/skigard/edit/main/website/" },
      customCss: ["./src/styles/fonts.css", "./src/styles/ambier-tokens.css", "./src/styles/skigard.css", "./src/styles/starlight.css"],
      components: {
        Head: "./src/components/Head.astro",
        Footer: "./src/components/Footer.astro",
      },
      head: [
        { tag: "meta", attrs: { property: "og:site_name", content: "skigard by Ambier" } },
        { tag: "meta", attrs: { property: "og:image", content: `${SITE}${BASE}/og.png` } },
        { tag: "meta", attrs: { property: "og:image:alt", content: "skigard: a lockfile and review gate for agent skills" } },
        { tag: "meta", attrs: { name: "twitter:card", content: "summary_large_image" } },
        { tag: "meta", attrs: { name: "twitter:image", content: `${SITE}${BASE}/og.png` } },
        { tag: "meta", attrs: { name: "theme-color", content: "#16130E" } },
        ...(cfToken
          ? [{ tag: "script", attrs: { defer: true, src: "https://static.cloudflareinsights.com/beacon.min.js", "data-cf-beacon": JSON.stringify({ token: cfToken }) } }]
          : []),
      ],
      sidebar: [
        { label: "Start", items: [
          { label: "Install", slug: "docs/install" },
          { label: "Getting started", slug: "docs/getting-started" },
          { label: "Concepts", slug: "docs/concepts" },
          { label: "Using skigard in CI", slug: "docs/ci" },
        ] },
        { label: "Reference", items: [
          { label: "Threat model", slug: "docs/threat-model" },
          { label: "Agent review", slug: "docs/review" },
          { label: "Lockfile format", slug: "docs/lockfile" },
          { label: "Windows notes", slug: "docs/windows" },
          { label: "Supported harnesses", slug: "docs/harnesses" },
          { label: "Scanner rules", slug: "docs/rules" },
        ] },
      ],
      lastUpdated: false,
      pagination: true,
      disable404Route: true,
    }),
  ],
});
