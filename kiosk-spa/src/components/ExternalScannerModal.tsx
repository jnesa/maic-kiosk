import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { XClose } from '@untitledui/icons';
import { extractMrzLines, isoFromMrzDate, alpha2FromMrzNationality, type MrzFields } from '@/lib/mrz';

interface ExternalScannerModalProps {
  open: boolean;
  onClose: () => void;
  onParsed: (fields: MrzFields) => void;
}

type Stage =
  | { kind: 'placement' }       // waiting for the desk scanner to fire
  | { kind: 'reading' }         // burst received, parsing
  | { kind: 'writing'; fields: MrzFields }  // showing the pen-writing animation
  | { kind: 'failed' };

// Most keyboard-wedge scanners deliver an MRZ in <500ms. We commit the buffer
// once a 250ms quiet window passes after a burst.
const QUIET_MS = 250;
const MIN_BUFFER = 56; // shortest plausible MRZ payload (TD3 = 88, TD2 = 72)

/**
 * Modal for hotel kiosks that pair with a desk-mounted document scanner
 * (Cipherlab, Cherry CR-1300, Datalogic) emulating a USB keyboard. The
 * scanner "types" the MRZ at high speed — we listen on a hidden textarea,
 * detect the burst, parse, and fire onParsed.
 *
 * Two animated stages frame the wait:
 *   1. Placement — bobbing passport above an open scanner with a sweeping
 *      scan line. Stays until a complete MRZ comes in.
 *   2. Writing — a pen draws ink lines onto paper while we autofill the
 *      guest fields. Plays for ~1.8s, then closes.
 */
export const ExternalScannerModal = ({ open, onClose, onParsed }: ExternalScannerModalProps) => {
  const { t } = useTranslation();
  const inputRef = useRef<HTMLTextAreaElement | null>(null);
  const bufferRef = useRef('');
  const quietTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const writingTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [stage, setStage] = useState<Stage>({ kind: 'placement' });

  // Refocus the hidden input whenever the modal becomes visible / the
  // window regains focus. Keyboard scanners need an active focus target.
  useEffect(() => {
    if (!open) return;
    const focus = () => inputRef.current?.focus();
    focus();
    const onFocusIn = () => focus();
    window.addEventListener('focus', onFocusIn);
    document.addEventListener('visibilitychange', focus);
    return () => {
      window.removeEventListener('focus', onFocusIn);
      document.removeEventListener('visibilitychange', focus);
    };
  }, [open, stage.kind]);

  // Reset state when the modal opens or closes.
  useEffect(() => {
    if (open) {
      bufferRef.current = '';
      setStage({ kind: 'placement' });
      return;
    }
    if (quietTimer.current) clearTimeout(quietTimer.current);
    if (writingTimer.current) clearTimeout(writingTimer.current);
  }, [open]);

  const tryCommit = async () => {
    const raw = bufferRef.current;
    if (raw.length < MIN_BUFFER) return false;
    setStage({ kind: 'reading' });
    const lines = splitMrzCandidate(raw);
    if (lines.length < 2) {
      setStage({ kind: 'failed' });
      return false;
    }
    try {
      const { parse } = await import('mrz');
      const parsed = parse(lines);
      if (!parsed.valid) {
        setStage({ kind: 'failed' });
        return false;
      }
      const fields = toMrzFields(parsed);
      setStage({ kind: 'writing', fields });
      // Let the pen-on-paper animation breathe before we close — the user
      // should read the autofill happening, not feel the modal flicker shut.
      writingTimer.current = setTimeout(() => {
        onParsed(fields);
      }, 1800);
      return true;
    } catch {
      setStage({ kind: 'failed' });
      return false;
    }
  };

  const onInput = (ev: React.FormEvent<HTMLTextAreaElement>) => {
    if (stage.kind === 'writing' || stage.kind === 'reading') return;
    bufferRef.current = ev.currentTarget.value.toUpperCase();
    if (quietTimer.current) clearTimeout(quietTimer.current);
    quietTimer.current = setTimeout(() => {
      void tryCommit();
    }, QUIET_MS);
  };

  const onRetry = () => {
    bufferRef.current = '';
    if (inputRef.current) inputRef.current.value = '';
    setStage({ kind: 'placement' });
    inputRef.current?.focus();
  };

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-40 flex items-center justify-center bg-ink/85 px-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-label={t('external_scanner.title')}
    >
      <div className="kiosk-card relative w-full max-w-2xl overflow-hidden p-0">
        <header className="flex items-center justify-between gap-3 border-b border-border-subtle px-6 py-4">
          <h2 className="display text-xl font-semibold text-ink">
            {stage.kind === 'writing'
              ? t('external_scanner.title_writing')
              : t('external_scanner.title')}
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="rounded-full p-2 text-ink-muted transition-colors hover:bg-surface-muted"
            aria-label={t('scanner.cancel')}
          >
            <XClose className="h-5 w-5" />
          </button>
        </header>

        {/* Stage 1 + 2 share the same container so the swap is just a fade */}
        <div className="relative flex min-h-[360px] items-center justify-center bg-surface-muted px-8 py-10">
          {/* Hidden input — eats the keyboard burst from the desk scanner */}
          <textarea
            ref={inputRef}
            className="absolute -left-[9999px] top-0 h-px w-px opacity-0"
            tabIndex={-1}
            autoComplete="off"
            spellCheck={false}
            onInput={onInput}
            aria-hidden="true"
          />

          {(stage.kind === 'placement' || stage.kind === 'reading') && <PlacementIllustration />}
          {stage.kind === 'writing' && <PenWritingIllustration fields={stage.fields} />}
          {stage.kind === 'failed' && (
            <div className="flex flex-col items-center gap-4 text-center">
              <p className="display text-2xl font-semibold text-ink">{t('scanner.failed')}</p>
              <button type="button" onClick={onRetry} className="kiosk-btn-primary">
                {t('scanner.retry')}
              </button>
            </div>
          )}
        </div>

        <footer className="flex flex-col gap-2 px-6 py-5 tablet:flex-row tablet:items-center tablet:justify-between">
          <p className="text-base text-ink-muted">
            {stage.kind === 'placement' && t('external_scanner.placement_hint')}
            {stage.kind === 'reading' && t('external_scanner.reading')}
            {stage.kind === 'writing' && t('external_scanner.writing_hint')}
            {stage.kind === 'failed' && t('scanner.failed')}
          </p>
          <div className="flex items-center gap-3">
            {stage.kind === 'placement' && (
              <span className="kiosk-pulse-dot inline-flex items-center gap-2 rounded-full bg-brand-soft px-3 py-1.5 text-xs font-semibold uppercase tracking-wider text-brand">
                <span className="block h-2 w-2 rounded-full bg-brand" />
                {t('external_scanner.waiting')}
              </span>
            )}
            <button
              type="button"
              onClick={onClose}
              className="kiosk-btn-secondary"
            >
              {t('scanner.cancel')}
            </button>
          </div>
        </footer>
      </div>
    </div>
  );
};

