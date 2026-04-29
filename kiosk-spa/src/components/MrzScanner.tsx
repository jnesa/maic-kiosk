import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Camera01, XClose, RefreshCw01 } from '@untitledui/icons';
import {
  cropMrzImageData,
  extractMrzLines,
  isoFromMrzDate,
  alpha2FromMrzNationality,
  type MrzFields,
} from '@/lib/mrz';
// `tesseract.js` and `mrz` are imported dynamically below so the entry chunk
// stays small. They only load when a guest taps "Scan ID".
import type { Worker as TesseractWorker } from 'tesseract.js';

interface MrzScannerProps {
  open: boolean;
  onClose: () => void;
  onParsed: (fields: MrzFields) => void;
}

type Status =
  | { kind: 'init' }
  | { kind: 'permission_denied' }
  | { kind: 'no_camera' }
  | { kind: 'streaming' }
  | { kind: 'reading' }
  | { kind: 'failed' };

/**
 * Camera-based MRZ scanner. Lazily imports tesseract.js + mrz so the entry
 * chunk stays small. Auto-captures the MRZ band every 1.5s; bails out after
 * 5 failed parses with a clear retry/cancel screen.
 */
export const MrzScanner = ({ open, onClose, onParsed }: MrzScannerProps) => {
  const { t } = useTranslation();
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const workerRef = useRef<TesseractWorker | null>(null);
  const busyRef = useRef(false);
  const attemptsRef = useRef(0);
  const tickRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [status, setStatus] = useState<Status>({ kind: 'init' });

  // ──────────────────────────────────────────────────────────────────────
  // Lifecycle: start camera + OCR worker on open, tear down on close.
  // ──────────────────────────────────────────────────────────────────────
  useEffect(() => {
    if (!open) return;

    let cancelled = false;
    void (async () => {
      // 1. Camera
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: {
            facingMode: { ideal: 'environment' },
            width: { ideal: 1280 },
            height: { ideal: 720 },
          },
          audio: false,
        });
        if (cancelled) {
          stream.getTracks().forEach((tr) => tr.stop());
          return;
        }
        streamRef.current = stream;
        if (videoRef.current) {
          videoRef.current.srcObject = stream;
          await videoRef.current.play().catch(() => undefined);
        }
        setStatus({ kind: 'streaming' });
      } catch (err) {
        if (cancelled) return;
        const name = (err as DOMException | null)?.name ?? '';
        if (name === 'NotAllowedError' || name === 'SecurityError') {
          setStatus({ kind: 'permission_denied' });
        } else {
          setStatus({ kind: 'no_camera' });
        }
        return;
      }

      // 2. Tesseract worker (lazy import)
      try {
        const { createWorker } = await import('tesseract.js');
        // MRZ uses A-Z, 0-9, '<' as filler. Whitelisting drops accuracy of
        // adjacent letters by ~30% in our experience.
        const worker = await createWorker('eng');
        await worker.setParameters({
          tessedit_char_whitelist: 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789<',
        });
        if (cancelled) {
          await worker.terminate();
          return;
        }
        workerRef.current = worker;
      } catch {
        // OCR worker failed — surface a generic failure but keep the camera up
        // so the user gets a clear retry path.
        if (!cancelled) setStatus({ kind: 'failed' });
        return;
      }

      // 3. Capture loop — every 1.5s, capture+OCR if not already busy.
      tickRef.current = setInterval(() => {
        if (busyRef.current) return;
        void runOnce();
      }, 1500);
    })();

    return () => {
      cancelled = true;
      if (tickRef.current) {
        clearInterval(tickRef.current);
        tickRef.current = null;
      }
      if (streamRef.current) {
        streamRef.current.getTracks().forEach((tr) => tr.stop());
        streamRef.current = null;
      }
      if (workerRef.current) {
        const w = workerRef.current;
        workerRef.current = null;
        void w.terminate().catch(() => undefined);
      }
      busyRef.current = false;
      attemptsRef.current = 0;
      // Only reset status when truly closed (open=false), not on cleanup
      // triggered by re-render.
      setStatus({ kind: 'init' });
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  // ──────────────────────────────────────────────────────────────────────
  // Single capture+OCR+parse attempt.
  // ──────────────────────────────────────────────────────────────────────
  const runOnce = async () => {
    if (busyRef.current) return;
    if (!videoRef.current || !canvasRef.current || !workerRef.current) return;
    busyRef.current = true;
    setStatus({ kind: 'reading' });
    try {
      const img = cropMrzImageData(videoRef.current, canvasRef.current);
      if (!img) {
        busyRef.current = false;
        setStatus({ kind: 'streaming' });
        return;
      }
      const ocr = await workerRef.current.recognize(canvasRef.current);
      const lines = extractMrzLines(ocr.data.text);
      if (lines.length < 2) {
        attemptsRef.current += 1;
        if (attemptsRef.current >= 5) setStatus({ kind: 'failed' });
        else setStatus({ kind: 'streaming' });
        busyRef.current = false;
        return;
      }
      const { parse } = await import('mrz');
      const parsed = parse(lines);
      if (!parsed.valid) {
        attemptsRef.current += 1;
        if (attemptsRef.current >= 5) setStatus({ kind: 'failed' });
        else setStatus({ kind: 'streaming' });
        busyRef.current = false;
        return;
      }
      const fields = toMrzFields(parsed);
      // Stop the loop, clean up, and return up.
      if (tickRef.current) {
        clearInterval(tickRef.current);
        tickRef.current = null;
      }
      onParsed(fields);
    } catch {
      attemptsRef.current += 1;
      if (attemptsRef.current >= 5) setStatus({ kind: 'failed' });
      else setStatus({ kind: 'streaming' });
    } finally {
      busyRef.current = false;
    }
  };

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-40 flex items-center justify-center bg-ink/85 px-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-label={t('scanner.title')}
    >
      <div className="kiosk-card relative w-full max-w-3xl overflow-hidden p-0">
        <header className="flex items-center justify-between gap-3 border-b border-border-subtle px-6 py-4">
          <div className="flex items-center gap-3">
            <Camera01 className="h-6 w-6 text-brand" />
            <h2 className="display text-xl font-semibold text-ink">{t('scanner.title')}</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-full p-2 text-ink-muted transition-colors hover:bg-surface-muted"
            aria-label={t('scanner.cancel')}
          >
            <XClose className="h-5 w-5" />
          </button>
        </header>

        <div className="relative aspect-[16/10] w-full bg-black">
          {/* Live camera preview */}
          <video
            ref={videoRef}
            playsInline
            muted
            className="h-full w-full object-cover"
          />
          {/* Hidden capture target */}
          <canvas ref={canvasRef} className="hidden" />

          {/* Guide rectangle — matches cropMrzImageData (88% × 22% at 70% from top) */}
          {(status.kind === 'streaming' || status.kind === 'reading') && (
            <svg
              className="pointer-events-none absolute inset-0 h-full w-full"
              viewBox="0 0 100 100"
              preserveAspectRatio="none"
            >
              <rect
                x="6"
                y="70"
                width="88"
                height="22"
                rx="1.5"
                fill="none"
                stroke="rgba(255, 255, 255, 0.85)"
                strokeWidth="0.4"
                strokeDasharray="2 1"
              />
            </svg>
          )}

          {/* Status overlays */}
          {status.kind === 'init' && (
            <div className="absolute inset-0 flex items-center justify-center bg-ink/40 text-surface">
              <p className="text-base">{t('scanner.scanning')}</p>
            </div>
          )}
          {status.kind === 'reading' && (
            <div className="pointer-events-none absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full bg-ink/85 px-4 py-2 text-sm font-medium text-surface backdrop-blur">
              {t('scanner.scanning')}
            </div>
          )}
          {status.kind === 'permission_denied' && (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-4 bg-ink/85 px-8 text-center text-surface">
              <Camera01 className="h-10 w-10 opacity-80" />
              <p className="text-base">{t('scanner.permission_denied')}</p>
            </div>
          )}
          {status.kind === 'no_camera' && (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-4 bg-ink/85 px-8 text-center text-surface">
              <p className="text-base">{t('scanner.failed')}</p>
            </div>
          )}
          {status.kind === 'failed' && (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-4 bg-ink/85 px-8 text-center text-surface">
              <p className="text-base">{t('scanner.failed')}</p>
            </div>
          )}
        </div>

        <footer className="flex flex-col gap-3 px-6 py-4 tablet:flex-row tablet:items-center tablet:justify-between">
          <p className="text-sm text-ink-muted">{t('scanner.position')}</p>
          <div className="flex items-center gap-3">
            <button type="button" onClick={onClose} className="kiosk-btn-secondary">
              {t('scanner.cancel')}
            </button>
            {status.kind === 'failed' && (
              <button
                type="button"
                onClick={() => {
                  attemptsRef.current = 0;
                  setStatus({ kind: 'streaming' });
                  if (!tickRef.current) {
                    tickRef.current = setInterval(() => {
                      if (busyRef.current) return;
                      void runOnce();
                    }, 1500);
                  }
                }}
                className="kiosk-btn-primary"
              >
                <RefreshCw01 className="h-5 w-5" />
                {t('scanner.retry')}
              </button>
            )}
          </div>
        </footer>
      </div>
    </div>
  );
};

// ──────────────────────────────────────────────────────────────────────────
// Adapter — narrows the discriminated union mrz.parse returns into our
// stable MrzFields contract.
// ──────────────────────────────────────────────────────────────────────────
type MrzParsed = Awaited<ReturnType<typeof import('mrz').parse>>;

function toMrzFields(p: MrzParsed): MrzFields {
  const f = p.fields;
  const fmt = p.format;
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
    document: fmt === 'TD3' ? 1 : 2,
  };
}
