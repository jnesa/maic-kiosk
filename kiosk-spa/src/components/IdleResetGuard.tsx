import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useSession } from '@/store/session';
import { useForm } from '@/store/form';

const IDLE_MS = 90_000;
const WARN_MS = 10_000;

// Resets the kiosk to the welcome screen after 90s of no activity. Shows a
// last-second countdown banner so a guest who's just thinking has the chance
// to dismiss the reset.
export const IdleResetGuard = () => {
  const navigate = useNavigate();
  const sessionReset = useSession((s) => s.reset);
  const formReset = useForm((s) => s.reset);
  const { t } = useTranslation();
  const [warnRemaining, setWarnRemaining] = useState<number | null>(null);

  useEffect(() => {
    let idle: ReturnType<typeof setTimeout>;
    let warn: ReturnType<typeof setTimeout>;
    let tick: ReturnType<typeof setInterval>;
    const reset = () => {
      sessionReset();
      formReset();
      navigate('/', { replace: true });
    };
    const arm = () => {
      clearTimeout(idle);
      clearTimeout(warn);
      clearInterval(tick);
      setWarnRemaining(null);
      warn = setTimeout(() => {
        let s = WARN_MS / 1000;
        setWarnRemaining(s);
        tick = setInterval(() => {
          s -= 1;
          setWarnRemaining(s);
          if (s <= 0) clearInterval(tick);
        }, 1000);
      }, IDLE_MS - WARN_MS);
      idle = setTimeout(reset, IDLE_MS);
    };

    const events: (keyof DocumentEventMap)[] = ['pointerdown', 'keydown', 'touchstart'];
    events.forEach((e) => document.addEventListener(e, arm, { passive: true }));
    arm();
    return () => {
      events.forEach((e) => document.removeEventListener(e, arm));
      clearTimeout(idle);
      clearTimeout(warn);
      clearInterval(tick);
    };
  }, [navigate, sessionReset, formReset]);

  if (warnRemaining === null) return null;
  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-6 flex justify-center">
      <div className="pointer-events-auto rounded-full bg-ink/90 px-6 py-3 text-sm font-medium text-surface shadow-xl backdrop-blur">
        {t('common.idleSoon', { seconds: Math.max(0, warnRemaining) })}
      </div>
    </div>
  );
};
