import { FormEvent, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { KioskShell } from '@/components/KioskShell';
import { Field } from '@/components/Field';
import { lookupReservation } from '@/api/lookup';
import { useSession } from '@/store/session';
import { useLookup } from '@/store/lookup';

// Stage A: ask only for the last name. Mews-style — minimal typing.
export const LookupLastNamePage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const start = useSession((s) => s.start);
  const setCandidate = useLookup((s) => s.setCandidate);
  const [lastName, setLastName] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!lastName.trim()) return;
    setBusy(true);
    setErr(null);
    try {
      const res = await lookupReservation({ lastName: lastName.trim() });
      switch (res.result) {
        case 'matched':
          start(res.reservation, 'last_name');
          navigate('/checkin/1');
          break;
        case 'ambiguous':
          setCandidate(res.candidateToken, res.candidates, lastName.trim());
          navigate('/lookup/pick-guest');
          break;
        case 'not_found':
          navigate('/lookup/not-found');
          break;
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
        <p className="text-sm font-medium uppercase tracking-[0.25em] text-ink-muted">
          1 / 3
        </p>
        <h1 className="display mt-3 text-5xl font-semibold leading-tight tracking-tight text-ink tablet:text-6xl">
          {t('lookup.lastName.title')}
        </h1>
        <p className="mt-4 text-lg text-ink-muted">{t('lookup.lastName.hint')}</p>
        <form onSubmit={onSubmit} className="mt-10 space-y-6">
          <Field label={t('lookup.lastName.placeholder')} required error={err ?? undefined}>
            {(id) => (
              <input
                id={id}
                autoFocus
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                className="kiosk-input text-3xl"
                placeholder="Rossi"
                autoCapitalize="words"
                autoComplete="off"
                spellCheck={false}
              />
            )}
          </Field>
          <div className="flex flex-col gap-4 tablet:flex-row tablet:items-center tablet:justify-between">
            <Link to="/lookup/booking" className="kiosk-btn-tertiary">
              {t('lookup.haveBookingNumber')}
            </Link>
            <button
              type="submit"
              disabled={busy || !lastName.trim()}
              className="kiosk-btn-primary"
            >
              {busy ? '…' : t('lookup.continue')}
              <span aria-hidden>→</span>
            </button>
          </div>
        </form>
      </div>
    </KioskShell>
  );
};
