import { useTranslation } from 'react-i18next';
import type { PublicProperty } from '@/api/property';
import type { ReservationSummary } from '@/api/types';
import { countNights, formatConfirmationParts } from '@/lib/confirmationDate';

type Props = {
  reservation: ReservationSummary;
  property: PublicProperty | null;
};

/**
 * PDF-style stay summary: dark “welcome” band, 3-column dates + PMS booking code,
 * check-in/out row, inclusions grid (static labels from i18n + live counts).
 * Data is limited to what `POST /api/kiosk/lookup` + `/form` expose — no balance/taxes on kiosk API.
 */
export const BookingConfirmationCard = ({ reservation, property }: Props) => {
  const { t, i18n } = useTranslation();
  const loc = i18n.resolvedLanguage || 'en';
  const inD = formatConfirmationParts(reservation.arrival, loc);
  const outD = formatConfirmationParts(reservation.departure, loc);
  const nights = countNights(reservation.arrival, reservation.departure);
  const roomLabel = reservation.roomName?.trim() || t('confirmation.roomTbd');
  const propName = (property?.name || property?.subtitle || 'Hotel').trim();

  return (
    <div className="w-full max-w-3xl text-ink">
      <div className="space-y-1 border-b border-ink/10 pb-5">
        <p className="text-[13px] font-bold uppercase leading-snug tracking-[0.08em]">
          {reservation.firstName},
        </p>
        <p className="text-[13px] font-bold uppercase leading-snug tracking-[0.08em]">
          {t('confirmation.line1')}
        </p>
        <p className="text-[13px] font-bold uppercase leading-snug tracking-[0.08em]">
          {t('confirmation.line2')}
        </p>
      </div>

      <div className="mt-4 bg-[#0d0d0d] px-4 py-4 text-surface sm:px-5">
        <p className="text-[10px] font-medium uppercase tracking-[0.22em] text-surface/55">{t('confirmation.welcomeTo')}</p>
        <p className="mt-1.5 text-xl font-bold uppercase leading-tight tracking-tight text-surface">
          {propName}
        </p>
      </div>

      <div className="grid grid-cols-1 border border-ink/10 bg-[#0d0d0d] text-surface min-[500px]:grid-cols-3 min-[500px]:divide-x min-[500px]:divide-white/10">
        <div className="p-4 text-left min-[500px]:text-left">
          <p className="text-[11px] text-surface/70">{inD.weekday}</p>
          <p className="mt-1.5 text-[15px] font-extrabold uppercase leading-tight tracking-[0.06em]">{inD.dateBlock}</p>
        </div>
        <div className="p-4 text-center min-[500px]:border-t-0 border-t border-white/10 min-[500px]:border-t-0">
          <p className="text-[9px] font-medium uppercase tracking-[0.2em] text-surface/50">{t('confirmation.reservationNo')}</p>
          <p className="mt-2 text-lg font-extrabold leading-none tracking-tight sm:text-xl">{reservation.code}</p>
        </div>
        <div className="p-4 text-right min-[500px]:text-right">
          <p className="text-[11px] text-surface/70">{outD.weekday}</p>
          <p className="mt-1.5 text-[15px] font-extrabold uppercase leading-tight tracking-[0.06em]">{outD.dateBlock}</p>
        </div>
      </div>

      <div className="grid grid-cols-2 bg-[#0d0d0d] px-2 py-2.5 text-[11px] font-bold uppercase tracking-[0.12em] text-surface/90">
        <div className="pl-2 text-left">
          {t('confirmation.checkin')} {t('confirmation.timeIn')}
        </div>
        <div className="pr-2 text-right">
          {t('confirmation.checkout')} {t('confirmation.timeOut')}
        </div>
      </div>

      <div className="mt-4 border border-ink/10">
        <div className="grid grid-cols-2 border-b border-ink/10 text-[10px] font-bold uppercase tracking-[0.12em]">
          <div className="border-r border-ink/10 p-3">{t('confirmation.inclusion1')}</div>
          <div className="p-3">{t('confirmation.inclusion2')}</div>
        </div>
        <div className="grid grid-cols-2 border-b border-ink/10 text-sm">
          <div className="border-r border-ink/10 p-3">
            {nights === 1 ? t('confirmation.nightsOne') : t('confirmation.nightsMany', { count: nights })}
          </div>
          <div className="p-3 font-bold uppercase text-ink">{roomLabel}</div>
        </div>
        <div className="grid grid-cols-2 text-sm">
          <div className="border-r border-ink/10 p-3 font-medium">
            {t('confirmation.adultsLine', { count: reservation.adults })}
            {reservation.children > 0
              ? ` · ${t('confirmation.childrenLine', { count: reservation.children })}`
              : ''}
          </div>
          <div className="p-3 text-right text-[9px] font-bold uppercase tracking-[0.2em] text-ink/45">{t('confirmation.included')}</div>
        </div>
      </div>

      {(property?.supportPhone || property?.supportEmail) && (
        <p className="mt-4 text-center text-[10px] uppercase leading-relaxed tracking-[0.08em] text-ink/45">
          {property?.supportPhone ? <span>{property.supportPhone}</span> : null}
          {property?.supportPhone && property?.supportEmail ? ' · ' : null}
          {property?.supportEmail ? <span>{property.supportEmail}</span> : null}
        </p>
      )}

      <p className="mt-2 text-center text-xs text-ink/40">{t('confirmation.kioskNote')}</p>
    </div>
  );
};
