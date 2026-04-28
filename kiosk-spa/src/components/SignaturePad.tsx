import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface Props {
  initial?: string;
  onChange: (dataUrl: string) => void;
}

// HTML5 canvas signature capture. Adapted from
// GuestSPA/src/features/prestay/PrestayPage.tsx — bigger canvas, kiosk-friendly
// stroke width. Pointer events handle mouse + touch + stylus uniformly.
export const SignaturePad = ({ initial, onChange }: Props) => {
  const ref = useRef<HTMLCanvasElement>(null);
  const drawing = useRef(false);
  const [hasInk, setHasInk] = useState(false);
  const { t } = useTranslation();

  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const ratio = window.devicePixelRatio || 1;
    canvas.width = canvas.clientWidth * ratio;
    canvas.height = canvas.clientHeight * ratio;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.scale(ratio, ratio);
    ctx.lineWidth = 2.5;
    ctx.lineCap = 'round';
    ctx.strokeStyle = '#0F2942';
    if (initial && initial.startsWith('data:image')) {
      const img = new Image();
      img.onload = () => ctx.drawImage(img, 0, 0, canvas.clientWidth, canvas.clientHeight);
      img.src = initial;
      setHasInk(true);
    }
  }, [initial]);

  const pos = (e: React.PointerEvent<HTMLCanvasElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    return { x: e.clientX - rect.left, y: e.clientY - rect.top };
  };

  const start = (e: React.PointerEvent<HTMLCanvasElement>) => {
    e.preventDefault();
    e.currentTarget.setPointerCapture(e.pointerId);
    drawing.current = true;
    const ctx = e.currentTarget.getContext('2d');
    if (!ctx) return;
    const p = pos(e);
    ctx.beginPath();
    ctx.moveTo(p.x, p.y);
  };

  const move = (e: React.PointerEvent<HTMLCanvasElement>) => {
    if (!drawing.current) return;
    e.preventDefault();
    const ctx = e.currentTarget.getContext('2d');
    if (!ctx) return;
    const p = pos(e);
    ctx.lineTo(p.x, p.y);
    ctx.stroke();
    setHasInk(true);
  };

  const end = (e: React.PointerEvent<HTMLCanvasElement>) => {
    if (!drawing.current) return;
    drawing.current = false;
    e.currentTarget.releasePointerCapture(e.pointerId);
    const canvas = ref.current;
    if (!canvas) return;
    onChange(canvas.toDataURL('image/png'));
  };

  const clear = () => {
    const canvas = ref.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    setHasInk(false);
    onChange('');
  };

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-ink-muted">{t('firm.signatureHint')}</p>
      <canvas
        ref={ref}
        className="h-56 w-full touch-none rounded-3xl border-2 border-dashed border-border-subtle bg-surface"
        onPointerDown={start}
        onPointerMove={move}
        onPointerUp={end}
        onPointerCancel={end}
      />
      <div className="flex justify-end">
        <button type="button" onClick={clear} disabled={!hasInk} className="kiosk-btn-tertiary disabled:opacity-40">
          {t('firm.signatureClear')}
        </button>
      </div>
    </div>
  );
};
