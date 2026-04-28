import { ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { LanguageSwitcher } from './LanguageSwitcher';
import { Wordmark } from './Wordmark';
import { useSession } from '@/store/session';
import { useForm } from '@/store/form';

interface Props {
  children: ReactNode;
  /** Welcome screen renders the hero photo full-bleed; everything else uses the calm cream layout. */
  variant?: 'hero' | 'plain';
}

export const KioskShell = ({ children, variant = 'plain' }: Props) => {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const sessionReset = useSession((s) => s.reset);
  const formReset = useForm((s) => s.reset);
  const onStartOver = () => {
    sessionReset();
    formReset();
    navigate('/', { replace: true });
  };

  return (
    <div className="relative flex min-h-screen flex-col bg-bg">
      <header className="z-10 flex items-center justify-between px-6 py-5 tablet:px-10">
        <Wordmark variant="light" />
        <div className="flex items-center gap-3">
          <LanguageSwitcher />
          {variant === 'plain' && (
            <button type="button" onClick={onStartOver} className="kiosk-btn-tertiary">
              {t('common.startOver')}
            </button>
          )}
        </div>
      </header>
      <main className="flex flex-1">{children}</main>
    </div>
  );
};
