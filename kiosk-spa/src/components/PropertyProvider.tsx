import { ReactNode, useEffect } from 'react';
import { useProperty } from '@/store/property';
import { fetchProperty } from '@/api/property';
import { propertySlug } from '@/api/client';
import { applyTheme, resolveTheme } from '@/theme';

interface Props {
  children: ReactNode;
}

/**
 * Boots the kiosk by fetching the active property's public config and
 * applying its theme. Renders a tiny loading shell until /config resolves
 * so we never flash a default-themed screen at the guest.
 *
 * The slug comes from the URL path (see api/client.ts). If it's missing
 * or the backend doesn't recognise it, we render an error screen pointing
 * to /properties so an operator can pick the right one.
 */
export const PropertyProvider = ({ children }: Props) => {
  const { property, status, error, setProperty, setError, setLoading } = useProperty();

  useEffect(() => {
    if (!propertySlug()) {
      setError('no_slug');
      return;
    }
    setLoading();
    fetchProperty()
      .then((p) => {
        setProperty(p);
        applyTheme(resolveTheme(p.theme));
      })
      .catch(() => setError('config_failed'));
  }, [setProperty, setError, setLoading]);

  if (status === 'loading' || status === 'idle') {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg text-ink-muted">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-current border-t-transparent" />
      </div>
    );
  }
  if (status === 'error' || !property) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-bg p-8 text-center">
        <h1 className="text-2xl font-semibold text-ink">
          {error === 'no_slug' ? 'No property selected' : 'Property unavailable'}
        </h1>
        <p className="max-w-md text-ink-muted">
          {error === 'no_slug'
            ? 'This kiosk URL is missing a property identifier. Add the property slug as the first path segment, e.g. /smart-moov.'
            : 'We could not load the configuration for this kiosk. Please contact reception so we can help in person.'}
        </p>
      </div>
    );
  }
  return <>{children}</>;
};