// ──────────────────────────────────────────────────────────────────────────
// Placement illustration — passport hovering above an open scanner bed,
// with a sweeping scan line and a soft shadow on the bed.
// ──────────────────────────────────────────────────────────────────────────
const PlacementIllustration = () => (
  <div className="relative flex flex-col items-center gap-6">
    <svg
      width="240"
      height="200"
      viewBox="0 0 240 200"
      role="img"
      aria-hidden="true"
      className="select-none"
    >
      {/* Scanner bed (floor) */}
      <defs>
        <linearGradient id="bedGlass" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--kt-surface)" stopOpacity="1" />
          <stop offset="100%" stopColor="var(--kt-surface-muted)" stopOpacity="1" />
        </linearGradient>
        <linearGradient id="scanLine" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stopColor="var(--kt-brand)" stopOpacity="0" />
          <stop offset="50%" stopColor="var(--kt-brand)" stopOpacity="0.95" />
          <stop offset="100%" stopColor="var(--kt-brand)" stopOpacity="0" />
        </linearGradient>
      </defs>

      {/* Bed shadow */}
      <ellipse cx="120" cy="186" rx="92" ry="6" fill="var(--kt-ink)" opacity="0.08" />
      {/* Bed body */}
      <rect x="20" y="120" width="200" height="60" rx="10" fill="var(--kt-ink)" opacity="0.85" />
      {/* Glass top */}
      <rect x="28" y="128" width="184" height="44" rx="4" fill="url(#bedGlass)" stroke="var(--kt-border)" strokeWidth="1" />
      {/* Sweeping scan line */}
      <g className="kiosk-anim-sweep">
        <rect x="32" y="130" width="176" height="3" rx="1.5" fill="url(#scanLine)" />
      </g>

      {/* Passport — bobbing */}
      <g className="kiosk-anim-bob">
        <rect x="64" y="42" width="112" height="74" rx="6" fill="var(--kt-brand)" />
        <rect x="68" y="46" width="104" height="66" rx="4" fill="var(--kt-brand)" stroke="var(--kt-accent)" strokeOpacity="0.65" strokeWidth="1.5" />
        {/* Crest dot */}
        <circle cx="120" cy="62" r="4" fill="var(--kt-accent)" />
        {/* Crest ring */}
        <circle cx="120" cy="62" r="9" fill="none" stroke="var(--kt-accent)" strokeOpacity="0.55" strokeWidth="1.2" />
        {/* MRZ-band hint */}
        <rect x="74" y="92" width="92" height="3" rx="1" fill="var(--kt-accent)" opacity="0.45" />
        <rect x="74" y="100" width="92" height="3" rx="1" fill="var(--kt-accent)" opacity="0.45" />
      </g>
    </svg>
  </div>
);

