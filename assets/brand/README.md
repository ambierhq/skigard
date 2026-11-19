# skigard brand assets

> **The mark is provisional** until the owner approves it. Everything in the repository references it by path under
> `assets/brand/`, so swapping it means replacing the files here and nothing else.

## Files

| File | Use |
| --- | --- |
| `mark.svg` | the mark on dark backgrounds |
| `mark-on-light.svg` | the mark on light backgrounds (darker blue for contrast) |
| `mark-mono.svg` | one colour, follows CSS `color` (`currentColor`); stamps, print, dark-on-anything |
| `logo-full.svg`, `logo-full-on-light.svg` | mark and the wordmark "skigard" (README header, site) |
| `favicon.svg`, `favicon.ico`, `favicon-32.png`, `apple-touch-icon.png` (180), `icon-512.png` | small sizes; heavier strokes than the mark so it holds at 16 px |
| `social-preview.png` (1280x640) | GitHub social preview and `og:image` |
| `readme-banner.svg`, `readme-banner.png` | wide banner (1280x320) |

The SVGs are hand-written, contain no `<image>` and no `<text>`: the wordmark is the lowercase word set in Newsreader and
converted to outlines (the font is SIL OFL, which allows this). The rasters are rendered from the SVGs.

## Palette

The accent is sky blue (changed from amber, which the Ambier company site keeps). It lives in one place per file; change `#5AA9FF` / `#2B6CB0` in the SVGs and re-render the rasters to move it.

| Role | Hex |
| --- | --- |
| Background | `#16130E` |
| Foreground | `#F4EEE2` |
| Dim text | `#B7A98C` |
| Accent (sky blue, the "byte") | `#5AA9FF` |
| Accent on light backgrounds | `#2B6CB0` |

## Fonts

- **Newsreader** (serif) for display and the wordmark.
- **IBM Plex Sans** for UI and body text, **IBM Plex Mono** for code.

Both are SIL OFL. Self-host them; never load them from Google Fonts.

## Usage

- **Clear space:** keep at least the height of the accent square (about 1/6 of the mark height) free on every side.
- **Minimum size:** `mark*.svg` 24 px high, `logo-full*.svg` 24 px high. Below that use `favicon.svg`.
- Dark backgrounds get `mark.svg`/`logo-full.svg`; light ones get the `-on-light` files. In READMEs use
  `<picture>` with `prefers-color-scheme`.

Do:
- Keep the square the only accent-coloured element.
- Use the files as they are.

Don't:
- Recolour, rotate, outline, add shadows or gradients.
- Stretch, or redraw the wordmark in another typeface.
- Put the dark-background mark on a light background (the cream shield disappears).
