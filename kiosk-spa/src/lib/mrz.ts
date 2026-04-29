/*
 * MRZ helpers — DOM-free where possible.
 *
 * `tesseract.js` and `mrz` are both lazy-imported from MrzScanner.tsx to keep
 * them out of the entry chunk. The shape `MrzFields` is the contract the
 * scanner emits; `mapMrzToGuest` is what Step1GuestPage feeds into
 * `patchGuest(idx, ...)` from `src/store/form.ts`.
 */

import type { GuestData } from '@/api/types';

export interface MrzFields {
  fname?: string;
  lname?: string;
  /** ISO YYYY-MM-DD */
  dob?: string;
  /** 3-letter ISO country code (MRZ native) */
  nationality?: string;
  /** 2-letter ISO country code derived from MRZ nationality (best-effort) */
  country?: string;
  document_id?: string;
  /** rare — most MRZs only carry expiry, not issue */
  document_issue_date?: string;
  /** legacy `document` enum: 1 = passport, 2 = ID card */
  document?: 1 | 2;
}

/**
 * MRZ encodes year as YY. Pivot at 30: 00–29 → 2000s, 30–99 → 1900s.
 * Good for guests born 1930–2029.
 */
export function isoFromMrzDate(yymmdd: string | undefined | null): string {
  if (!yymmdd || yymmdd.length !== 6 || !/^[0-9]{6}$/.test(yymmdd)) return '';
  const yy = parseInt(yymmdd.slice(0, 2), 10);
  const mm = yymmdd.slice(2, 4);
  const dd = yymmdd.slice(4, 6);
  const yyyy = yy < 30 ? 2000 + yy : 1900 + yy;
  // Sanity guard for impossible months/days — skip rather than emit garbage.
  const m = parseInt(mm, 10);
  const d = parseInt(dd, 10);
  if (m < 1 || m > 12 || d < 1 || d > 31) return '';
  return `${yyyy}-${mm}-${dd}`;
}

/**
 * 3-letter MRZ nationality codes that we map to the 2-letter ISO 3166-1
 * alpha-2 codes the legacy form expects in `country`. Covers the common
 * arrivals at the Caorle property; everything else stays empty so the user
 * can pick from the manual select.
 */
const ALPHA3_TO_ALPHA2: Record<string, string> = {
  ITA: 'IT',
  DEU: 'DE',
  D: 'DE', // German passports historically use single-letter "D"
  AUT: 'AT',
  CHE: 'CH',
  FRA: 'FR',
  ESP: 'ES',
  GBR: 'GB',
  NLD: 'NL',
  BEL: 'BE',
  POL: 'PL',
  CZE: 'CZ',
  SVK: 'SK',
  HUN: 'HU',
  HRV: 'HR',
  SVN: 'SI',
  USA: 'US',
  CAN: 'CA',
  PRT: 'PT',
  ROU: 'RO',
  BGR: 'BG',
};

export function alpha2FromMrzNationality(code: string | undefined | null): string {
  if (!code) return '';
  const k = code.toUpperCase().replace(/<+$/, '');
  return ALPHA3_TO_ALPHA2[k] ?? '';
}

/**
 * Crops the MRZ band — bottom ~25% of the document — out of the live video
 * frame and returns ImageData ready for tesseract. Width is upscaled to
 * 1280 px to give the OCR enough resolution on phone-class cameras.
 */
export function cropMrzImageData(
  video: HTMLVideoElement,
  canvas: HTMLCanvasElement,
): ImageData | null {
  const vw = video.videoWidth;
  const vh = video.videoHeight;
  if (vw === 0 || vh === 0) return null;

  // Crop the visible MRZ guide rectangle: 88% width, 22% height,
  // anchored at 70% from the top (matching the SVG overlay in MrzScanner).
  const cropW = Math.round(vw * 0.88);
  const cropH = Math.round(vh * 0.22);
  const cropX = Math.round((vw - cropW) / 2);
  const cropY = Math.round(vh * 0.70);

  // Upscale to 1280-wide for OCR — narrower frames produce mush.
  const targetW = Math.max(1280, cropW);
  const targetH = Math.round((cropH / cropW) * targetW);
  canvas.width = targetW;
  canvas.height = targetH;
  const ctx = canvas.getContext('2d');
  if (!ctx) return null;
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = 'high';
  ctx.drawImage(video, cropX, cropY, cropW, cropH, 0, 0, targetW, targetH);
  return ctx.getImageData(0, 0, targetW, targetH);
}

/**
 * Cleans a raw OCR string into MRZ candidate lines.
 *  - keeps only [A-Z0-9<]
 *  - removes lines shorter than 28 chars (TD2/TD3 are 36/44; allow some slack)
 *  - returns the last 2 or 3 lines (TD1 = 3, TD2/TD3 = 2)
 */
export function extractMrzLines(raw: string): string[] {
  const lines = raw
    .split(/\r?\n/)
    .map((l) => l.replace(/[^A-Z0-9<]/gi, '').toUpperCase())
    .filter((l) => l.length >= 28);
  if (lines.length >= 3) {
    const last3 = lines.slice(-3);
    // TD1 lines are all 30 chars. If the last three look balanced, return all 3.
    if (last3.every((l) => Math.abs(l.length - last3[0].length) <= 2)) return last3;
  }
  return lines.slice(-2);
}

/**
 * Map the parsed MRZ into the partial `GuestData` shape that
 * `patchGuest(idx, partial)` accepts. Only fills fields we are confident in;
 * the rest stay untouched so the user keeps any manually-typed values.
 */
export function mapMrzToGuest(f: MrzFields): Partial<GuestData> {
  const out: Partial<GuestData> = {};
  if (f.fname) out.fname = capitaliseName(f.fname);
  if (f.lname) out.lname = capitaliseName(f.lname);
  if (f.dob) out.dob = f.dob;
  if (f.nationality) out.nationality = f.nationality;
  if (f.country) out.country = f.country;
  if (f.document_id) out.document_id = f.document_id;
  if (f.document_issue_date) out.document_issue_date = f.document_issue_date;
  if (typeof f.document === 'number') out.document = f.document;
  return out;
}

function capitaliseName(s: string): string {
  return s
    .toLowerCase()
    .split(/\s+/)
    .map((p) => (p ? p[0].toUpperCase() + p.slice(1) : p))
    .join(' ')
    .trim();
}
