import { smartMoovTheme } from './themes/smart-moov';
import { pareusTheme } from './themes/pareus';
import type { ThemeManifest } from './types';

/*
 * Theme registry.
 *
 * Themes are bundled at build time (one SPA bundle, all properties). The
 * ACTIVE theme is selected at runtime by the property's `theme` field
 * returned from /api/kiosk/v1/<slug>/config. That way one SPA serves
 * many properties without rebuilds.
 */
const themes: Record<string, ThemeManifest> = {
  'smart-moov': smartMoovTheme,
  pareus: pareusTheme,
};

/** Falls back to smart-moov so the kiosk still renders if the property
 *  config references an unknown theme (e.g. a future property added on
 *  the server before the SPA was redeployed with the matching manifest). */
export const resolveTheme = (id: string | undefined): ThemeManifest => {
  if (id && themes[id]) return themes[id];
  return smartMoovTheme;
};

/** Backwards compat: the welcome page used to read activeTheme directly.
 *  Components that load AFTER the property is fetched should use the
 *  ThemeProvider context instead — but a sensible default is kept here
 *  so older imports compile during the migration. */
export const activeTheme: ThemeManifest = smartMoovTheme;

/** Apply the theme's palette + fonts as CSS variables on :root so
 *  index.css can consume them via var(--kt-...). Idempotent. */
export const applyTheme = (t: ThemeManifest) => {
  const r = document.documentElement.style;
  const p = t.palette;
  r.setProperty('--kt-bg', p.bg);
  r.setProperty('--kt-surface', p.surface);
  r.setProperty('--kt-surface-muted', p.surfaceMuted);
  r.setProperty('--kt-ink', p.ink);
  r.setProperty('--kt-ink-muted', p.inkMuted);
  r.setProperty('--kt-border', p.border);
  r.setProperty('--kt-brand', p.brand);
  r.setProperty('--kt-brand-soft', p.brandSoft);
  r.setProperty('--kt-accent', p.accent);
  r.setProperty('--kt-accent-strong', p.accentStrong);
  r.setProperty('--kt-accent-ink', p.accentInk);
  r.setProperty('--kt-hero-from', p.heroOverlayFrom);
  r.setProperty('--kt-hero-to', p.heroOverlayTo);
  r.setProperty('--kt-font-display', t.fonts.display);
  r.setProperty('--kt-font-body', t.fonts.body);
  document.documentElement.dataset.theme = t.id;
};

export type { ThemeManifest } from './types';
