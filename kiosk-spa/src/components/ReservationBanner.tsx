import { useTranslation } from 'react-i18next';
import { Calendar, Home01, User01 } from '@untitledui/icons';
import type { ReservationSummary } from '@/api/types';

interface ReservationBannerProps {
  reservation: ReservationSummary;
}

/**
 * Sticky banner that pins the active reservation to the top of every check-in
 * step. Replaces the weak inline summary that used to sit next to the step
 * progress in CheckinLayout.
 *
 * Visual: 4px brand stripe across the top, then a 3-column grid (mobile-first
 * single column, expanding on tablet+):
 *   - Left:   guest name + adults + room
 *   - Center: arrival → departure with night count
 *   - Right:  reservation code chip in brand-soft pill
 *
 * Sized to be unmistakable on the kiosk display without taking the entire
 * fold — body height ~ 96 px on tablet.
 */
export const ReservationBanner = ({ reservation }: ReservationBannerProps) => {
  const { t, i18n } = useTranslation();
  const lang = i18n.resolvedLanguage ?? 'en';

  const arrival = parseISODate(reservation.arrival);
  const departure = parseISODate(reservation.departure);
  const nights = arrival && departure ? Math.round((+departure - +arrival) / 86_400_000) : 0;

  const fmt = new Intl.DateTimeFormat(lang, { day: '2-digit', month: 'short' });
  const arrivalLabel = arrival ? fmt.format(arrival) : reservation.arrival;
  const departureLabel = departure ? fmt.format(departure) : reservation.departure;

  const fullName = [reservation.firstName, reservation.lastName].filter(Boolean).join(' ').trim();

  return (
    <div className="sticky top-0 z-20 border-b border-border-subtle bg-bg/90 backdrop-blur-md">
      <div aria-hidden="true" className="h-1 w-full bg-brand" />
      <div className="mx-auto grid w-full max-w-5xl grid-cols-1 items-center gap-4 px-6 py-4 tablet:grid-cols-[1.4fr_1fr_auto] tablet:gap-6 tablet:px-10">
        {/* Left: guest name + meta */}
        <div className="min-w-0">
          <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">
            {t('checkin.summary.eyebrow', 'Reservation')}
          </p>
          <h1 className="display mt-1 truncate text-2xl font-semibold text-ink tablet:text-3xl">
            {fullName || '—'}
          </h1>
          <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-ink-muted">
            <span className="inline-flex items-center gap-1.5">
              <User01 className="h-4 w-4 shrink-0" />
              {t(reservation.adults === 1 ? 'checkin.summary.adults_one' : 'checkin.summary.adults_other', { count: reservation.adults })}
            </span>
            {reservation.roomName && (
              <span className="inline-flex items-center gap-1.5 truncate">
                <Home01 className="h-4 w-4 shrink-0" />
                <span className="truncate">{reservation.roomName}</span>
              </span>
            )}
          </div>
        </div>

        {/* Center: dates */}
        <div className="min-w-0 tablet:justify-self-center">
          <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">
            <span className="inline-flex items-center gap-1.5">
              <Calendar className="h-4 w-4" />
              {t('checkin.summary.stay_label', 'Stay')}
            </span>
          </p>
          <p className="display mt-1 text-xl font-semibold text-ink tablet:text-2xl">
            <span>{arrivalLabel}</span>
            <span aria-hidden="true" className="mx-2 text-brand">→</span>
            <span>{departureLabel}</span>
          </p>
          <p className="mt-1 text-sm text-ink-muted">
            {t('checkin.summary.nights', { count: nights, defaultValue: '{{count}} nights' })}
          </p>
        </div>

        {/* Right: code chip */}
        {reservation.code && (
          <div className="tablet:justify-self-end">
            <span className="inline-flex items-center gap-1.5 rounded-full bg-brand-soft px-3 py-1.5 font-mono text-xs font-semibold uppercase tracking-wider text-brand">
              {reservation.code}
            </span>
          </div>
        )}
      </div>
    </div>
  );
};

// Local helper — accepts both `YYYY-MM-DD` and full ISO. Returns `null` on
// failure so the banner can fall back to the raw string instead of throwing.
function parseISODate(s: string): Date | null {
  if (!s) return null;
  const ymd = s.length >= 10 ? s.slice(0, 10) : s;
  const d = new Date(ymd + 'T00:00:00');
  return Number.isNaN(d.getTime()) ? null : d;
}
