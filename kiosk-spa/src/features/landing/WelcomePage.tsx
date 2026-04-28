import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { motion } from 'motion/react';
import { LanguageSwitcher } from '@/components/LanguageSwitcher';
import { Wordmark } from '@/components/Wordmark';
import { useProperty } from '@/store/property';

// Welcome screen — the kiosk's billboard. Full-bleed hero photography from
// the active theme, brand-tinted gradient overlay, single large CTA. Idle.
export const WelcomePage = () => {
  const { t } = useTranslation();
  const property = useProperty((s) => s.property);
  return (
    <div className="hero-overlay grain-overlay relative min-h-screen overflow-hidden bg-ink text-surface">
      {property?.heroImage && (
        <div
          className="absolute inset-0 bg-cover bg-center"
          style={{ backgroundImage: `url(${property.heroImage})` }}
          aria-hidden
        />
      )}
      <div className="relative flex min-h-screen flex-col">
        <header className="flex items-center justify-between px-8 py-6">
          <Wordmark variant="dark" />
          <LanguageSwitcher />
        </header>
        <div className="flex flex-1 items-center px-8 tablet:px-16 desk:px-24">
          <motion.div
            initial={{ opacity: 0, y: 18 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.7, ease: [0.22, 1, 0.36, 1] }}
            className="max-w-3xl"
          >
            <p className="text-sm font-medium uppercase tracking-[0.3em] text-surface/80">
              {t('welcome.eyebrow', { property: property?.name ?? '' })}
            </p>
            <h1 className="display mt-6 text-6xl font-semibold leading-[0.95] tracking-tight text-surface tablet:text-7xl desk:text-[120px]">
              {t('welcome.title')}
            </h1>
            <p className="mt-6 max-w-xl text-lg leading-relaxed text-surface/85 tablet:text-xl">
              {t('welcome.subtitle')}
            </p>
            <Link to="/lookup/last-name" className="kiosk-btn-primary mt-12 text-2xl">
              {t('welcome.cta')}
              <span aria-hidden>→</span>
            </Link>
          </motion.div>
        </div>
        <footer className="px-8 pb-8 text-sm text-surface/60 tablet:px-16">
          {new Date().getFullYear()}
        </footer>
      </div>
    </div>
  );
};
