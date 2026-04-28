import { useTranslation } from 'react-i18next';
import { cn } from '@/utils/cn';

const langs = [
  { code: 'en', label: 'EN' },
  { code: 'de', label: 'DE' },
  { code: 'it', label: 'IT' },
  { code: 'fr', label: 'FR' },
];

export const LanguageSwitcher = () => {
  const { i18n } = useTranslation();
  const current = i18n.resolvedLanguage ?? 'en';
  return (
    <div className="inline-flex rounded-full border border-border-subtle bg-surface p-1 shadow-sm">
      {langs.map((l) => (
        <button
          key={l.code}
          type="button"
          onClick={() => void i18n.changeLanguage(l.code)}
          className={cn(
            'min-h-[44px] rounded-full px-4 text-sm font-semibold tracking-wide transition-colors',
            current === l.code ? 'bg-brand text-white' : 'text-ink-muted hover:bg-surface-muted',
          )}
          aria-pressed={current === l.code}
        >
          {l.label}
        </button>
      ))}
    </div>
  );
};
