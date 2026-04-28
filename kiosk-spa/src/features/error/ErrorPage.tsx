import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { KioskShell } from '@/components/KioskShell';

export const ErrorPage = () => {
  const { t } = useTranslation();
  return (
    <KioskShell>
      <div className="mx-auto flex w-full max-w-2xl flex-col items-start justify-center px-6 py-10 tablet:px-10">
        <h1 className="display text-5xl font-semibold tracking-tight text-ink tablet:text-6xl">
          {t('error.title')}
        </h1>
        <p className="mt-4 text-lg text-ink-muted">{t('error.body')}</p>
        <Link to="/" className="kiosk-btn-primary mt-10">
          {t('error.retry')}
        </Link>
      </div>
    </KioskShell>
  );
};
