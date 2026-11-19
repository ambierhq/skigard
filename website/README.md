# skigard website

Astro + Starlight, served at `https://ambierhq.com/skigard/` by the Cloudflare Worker `ambier-skigard`
(static assets, route `ambierhq.com/skigard/*`).

```sh
cd website
npm ci --ignore-scripts
npm run build        # sync docs, astro build, output to deploy/skigard/
npm run check:links
npm run lhci         # Lighthouse CI, all categories >= 0.95 (needs Chrome)
npx wrangler deploy  # needs CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID; CI does this on main
```

- Docs pages come from `../docs/*.md`, the harness grid from `../internal/harness/registry.toml`, the logo, favicons and social card from
  `../assets/brand/`. `scripts/sync-docs.mjs` runs before every build; generated files are gitignored. Never copy docs here.
- Colours and fonts live in `src/styles/ambier-tokens.css` (shared across Ambier sites). Fonts are self-hosted.
- Analytics is off. Set `CF_WEB_ANALYTICS_TOKEN` at build time to add the Cloudflare Web Analytics beacon.
- `/skigard/install.sh` and `/install.ps1` are 301s in `_redirects` to the latest GitHub release assets.