// ──────────────────────────────────────────────────────────────────────────
// Pen-on-paper illustration — three "ink lines" appear sequentially while a
// pen tracks left-to-right. SMIL animations keep this pure-SVG; no JS loop.
// ──────────────────────────────────────────────────────────────────────────
const PenWritingIllustration = ({ fields }: { fields: MrzFields }) => {
  const summary = [fields.lname, fields.fname].filter(Boolean).join(', ');
  return (
    <div className="flex flex-col items-center gap-5">
      <svg
        width="240"
        height="180"
        viewBox="0 0 240 180"
        role="img"
        aria-hidden="true"
        className="select-none"
      >
        {/* Paper */}
        <rect x="22" y="14" width="196" height="148" rx="8" fill="var(--kt-surface)" stroke="var(--kt-border)" strokeWidth="1" />
        {/* Decorative top bar (form heading) */}
        <rect x="38" y="30" width="60" height="6" rx="2" fill="var(--kt-ink)" opacity="0.25" />
        {/* Three writing lines — revealed by a left-to-right mask */}
        <defs>
          <clipPath id="writeMask1">
            <rect x="38" y="56" width="0" height="6" rx="2">
              <animate attributeName="width" from="0" to="156" begin="0s" dur="0.55s" fill="freeze" />
            </rect>
          </clipPath>
          <clipPath id="writeMask2">
            <rect x="38" y="84" width="0" height="6" rx="2">
              <animate attributeName="width" from="0" to="124" begin="0.6s" dur="0.55s" fill="freeze" />
            </rect>
          </clipPath>
          <clipPath id="writeMask3">
            <rect x="38" y="112" width="0" height="6" rx="2">
              <animate attributeName="width" from="0" to="172" begin="1.2s" dur="0.55s" fill="freeze" />
            </rect>
          </clipPath>
        </defs>
        <rect x="38" y="56" width="156" height="6" rx="2" fill="var(--kt-brand)" clipPath="url(#writeMask1)" />
        <rect x="38" y="84" width="124" height="6" rx="2" fill="var(--kt-brand)" clipPath="url(#writeMask2)" />
        <rect x="38" y="112" width="172" height="6" rx="2" fill="var(--kt-brand)" clipPath="url(#writeMask3)" />

        {/* Pen — tracks across, then bounces between lines */}
        <g>
          <line x1="0" y1="0" x2="0" y2="0" stroke="transparent" />
          <g className="kiosk-anim-pen">
            {/* Pen body */}
            <rect x="-6" y="-44" width="12" height="44" rx="3" fill="var(--kt-ink)" />
            <polygon points="-6,0 6,0 0,8" fill="var(--kt-accent)" />
            <rect x="-6" y="-44" width="12" height="6" rx="1" fill="var(--kt-accent)" />
            <animateMotion
              dur="1.8s"
              fill="freeze"
              path="M 38 60 L 194 60 M 38 88 L 162 88 M 38 116 L 210 116"
              keyPoints="0; 0.31; 0.31; 0.56; 0.56; 1"
              keyTimes="0; 0.31; 0.34; 0.65; 0.68; 1"
              calcMode="linear"
            />
          </g>
        </g>
      </svg>
      {summary && (
        <p className="text-base text-ink-muted">
          <span className="font-medium text-ink">{summary}</span>
        </p>
      )}
    </div>
  );
};

// ──────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────

/**
 * Splits a keyboard-wedge MRZ burst into 2 or 3 lines. Some scanners send
 * newlines; others concatenate everything. We try the newline path first
 * and fall back to known fixed widths (TD3=44, TD2=36, TD1=30).
 */
function splitMrzCandidate(raw: string): string[] {
  // 1. Keep MRZ alphabet only — drop tabs, CRs, terminating Enter, etc.
  const cleaned = raw.replace(/[^A-Z0-9<\n]/g, '');
  // 2. Newline path
  const fromLines = extractMrzLines(cleaned);
  if (fromLines.length >= 2) return fromLines;
  // 3. Single-burst path — slice by known widths.
  const flat = cleaned.replace(/\n/g, '');
  if (flat.length === 88) return [flat.slice(0, 44), flat.slice(44, 88)];
  if (flat.length === 72) return [flat.slice(0, 36), flat.slice(36, 72)];
  if (flat.length === 90) return [flat.slice(0, 30), flat.slice(30, 60), flat.slice(60, 90)];
  // 4. Tolerant: split into 2 halves if length is even and ≥56.
  if (flat.length >= 56 && flat.length % 2 === 0) {
    const half = flat.length / 2;
    return [flat.slice(0, half), flat.slice(half)];
  }
  return [];
}

type MrzParsed = Awaited<ReturnType<typeof import('mrz').parse>>;

function toMrzFields(p: MrzParsed): MrzFields {
  const f = p.fields;
  return {
    fname: typeof f.firstName === 'string' ? f.firstName : undefined,
    lname: typeof f.lastName === 'string' ? f.lastName : undefined,
    dob: typeof f.birthDate === 'string' ? isoFromMrzDate(f.birthDate) : '',
    nationality: typeof f.nationality === 'string' ? f.nationality.toUpperCase() : undefined,
    country: typeof f.nationality === 'string' ? alpha2FromMrzNationality(f.nationality) : undefined,
    document_id:
      typeof f.documentNumber === 'string'
        ? f.documentNumber.replace(/<+$/, '')
        : undefined,
    document: p.format === 'TD3' ? 1 : 2,
  };
}
