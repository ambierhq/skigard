# Ops log

Infrastructure actions taken by agents for ambierhq.com. No secrets. Newest last.

## 2026-11-19

- `cf dns records export --zone ambierhq.com` (read only). Backup stored outside this repo. 42 lines; nothing changed in DNS.
- `cf workers get website` and `cf builds list --worker c8773ebca32c46e9b5fe382b63528ea5` (read only). `website` is bound as a
  Custom Domain on `ambierhq.com` (domain binding id `2fdab17509f0f24d9209a9713037b763ce3bd0f4`) and is deployed by Workers Builds
  (trigger: branch `main`, deploy command `npx wrangler deploy`, root `/`).
- `ambierhq/website` PR #1 merged (squash). Workers Builds deployed it. Verified: `https://ambierhq.com/` 200,
  `/open-source/` 200, `/robots.txt` 200, `/sitemap-index.xml` 200, `/wrangler.jsonc` 404, `/README.md` 404.
- **Route precedence test (`ambier-route-test`) and the real `ambier-skigard` deploy were NOT run.** `wrangler whoami` reports
  "not authenticated" and the owner's `cf` OAuth login does not cover `wrangler`. Needs an API token, see the PR description.
  Until then `https://ambierhq.com/skigard/` returns 404 from the main Worker.
