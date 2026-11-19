# Ops log

Infrastructure actions taken by agents for ambierhq.com. No secrets. Newest last.

## 2026-11-19

- `cf dns records export --zone ambierhq.com` (read only). Backup stored outside this repo. 42 lines; nothing changed in DNS.
- `cf workers get website` and `cf builds list --worker c8773ebca32c46e9b5fe382b63528ea5` (read only). `website` is bound as a
  Custom Domain on `ambierhq.com` (domain binding id `2fdab17509f0f24d9209a9713037b763ce3bd0f4`) and is deployed by Workers Builds
  (trigger: branch `main`, deploy command `npx wrangler deploy`, root `/`).
- `ambierhq/website` PR #1 merged (squash). Workers Builds deployed it. Verified: `https://ambierhq.com/` 200,
  `/open-source/` 200, `/robots.txt` 200, `/sitemap-index.xml` 200, `/wrangler.jsonc` 404, `/README.md` 404.
- `wrangler login` done by the owner. DNS exported again before changes (`~/repos/docs/cloudflare-backups/2026-11-19-before-route-test.txt`, identical to the first export).
- **Route precedence test passed.** Throwaway Worker `ambier-route-test` with route `ambierhq.com/__route-test/*`: that path returned the test Worker (200) while `/` and `/open-source/` kept returning the main `website` Worker. A path route on another Worker beats the main Worker's Custom Domain. Deleted `ambier-route-test` afterwards (`wrangler delete --force`); the route went with it (404 again), and only the `website` Worker remains.
- Deployed Worker `ambier-skigard` (`wrangler deploy` from `website/`), routes `ambierhq.com/skigard` and `ambierhq.com/skigard/*`. Verified: `/skigard/` 200, `/skigard/docs/getting-started/` 200, `/skigard/llms.txt` 200, `/skigard/sitemap-index.xml` 200, `/skigard/install.sh` 301 to the GitHub release asset, unknown path 404, `/` and `/open-source/` still 200. `/skigard` without a slash first returned 404 (the `/*` route does not match it); added the exact route and a `_redirects` rule, now 301 to `/skigard/`. No DNS record was created or changed.
