import { useEffect, useState } from 'react';
import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { KioskShell } from '@/components/KioskShell';
import { StepProgress } from '@/components/StepProgress';
import { useSession } from '@/store/session';
import { useForm } from '@/store/form';
import { fetchForm } from '@/api/checkin';
import { useTranslation } from 'react-i18next';

// Loads the form payload (config + prefill) once for the active reservation,
// then renders the per-step <Outlet/>. Each step manages its own validation
// and per-step save call.
export const CheckinLayout = () => {
  const reservation = useSession((s) => s.reservation);
  const { payload, setPayload } = useForm();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const location = useLocation();
  const { t, i18n } = useTranslation();

  useEffect(() => {
    if (!reservation) return;
    void (async () => {
      try {
        const p = await fetchForm(reservation.id);
        setPayload(p);
      } catch {
        setError('load_failed');
      } finally {
        setLoading(false);
      }
    })();
  }, [reservation, setPayload]);

  if (!reservation) return <Navigate to="/" replace />;
  if (error) return <Navigate to="/error" replace />;

  const step = location.pathname.endsWith('/3') ? 3 : location.pathname.endsWith('/2') ? 2 : 1;

  return (
    <KioskShell>
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-8 px-6 py-8 tablet:px-10">
        <div className="flex flex-col gap-4 tablet:flex-row tablet:items-center tablet:justify-between">
          <StepProgress current={step as 1 | 2 | 3} />
          <p className="text-sm text-ink-muted">
            {t('checkin.summary.welcome', { name: `${reservation.firstName} ${reservation.lastName}` })}
            {' · '}
            {new Date(reservation.arrival).toLocaleDateString(i18n.resolvedLanguage)}
            {' — '}
            {new Date(reservation.departure).toLocaleDateString(i18n.resolvedLanguage)}
            {reservation.roomName ? ` · ${t('checkin.summary.room', { room: reservation.roomName })}` : ''}
          </p>
        </div>
        {loading && !payload ? (
          <div className="kiosk-card flex h-64 items-center justify-center text-ink-muted">…</div>
        ) : (
          <Outlet />
        )}
      </div>
    </KioskShell>
  );
};
