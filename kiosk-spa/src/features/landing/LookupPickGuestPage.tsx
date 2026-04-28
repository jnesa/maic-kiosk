import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { KioskShell } from '@/components/KioskShell';
import { useLookup } from '@/store/lookup';
import { useSession } from '@/store/session';
import { selectCandidate } from '@/api/lookup';

// Stage B: multiple reservations share the same surname. Show a tile grid of
// first names — minimum PII, maximum tap target.
export const LookupPickGuestPage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { candidateToken, candidates, lastName, clear } = useLookup();
  const start = useSession((s) => s.start);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!candidateToken || candidates.length === 0) navigate('/lookup/last-name', { replace: true });
  }, [candidateToken, candidates.length, navigate]);

  const onPick = async (candidateId: string) => {
    setBusy(true);
    try {
      const res = await selectCandidate(candidateToken, candidateId);
      if (res.result === 'matched') {
        start(res.reservation, 'last_name');
        clear();
        navigate('/checkin/1');
      } else if (res.result === 'ambiguous') {
        // Same first name across multiple reservations — fall back to booking
        // number entry. Keeping UX simple is more important than a fourth screen.
        navigate('/lookup/booking');
      } else {
        navigate('/lookup/not-found');
      }
    } catch {
      navigate('/error');
    } finally {
      setBusy(false);
    }
  };

  return (
    <KioskShell>
      <div className="mx-auto flex w-full max-w-4xl flex-col justify-center px-6 py-10 tablet:px-10">
        <p className="text-sm font-medium uppercase tracking-[0.25em] text-ink-muted">
          {lastName ? `${lastName.toUpperCase()}` : ''}
        </p>
        <h1 className="display mt-3 text-4xl font-semibold leading-tight tracking-tight text-ink tablet:text-5xl">
          {t('lookup.pickGuest.title')}
        </h1>
        <p className="mt-3 text-lg text-ink-muted">{t('lookup.pickGuest.subtitle')}</p>
        <div className="mt-10 grid grid-cols-1 gap-4 tablet:grid-cols-2 desk:grid-cols-3">
          {candidates.map((c) => (
            <button
              key={c.candidateId}
              type="button"
              disabled={busy}
              onClick={() => onPick(c.candidateId)}
              className="kiosk-tile"
            >
              <span className="text-sm font-medium uppercase tracking-[0.2em] text-ink-muted">
                {t('checkin.summary.eyebrow')}
              </span>
              <span className="display mt-3 text-3xl font-semibold text-ink">
                {c.firstName}
              </span>
            </button>
          ))}
        </div>
      </div>
    </KioskShell>
  );
};
