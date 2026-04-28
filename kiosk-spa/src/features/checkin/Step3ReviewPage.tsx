import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { SignaturePad } from '@/components/SignaturePad';
import { useForm, fc } from '@/store/form';
import { useSession } from '@/store/session';
import { useProperty } from '@/store/property';
import { saveFirm, submitCheckin } from '@/api/checkin';
import { BookingConfirmationCard } from '@/features/checkin/BookingConfirmationCard';

export const Step3ReviewPage = () => {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { payload, firm, patchFirm, guests } = useForm();
  const { lookupMethod, reservation } = useSession();
  const property = useProperty((s) => s.property);
  const cfg = payload?.config;
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const sigField = fc(cfg, 'signature-pad');
  const sigRequired = sigField?.use && sigField.required === 1;

  const onSubmit = async () => {
    if (sigRequired && !firm.signature.startsWith('data:image/png;base64,')) {
      setErr(t('firm.signature') + ' — ' + t('common.required'));
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      // Persist any final signature edits before submit, then finalise.
      if (!reservation) return;
      await saveFirm(reservation.id, firm);
      await submitCheckin(reservation.id, lookupMethod || 'last_name', i18n.resolvedLanguage ?? 'en');
      navigate('/done');
    } catch {
      navigate('/error');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-6">
      <section className="kiosk-card">
        <h2 className="display text-3xl font-semibold tracking-tight text-ink">{t('review.title')}</h2>
        <p className="mt-2 text-lg text-ink-muted">{t('review.subtitle')}</p>

        {reservation && (
          <div className="mt-8">
            <BookingConfirmationCard reservation={reservation} property={property} />
          </div>
        )}

        {guests.some((g) => g.fname || g.lname) && (
          <div className="mt-8 rounded-2xl border border-ink/10 bg-surface-muted p-4">
            <h3 className="text-sm font-semibold uppercase tracking-wide text-ink-muted">{t('step.guest')}</h3>
            <p className="mt-2 text-base text-ink">
              {guests
                .filter((g) => g.fname || g.lname)
                .map((g) => `${g.fname} ${g.lname}`.trim())
                .join(', ')}
            </p>
          </div>
        )}

        {sigField?.use && (
          <div className="mt-8">
            <h3 className="text-lg font-semibold text-ink">{t('firm.signature')}</h3>
            <div className="mt-3">
              <SignaturePad initial={firm.signature} onChange={(d) => patchFirm({ signature: d })} />
            </div>
            {err && <p className="mt-3 text-sm font-medium text-red-600">{err}</p>}
          </div>
        )}
      </section>

      <div className="flex flex-col gap-3 tablet:flex-row tablet:justify-between">
        <button type="button" onClick={() => navigate('/checkin/2')} className="kiosk-btn-secondary">
          ← {t('common.back')}
        </button>
        <button type="button" onClick={onSubmit} disabled={busy} className="kiosk-btn-primary">
          {busy ? '…' : t('review.submit')}
          <span aria-hidden>✓</span>
        </button>
      </div>
    </div>
  );
};
