/*
 * Theme manifest. Each property is a single object that drives:
 *   - palette CSS variables consumed by index.css
 *   - typography (display + body fonts; self-hosted)
 *   - hero photography path (under /public/themes/<id>/...)
 *   - logotype source (PNG/SVG inside public/themes/<id>/logo.svg)
 *   - copy bits that change per brand (property name, sub-label)
 *
 * Adding a new property = drop a file in src/theme/themes/<id>.ts and a
 * matching folder under public/themes/<id>/. No code changes needed.
 */
export interface ThemePalette {
  /** Page background. */
  bg: string;
  /** Surface used for cards, inputs. */
  surface: string;
  /** Subtle surface for muted blocks. */
  surfaceMuted: string;
  /** Default ink. */
  ink: string;
  /** Secondary ink (placeholders, captions). */
  inkMuted: string;
  /** Hairline border / divider. */
  border: string;
  /** Brand mid-tone — used for focus rings, secondary chrome. */
  brand: string;
  /** Brand soft tint for highlight backgrounds. */
  brandSoft: string;
  /** Primary CTA fill — the "go" colour. */
  accent: string;
  /** Primary CTA hover/pressed. */
  accentStrong: string;
  /** Ink colour to use ON the accent (contrast-safe). */
  accentInk: string;
  /** Hero overlay base (dark side of the gradient). */
  heroOverlayFrom: string;
  /** Hero overlay base (light side of the gradient). */
  heroOverlayTo: string;
}

export interface ThemeFonts {
  /** Display font (headings) — Google name or self-hosted family. */
  display: string;
  /** Body sans. */
  body: string;
}

export interface ThemeManifest {
  id: string;
  /** Property name shown in the wordmark/header. */
  propertyName: string;
  /** Optional small caption next to the wordmark (e.g. "smart moov hotel"). */
  propertySubtitle?: string;
  /** True when the brand uses a serif display + warm palette (Pareus).
   *  False for modern sans-only brands (smart moov). */
  serifDisplay: boolean;
  palette: ThemePalette;
  fonts: ThemeFonts;
  /** Hero asset under /public/themes/<id>/<heroImage>. */
  heroImage: string;
  /** Logotype (SVG preferred) under /public/themes/<id>/<logo>. */
  logo: string;
}
