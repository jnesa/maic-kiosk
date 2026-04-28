import { useProperty } from '@/store/property';

interface Props {
  /** "dark" places the mark on a dark hero (welcome / done); "light" sits on cream surface. */
  variant?: 'light' | 'dark';
}

/**
 * Property wordmark. Reads the active property from the store (populated
 * by PropertyProvider on boot) so a single SPA build can serve any number
 * of properties — no build-time theme constant needed.
 *
 * Falls back gracefully:
 *   - no logo URL configured → text-only wordmark in the theme's display font
 *   - no property loaded yet → empty (the PropertyProvider gates the SPA on this)
 */
export const Wordmark = ({ variant = 'light' }: Props) => {
  const property = useProperty((s) => s.property);
  if (!property) return null;

  const inkClass = variant === 'dark' ? 'text-surface' : 'text-ink';
  const subClass = variant === 'dark' ? 'text-surface/70' : 'text-ink-muted';

  return (
    <div className="flex items-center gap-3">
      {property.logo && (
        <img
          src={property.logo}
          alt=""
          aria-hidden
          className="h-8 w-auto"
          onError={(e) => {
            (e.currentTarget as HTMLImageElement).style.display = 'none';
          }}
        />
      )}
      <span className={`display text-xl font-semibold tracking-tight ${inkClass}`}>
        {property.name}
        {property.subtitle && (
          <span className={`ml-2 text-sm font-normal uppercase tracking-[0.25em] ${subClass}`}>
            {property.subtitle}
          </span>
        )}
      </span>
    </div>
  );
};
