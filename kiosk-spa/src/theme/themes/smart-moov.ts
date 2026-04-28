import type { ThemeManifest } from '../types';

// smart moov hotel — modern, urban, bold red on charcoal/white architecture.
// Reference: contemporary city hotel facade, dark grey/anthracite walls, white
// soffit/header, single bright red wave mark. We translate that into a calm
// kiosk skin: light surfaces, near-black ink, red used ONLY for the primary
// CTA + active accents, never as background.
export const smartMoovTheme: ThemeManifest = {
  id: 'smart-moov',
  propertyName: 'smart moov',
  propertySubtitle: 'hotel',
  serifDisplay: false,
  fonts: {
    display: '"Geist", "Inter Tight", system-ui, sans-serif',
    body: '"Geist", "Inter Tight", system-ui, sans-serif',
  },
  palette: {
    bg: 'oklch(0.985 0 0)',          // off-white
    surface: '#ffffff',
    surfaceMuted: 'oklch(0.96 0.003 280)',
    ink: 'oklch(0.18 0.01 280)',     // near-black charcoal
    inkMuted: 'oklch(0.55 0.01 280)',
    border: 'oklch(0.9 0.005 280)',
    brand: 'oklch(0.32 0.02 280)',    // dark anthracite (architecture base)
    brandSoft: 'oklch(0.95 0.005 280)',
    accent: 'oklch(0.62 0.22 25)',    // signature red
    accentStrong: 'oklch(0.55 0.24 25)',
    accentInk: '#ffffff',
    heroOverlayFrom: 'oklch(0.18 0.01 280 / 0.78)',
    heroOverlayTo: 'oklch(0.62 0.22 25 / 0.35)',
  },
  heroImage: '/themes/smart-moov/hero.jpg',
  logo: '/themes/smart-moov/logo.svg',
};
