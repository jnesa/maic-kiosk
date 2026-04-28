import { useEffect, useRef, useState } from 'react';
import QRCode from 'qrcode';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  rotateKioskKey,
  disableKiosk,
  enableKiosk,
  deleteKiosk,
} from '@/api/hotels';
import { kioskUrl } from '@/utils/format';
import type { Kiosk } from '@/api/types';

interface Props {
  kiosk: Kiosk;
  onClose: () => void;
  onChange: () => void;
}

/**
 * Modal-style detail card for a kiosk. Shows the public URL + QR + the
 * device key that needs to be pasted into the property's PMSApi `.env`.
 * Provides rotate-key, disable/enable, and delete actions.
 */
export const KioskDetailCard = ({ kiosk, onClose, onChange }: Props) => {
  const qc = useQueryClient();
  const [k, setK] = useState<Kiosk>(kiosk);
  const [revealKey, setRevealKey] = useState(false);
  const url = kioskUrl(k.id);

  const refreshAll = () => {
    onChange();
    void qc.invalidateQueries({ queryKey: ['hotel', k.hotel_id] });
  };

  const rotate = useMutation({
    mutationFn: () => rotateKioskKey(k.id),
    onSuccess: ({ device_key }) => {
      setK({ ...k, device_key });
      setRevealKey(true);
      refreshAll();
    },
  });
  const dis = useMutation({
    mutationFn: () => disableKiosk(k.id),
    onSuccess: (next) => {
      setK(next);
      refreshAll();
    },
  });
  const en = useMutation({
    mutationFn: () => enableKiosk(k.id),
    onSuccess: (next) => {
      setK(next);
      refreshAll();
    },
  });
  const del = useMutation({
    mutationFn: () => deleteKiosk(k.id),
    onSuccess: () => {
      refreshAll();
      onClose();
    },
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 px-6">
      <div className="w-full max-w-2xl card max-h-[92vh] overflow-y-auto">
        <div className="card-header sticky top-0 bg-white">
          <div>
            <h2 className="text-base font-semibold">{k.display_name}</h2>
            <div className="font-mono text-xs text-slate-500">{k.id}</div>
          </div>
          <div className="flex items-center gap-3">
            <span className={k.status === 'active' ? 'badge-active' : 'badge-disabled'}>
              {k.status}
            </span>
            <button onClick={onClose} className="btn-ghost">
              ✕
            </button>
          </div>
        </div>

        <div className="card-body space-y-6">
          <UrlBlock url={url} />
          <DeviceKeyBlock
            value={k.device_key}
            revealed={revealKey}
            onReveal={() => setRevealKey(true)}
            onRotate={() => {
              if (confirm('Rotate this kiosk\'s device key? The old key will stop working immediately. You must update KIOSK_DEVICE_KEY on the PMSApi side or the kiosk will go offline until you do.')) {
                rotate.mutate();
              }
            }}
            rotating={rotate.isPending}
            legacyGroupID={k.legacy_group_id}
          />

          <div className="rounded-lg border border-slate-200 bg-slate-50/60 p-4 text-sm">
            <div className="grid grid-cols-2 gap-x-6 gap-y-2">
              <Field label="Theme" value={k.theme} />
              <Field label="Languages" value={k.languages.join(', ')} />
              <Field
                label="Legacy group"
                value={
                  k.legacy_group_id != null
                    ? `#${k.legacy_group_id}${k.legacy_group_label ? ` · ${k.legacy_group_label}` : ''}`
                    : '— (single tenant)'
                }
              />
              <Field label="Created" value={new Date(k.created_at).toLocaleString()} />
            </div>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-2 border-t border-slate-200 pt-4">
            <div className="flex flex-wrap gap-2">
              {k.status === 'active' ? (
                <button onClick={() => dis.mutate()} className="btn-secondary" disabled={dis.isPending}>
                  Disable kiosk
                </button>
              ) : (
                <button onClick={() => en.mutate()} className="btn-secondary" disabled={en.isPending}>
                  Re-enable kiosk
                </button>
              )}
            </div>
            <button
              className="btn-danger"
              onClick={() => {
                if (confirm('Delete this kiosk permanently? The URL will return 404.')) {
                  del.mutate();
                }
              }}
              disabled={del.isPending}
            >
              Delete kiosk
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

const Field = ({ label, value }: { label: string; value: string }) => (
  <div>
    <div className="text-xs uppercase tracking-wide text-slate-500">{label}</div>
    <div className="mt-0.5 text-slate-900">{value}</div>
  </div>
);

const UrlBlock = ({ url }: { url: string }) => {
  const [copied, setCopied] = useState(false);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    if (canvasRef.current) {
      void QRCode.toCanvas(canvasRef.current, url, { width: 168, margin: 1 });
    }
  }, [url]);
  const onCopy = async () => {
    await navigator.clipboard.writeText(url);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };
  return (
    <div>
      <div className="mb-1 text-xs font-medium uppercase tracking-wide text-slate-500">
        Kiosk URL
      </div>
      <div className="flex flex-wrap items-start gap-4">
        <canvas ref={canvasRef} className="rounded bg-white ring-1 ring-slate-200" />
        <div className="min-w-0 flex-1 space-y-2">
          <div className="overflow-hidden rounded-lg border border-slate-200 bg-slate-50 p-3">
            <code className="block break-all font-mono text-sm text-slate-800">{url}</code>
          </div>
          <div className="flex gap-2">
            <button onClick={onCopy} className="btn-secondary">
              {copied ? 'Copied ✓' : 'Copy URL'}
            </button>
            <a href={url} target="_blank" rel="noreferrer" className="btn-ghost">
              Open in new tab ↗
            </a>
          </div>
        </div>
      </div>
    </div>
  );
};

const DeviceKeyBlock = ({
  value,
  revealed,
  onReveal,
  onRotate,
  rotating,
  legacyGroupID,
}: {
  value: string;
  revealed: boolean;
  onReveal: () => void;
  onRotate: () => void;
  rotating: boolean;
  legacyGroupID: number | null;
}) => {
  const [copied, setCopied] = useState(false);
  const onCopy = async () => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };
  const masked = value ? value.slice(0, 6) + '…' + '·'.repeat(20) : '';
  return (
    <div>
      <div className="mb-1 text-xs font-medium uppercase tracking-wide text-slate-500">
        Legacy PMSApi configuration
      </div>
      <div className="rounded-lg border border-amber-200 bg-amber-50/40 p-4">
        <p className="mb-3 text-xs text-amber-900">
          Set these on the property's PMSApi <code className="font-mono">.env</code>, then run{' '}
          <code className="font-mono">php artisan config:clear</code>. Without them the kiosk will
          return 401 from <code className="font-mono">/api/kiosk/lookup</code>.
        </p>
        <div className="space-y-2 font-mono text-xs">
          <KeyValueLine label="KIOSK_DEVICE_KEY" value={revealed ? value : masked} />
          {legacyGroupID != null && (
            <KeyValueLine label="KIOSK_GROUP_ID" value={String(legacyGroupID)} />
          )}
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          {!revealed ? (
            <button onClick={onReveal} className="btn-secondary">
              Reveal device key
            </button>
          ) : (
            <button onClick={onCopy} className="btn-secondary">
              {copied ? 'Copied ✓' : 'Copy device key'}
            </button>
          )}
          <button onClick={onRotate} disabled={rotating} className="btn-ghost">
            {rotating ? 'Rotating…' : 'Rotate key'}
          </button>
        </div>
      </div>
    </div>
  );
};

const KeyValueLine = ({ label, value }: { label: string; value: string }) => (
  <div className="grid grid-cols-[200px,1fr] items-center gap-2 rounded bg-white px-3 py-1.5 ring-1 ring-amber-200">
    <span className="text-amber-900">{label}</span>
    <span className="break-all text-slate-900">{value}</span>
  </div>
);
