# Deploying the site

The site is a Cloudflare Workers static-assets deployment (`wrangler.jsonc`) served at `https://ambierhq.com/skigard/`.

## Manual deploy

```sh
cd website
npm ci --ignore-scripts
npm run build
WRANGLER_SEND_METRICS=false npx --no-install wrangler@4.164.0 deploy
```

`wrangler` is pinned in `package.json` (4.164.0); keep the version above in step with it. You need `CLOUDFLARE_API_TOKEN`
and `CLOUDFLARE_ACCOUNT_ID` in the environment (or `wrangler login`).

## CI deploy

The `deploy` job in `.github/workflows/site.yml` runs on pushes to `main` only and does nothing unless the repository
secret `CLOUDFLARE_API_TOKEN` exists. Without it, merges to `main` do not update the live site.

Repository secrets:

- `CLOUDFLARE_API_TOKEN`: an API token with these permissions
  - Account: Workers Scripts: Edit (and Read)
  - Zone: Workers Routes: Edit, limited to the zone `ambierhq.com`
- `CLOUDFLARE_ACCOUNT_ID`: the Cloudflare account id.

## Checks before deploying

`npm run check:links` (internal links) and `CHROME_PATH=/path/to/chrome npm run check:overflow` (no horizontal scroll at
320 to 1280 px) run in CI on every pull request.
