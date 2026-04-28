# smart moov hotel — kiosk theme assets

Drop the property's brand assets here. The kiosk loads them at runtime from
`/themes/smart-moov/...`, so a deploy can update them without touching code.

## Required

- **`hero.jpg`** — full-bleed welcome / done photography. Recommended:
  - 2880×1620 (or larger), JPEG, ≤ 600 KB after compression
  - Building exterior at golden hour, no people, no other branding
  - Self-hosted (do not link to a CDN — kiosks may sit on captive networks)

- **`logo.svg`** — the red wave wordmark. Scaled to fit the header at h-8.
  A working placeholder ships with this folder; replace it with the official
  brand mark before launch.

## Optional

- `favicon.png` — tab icon, 256×256.

Until `hero.jpg` is provided, the welcome screen falls back to the brand
gradient overlay alone — visually still on-brand but less hospitality-grade.
