/** Formats an ISO timestamp for the audit table / "last login" labels. */
export const fmtDateTime = (iso: string | null | undefined) => {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
};

/** Builds the public kiosk URL by combining the current origin with the kiosk id. */
export const kioskUrl = (id: string) => `${window.location.origin}/${id}`;
