/**
 * Date formatting aligned with hotel PDF confirmations (e.g. MOOV):
 * lowercase weekday, uppercase "1 APRIL 2026" style line.
 */
export function formatConfirmationParts(isoDate: string, locale: string): { weekday: string; dateBlock: string } {
  const normalized = /^\d{4}-\d{2}-\d{2}$/.test(isoDate) ? `${isoDate}T12:00:00` : isoDate;
  const d = new Date(normalized);
  if (Number.isNaN(d.getTime())) {
    return { weekday: '', dateBlock: '' };
  }
  const weekday = d.toLocaleDateString(locale, { weekday: 'long' }).toLowerCase();
  const dayNum = d.getDate();
  const month = d.toLocaleDateString(locale, { month: 'long' });
  const year = d.getFullYear();
  const dateBlock = `${dayNum} ${month} ${year}`.toUpperCase();
  return { weekday, dateBlock };
}

export function countNights(arrival: string, departure: string): number {
  const a = new Date(/^\d{4}-\d{2}-\d{2}$/.test(arrival) ? `${arrival}T12:00:00` : arrival);
  const b = new Date(/^\d{4}-\d{2}-\d{2}$/.test(departure) ? `${departure}T12:00:00` : departure);
  if (Number.isNaN(a.getTime()) || Number.isNaN(b.getTime())) return 1;
  const diff = Math.round((b.getTime() - a.getTime()) / 86400000);
  return Math.max(1, diff);
}
