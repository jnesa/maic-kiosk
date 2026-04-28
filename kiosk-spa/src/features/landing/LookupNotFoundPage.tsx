import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { KioskShell } from '@/components/KioskShell';

export const LookupNotFoundPage = () => {
  const { t } = useTranslation();
  return (
    <KioskShell>
      <div className="mx-auto flex w-full max-w-2xl flex-col items-start justify-center px-6 py-10 tablet:px-10">
        <h1 className="display text-5xl font-semibold tracking-tight text-ink tablet:text-6xl">
          {t('lookup.notFound.title')}
        </h1>
        <p className="mt-4 max-w-xl text-lg text-ink-muted">{t('lookup.notFound.body')}</p>
        <div className="mt-10 flex flex-col gap-4 tablet:flex-row">
          <Link to="/lookup/last-name" className="kiosk-btn-primary">
            {t('lookup.notFound.retry')}
          </Link>
          <Link to="/lookup/booking" className="kiosk-btn-secondary">
            {t('lookup.haveBookingNumber')}
          </Link>
        </div>
      </div>
    </KioskShell>
  );
};
