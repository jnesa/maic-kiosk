import { FormEvent, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { KioskShell } from '@/components/KioskShell';
import { Field } from '@/components/Field';
import { lookupReservation } from '@/api/lookup';
import { useSession } from '@/store/session';

export const LookupByIdPage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const start = useSession((s) => s.start);
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!code.trim()) return;
    setBusy(true);
    setErr(null);
    try {
      const res = await lookupReservation({ reservationId: code.trim() });
      switch (res.result) {
        case 'matched':
          start(res.reservation, 'reservation_id');
          navigate('/checkin/1');
          break;
        case 'not_found':
          navigate('/lookup/not-found');
          break;
        default:
          setErr(t('lookup.notFound.title'));
      }
    } catch {
      navigate('/error');
    } finally {
      setBusy(false);
    }
  };

  return (
    <KioskShell>
      <div className="mx-auto flex w-full max-w-2xl flex-col justify-center px-6 py-10 tablet:px-10">
        <h1 className="display text-5xl font-semibold leading-tight tracking-tight text-ink tablet:text-6xl">
          {t('lookup.byId.title')}
        </h1>
        <form onSubmit={onSubmit} className="mt-10 space-y-6">
          <Field label={t('lookup.byId.placeholder')} required error={err ?? undefined}>
            {(id) => (
              <input
                id={id}
                autoFocus
                value={code}
                onChange={(e) => setCode(e.target.value.toUpperCase())}
                className="kiosk-input text-3xl tracking-widest"
                placeholder="PAR-90021"
                autoComplete="off"
                spellCheck={false}
              />
            )}
          </Field>
          <div className="flex flex-col gap-4 tablet:flex-row tablet:items-center tablet:justify-between">
            <Link to="/lookup/last-name" className="kiosk-btn-tertiary">
              ← {t('lookup.back')}
            </Link>
            <button type="submit" disabled={busy || !code.trim()} className="kiosk-btn-primary">
              {busy ? '…' : t('lookup.continue')}
            </button>
          </div>
        </form>
      </div>
    </KioskShell>
  );
};
