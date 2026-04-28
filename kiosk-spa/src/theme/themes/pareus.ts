import type { ThemeManifest } from '../types';

// Pareus Beach Resort — refined-coastal hospitality.
export const pareusTheme: ThemeManifest = {
  id: 'pareus',
  propertyName: 'Pareus',
  propertySubtitle: 'Beach Resort',
  serifDisplay: true,
  fonts: {
    display: '"Fraunces", "Cormorant Garamond", "Times New Roman", serif',
    body: '"Geist", "Inter Tight", system-ui, sans-serif',
  },
  palette: {
    bg: 'oklch(0.985 0.01 80)',
    surface: '#ffffff',
    surfaceMuted: 'oklch(0.965 0.014 78)',
    ink: 'oklch(0.22 0.03 60)',
    inkMuted: 'oklch(0.45 0.04 65)',
    border: 'oklch(0.935 0.022 76)',
    brand: 'oklch(0.37 0.12 232)',
    brandSoft: 'oklch(0.92 0.05 232)',
    accent: 'oklch(0.78 0.16 72)',
    accentStrong: 'oklch(0.86 0.13 78)',
    accentInk: 'oklch(0.22 0.03 60)',
    heroOverlayFrom: 'oklch(0.22 0.08 232 / 0.85)',
    heroOverlayTo: 'oklch(0.22 0.03 60 / 0.4)',
  },
  heroImage: '/themes/pareus/hero.jpg',
  logo: '/themes/pareus/logo.svg',
};
