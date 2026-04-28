import { useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { motion } from 'motion/react';
import { useSession } from '@/store/session';
import { useForm } from '@/store/form';
import { useProperty } from '@/store/property';

export const DonePage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const sessionReset = useSession((s) => s.reset);
  const formReset = useForm((s) => s.reset);
  const property = useProperty((s) => s.property);

  useEffect(() => {
    const id = setTimeout(() => {
      sessionReset();
      formReset();
      navigate('/', { replace: true });
    }, 6000);
    return () => clearTimeout(id);
  }, [navigate, sessionReset, formReset]);

  return (
    <div className="hero-overlay grain-overlay relative flex min-h-screen flex-col bg-ink text-surface">
      {property?.heroImage && (
        <div
          className="absolute inset-0 bg-cover bg-center opacity-50"
          style={{ backgroundImage: `url(${property.heroImage})` }}
          aria-hidden
        />
      )}
      <div className="relative flex flex-1 flex-col items-start justify-center px-8 tablet:px-16 desk:px-24">
        <motion.div
          initial={{ opacity: 0, y: 18 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
          className="max-w-3xl"
        >
          <p className="text-sm font-medium uppercase tracking-[0.3em] text-accent">✓</p>
          <h1 className="display mt-6 text-6xl font-semibold leading-[0.95] tracking-tight text-surface tablet:text-7xl desk:text-[120px]">
            {t('done.title')}
          </h1>
          <p className="mt-6 max-w-xl text-lg leading-relaxed text-surface/85 tablet:text-xl">
            {t('done.subtitle')}
          </p>
          <Link to="/" className="kiosk-btn-secondary mt-12">
            {t('done.return')}
          </Link>
        </motion.div>
      </div>
    </div>
  );
};
