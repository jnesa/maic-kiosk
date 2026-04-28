import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Field } from '@/components/Field';
import { useForm, isVisible } from '@/store/form';
import { useSession } from '@/store/session';
import { saveFirm } from '@/api/checkin';
import { cn } from '@/utils/cn';

const Toggle = ({ on, onChange, label }: { on: boolean; onChange: (v: boolean) => void; label: string }) => (
  <button
    type="button"
    role="switch"
    aria-checked={on}
    onClick={() => onChange(!on)}
    className={cn(
      'flex w-full items-center justify-between rounded-2xl border px-5 py-4 text-left text-lg font-medium transition-colors',
      on ? 'border-brand bg-brand-soft/40 text-ink' : 'border-border-subtle bg-surface text-ink-muted',
    )}
  >
    <span>{label}</span>
    <span
      className={cn(
        'flex h-7 w-12 items-center rounded-full border transition-colors',
        on ? 'border-brand bg-brand' : 'border-border-subtle bg-surface-muted',
      )}
    >
      <span
        className={cn(
          'block h-5 w-5 rounded-full bg-surface shadow transition-transform',
          on ? 'translate-x-6' : 'translate-x-1',
        )}
      />
    </span>
  </button>
);

export const Step2FirmPage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const reservation = useSession((s) => s.reservation);
  const { payload, firm, patchFirm } = useForm();
  const cfg = payload?.config;
  const [busy, setBusy] = useState(false);

  const onContinue = async () => {
    setBusy(true);
    try {
      if (!reservation) return;
      await saveFirm(reservation.id, firm);
      navigate('/checkin/3');
    } catch {
      navigate('/error');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-6">
      <section className="kiosk-card">
        <h2 className="display text-2xl font-semibold text-ink">{t('step.firm')}</h2>
        <div className="mt-6 grid grid-cols-1 gap-5 tablet:grid-cols-2">
          {isVisible(cfg, 'compname') && (
            <Field label={t('firm.compname')}>
              {(id) => (
                <input id={id} className="kiosk-input" value={firm.compname} onChange={(e) => patchFirm({ compname: e.target.value })} />
              )}
            </Field>
          )}
          {isVisible(cfg, 'vatid') && (
            <Field label={t('firm.vatid')}>
              {(id) => (
                <input id={id} className="kiosk-input" value={firm.vatid} onChange={(e) => patchFirm({ vatid: e.target.value })} />
              )}
            </Field>
          )}
          {isVisible(cfg, 'prestayEmail') && (
            <Field label={t('firm.email')}>
              {(id) => (
                <input id={id} type="email" className="kiosk-input" value={firm.email} onChange={(e) => patchFirm({ email: e.target.value })} autoComplete="off" />
              )}
            </Field>
          )}
          {isVisible(cfg, 'prestayPhone') && (
            <Field label={t('firm.phone')}>
              {(id) => (
                <input id={id} type="tel" className="kiosk-input" value={firm.phone} onChange={(e) => patchFirm({ phone: e.target.value })} autoComplete="off" />
              )}
            </Field>
          )}
        </div>

        <div className="mt-8 grid grid-cols-1 gap-3 tablet:grid-cols-2">
          {isVisible(cfg, 'babyBed') && (
            <div className="flex flex-col gap-3">
              <Toggle on={firm.babyBed} onChange={(v) => patchFirm({ babyBed: v })} label={t('firm.babyBed')} />
              {firm.babyBed && (
                <input className="kiosk-input" placeholder={t('firm.babyBed.text')} value={firm.babyBedText} onChange={(e) => patchFirm({ babyBedText: e.target.value })} />
              )}
            </div>
          )}
          {isVisible(cfg, 'dogPackage') && (
            <div className="flex flex-col gap-3">
              <Toggle on={firm.dogPackage} onChange={(v) => patchFirm({ dogPackage: v })} label={t('firm.dogPackage')} />
              {firm.dogPackage && (
                <input className="kiosk-input" placeholder={t('firm.dogPackage.text')} value={firm.dogPackageText} onChange={(e) => patchFirm({ dogPackageText: e.target.value })} />
              )}
            </div>
          )}
          {isVisible(cfg, 'alergies') && (
            <div className="flex flex-col gap-3">
              <Toggle on={firm.alergies} onChange={(v) => patchFirm({ alergies: v })} label={t('firm.alergies')} />
              {firm.alergies && (
                <input className="kiosk-input" placeholder={t('firm.alergies.text')} value={firm.alergiesText} onChange={(e) => patchFirm({ alergiesText: e.target.value })} />
              )}
            </div>
          )}
          {isVisible(cfg, 'transfer') && (
            <div className="flex flex-col gap-3">
              <Toggle on={firm.transfer} onChange={(v) => patchFirm({ transfer: v })} label={t('firm.transfer')} />
              {firm.transfer && (
                <input className="kiosk-input" placeholder={t('firm.transfer.text')} value={firm.transferText} onChange={(e) => patchFirm({ transferText: e.target.value })} />
              )}
            </div>
          )}
          {isVisible(cfg, 'accessible') && (
            <Toggle on={firm.accessible} onChange={(v) => patchFirm({ accessible: v })} label={t('firm.accessible')} />
          )}
        </div>
      </section>

      <div className="flex flex-col gap-3 tablet:flex-row tablet:justify-between">
        <button type="button" onClick={() => navigate('/checkin/1')} className="kiosk-btn-secondary">
          ← {t('common.back')}
        </button>
        <button type="button" onClick={onContinue} disabled={busy} className="kiosk-btn-primary">
          {busy ? '…' : t('common.next')}
          <span aria-hidden>→</span>
        </button>
      </div>
    </div>
  );
};
