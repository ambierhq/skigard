# Pipeline diagram source

`pipeline-dark.svg` and `pipeline-light.svg` (used by the README, docs/concepts.md and the site) come from the
[Archify](https://github.com/tt-a1i/archify) workflow diagram in `pipeline.workflow.json`:

1. From a scratch folder (git-ignored `.archify/`): `node archify.mjs finalize workflow pipeline.workflow.json assets/diagrams/pipeline-archify.html --quality showcase`.
2. `export-svg.sh <html> svg-dark|svg-light <out.svg>` drives the viewer's own SVG export in headless Chromium/Brave.
3. `brand-export.py <in.svg> dark|light <out.svg>` applies the skigard palette (`assets/brand/tokens.env`), drops the legend and lane frames and crops to the README shape.

The SVGs embed JetBrains Mono (SIL OFL, license text kept inside) as data URIs, so they render the same inside an `<img>` without webfonts.
`pipeline-archify.html` is the interactive version in Archify's own palette. Mermaid sources of the older diagrams stay one level up.
