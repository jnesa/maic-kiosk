import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Camera01 } from '@untitledui/icons';
import { Field } from '@/components/Field';
import { MrzScanner } from '@/components/MrzScanner';
import { ExternalScannerModal } from '@/components/ExternalScannerModal';
import { useForm, isVisible, isRequired } from '@/store/form';
import { useSession } from '@/store/session';
import { saveGuest } from '@/api/checkin';
import { mapMrzToGuest } from '@/lib/mrz';
import type { GuestData } from '@/api/types';

type ScannerTarget = { idx: number; mode: 'camera' | 'external' } | null;

const documentOptions = [
  { value: 1, key: 'field.document.passport' },
  { value: 2, key: 'field.document.id_card' },
  { value: 3, key: 'field.document.driver_license' },
];

// Step 1: per-guest details. We render each guest in its own card; the SPA
// validates locally then POSTs each one to /sessions/me/guest, caching the
// returned id so re-saves of the same index update the same row.
export const Step1GuestPage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const reservation = useSession((s) => s.reservation);
  const { payload, guests, patchGuest, setGuestId, ensureGuestSlots } = useForm();
  const cfg = payload?.config;
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<Record<number, Record<string, string>>>({});
  // Active scanner: which guest, and which mode (camera or desk scanner).
  // null when no modal is open.
  const [scanner, setScanner] = useState<ScannerTarget>(null);

  const adults = reservation?.adults ?? 1;
  if (guests.length < adults) ensureGuestSlots(adults);

  const validate = (g: GuestData, isPrimary: boolean) => {
    const e: Record<string, string> = {};
    const must = (key: string, val: string) => {
      if (isRequired(cfg, key, isPrimary) && !val.trim()) e[key] = 'required';
    };
    must('f_name', g.fname);
    must('l_name', g.lname);
    must('dob', g.dob);
    must('country', g.country);
    must('nationality', g.nationality);
    must('city', g.city);
    must('postal', g.postal);
    must('street', g.street);
    must('document_id', g.document_id);
    must('document_issuer', g.document_issuer);
    must('document_issue_date', g.document_issue_date);
    if (isRequired(cfg, 'document', isPrimary) && (g.document === null || g.document === undefined)) {
      e.document = 'required';
    }
    return e;
  };

  const onContinue = async () => {
    setBusy(true);
    const newErrors: Record<number, Record<string, string>> = {};
    let hasErr = false;
    for (let i = 0; i < guests.length; i++) {
      const e = validate(guests[i], i === 0);
      if (Object.keys(e).length) {
        newErrors[i] = e;
        hasErr = true;
      }
    }
    setErrors(newErrors);
    if (hasErr) {
      setBusy(false);
      return;
    }
    try {
      for (let i = 0; i < guests.length; i++) {
        if (!reservation) break;
        const r = await saveGuest(reservation.id, i, guests[i]);
        setGuestId(i, r.guestId);
      }
      navigate('/checkin/2');
    } catch {
      navigate('/error');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-6">
      {guests.map((g, i) => {
        const isPrimary = i === 0;
        const ge = errors[i] ?? {};
        return (
          <section key={i} className="kiosk-card">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h2 className="display text-2xl font-semibold text-ink">
                {isPrimary ? t('step.guest') : `${t('step.guest')} · ${i + 1}`}
              </h2>
              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={() => setScanner({ idx: i, mode: 'camera' })}
                  className="kiosk-btn-secondary inline-flex items-center gap-2"
                  aria-label={t('scanner.button')}
                >
                  <Camera01 className="h-5 w-5" />
                  {t('scanner.button')}
                </button>
                <button
                  type="button"
                  onClick={() => setScanner({ idx: i, mode: 'external' })}
                  className="kiosk-btn-secondary inline-flex items-center gap-2"
                  aria-label={t('external_scanner.button')}
                >
                  {/* Inline flatbed-scanner glyph — keeps us off the
                      icon-naming guess for @untitledui/icons. */}
                  <svg
                    aria-hidden="true"
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <rect x="3" y="6" width="18" height="12" rx="2" />
                    <path d="M3 12h18" />
                    <path d="M7 18v2M17 18v2" />
                  </svg>
                  {t('external_scanner.button')}
                </button>
              </div>
            </div>
            <div className="mt-6 grid grid-cols-1 gap-5 tablet:grid-cols-2 desk:grid-cols-3">
              {isVisible(cfg, 'f_name') && (
                <Field label={t('field.fname')} required={isRequired(cfg, 'f_name', isPrimary)} error={ge.f_name && t('common.required')}>
                  {(id) => (
                    <input id={id} className="kiosk-input" value={g.fname} onChange={(e) => patchGuest(i, { fname: e.target.value })} autoComplete="off" />
                  )}
                </Field>
              )}
              {isVisible(cfg, 'l_name') && (
                <Field label={t('field.lname')} required={isRequired(cfg, 'l_name', isPrimary)} error={ge.l_name && t('common.required')}>
                  {(id) => (
                    <input id={id} className="kiosk-input" value={g.lname} onChange={(e) => patchGuest(i, { lname: e.target.value })} autoComplete="off" />
                  )}
                </Field>
              )}
              {isVisible(cfg, 'dob') && (
                <Field label={t('field.dob')} required={isRequired(cfg, 'dob', isPrimary)} error={ge.dob && t('common.required')}>
                  {(id) => (
                    <input id={id} type="date" className="kiosk-input" value={g.dob} onChange={(e) => patchGuest(i, { dob: e.target.value })} />
                  )}
                </Field>
              )}
              {isVisible(cfg, 'country') && (
                <Field label={t('field.country')} required={isRequired(cfg, 'country', isPrimary)} error={ge.country && t('common.required')}>
                  {(id) => (
                    <input id={id} className="kiosk-input" value={g.country} onChange={(e) => patchGuest(i, { country: e.target.value.toUpperCase() })} maxLength={2} placeholder="IT" />
                  )}
                </Field>
              )}
              {isVisible(cfg, 'nationality') && (
                <Field label={t('field.nationality')} required={isRequired(cfg, 'nationality', isPrimary)} error={ge.nationality && t('common.required')}>
                  {(id) => (
                    <input id={id} className="kiosk-input" value={g.nationality} onChange={(e) => patchGuest(i, { nationality: e.target.value.toUpperCase() })} maxLength={3} placeholder="ITA" />
                  )}
                </Field>
              )}
              {isVisible(cfg, 'city') && (
                <Field label={t('field.city')} required={isRequired(cfg, 'city', isPrimary)} error={ge.city && t('common.required')}>
                  {(id) => (
                    <input id={id} className="kiosk-input" value={g.city} onChange={(e) => patchGuest(i, { city: e.target.value })} />
                  )}
                </Field>
              )}
              {isVisible(cfg, 'postal') && (
                <Field label={t('field.postal')} required={isRequired(cfg, 'postal', isPrimary)} error={ge.postal && t('common.required')}>
                  {(id) => (
                    <input id={id} className="kiosk-input" value={g.postal} onChange={(e) => patchGuest(i, { postal: e.target.value })} />
                  )}
                </Field>
              )}
              {isVisible(cfg, 'street') && (
                <Field label={t('field.street')} required={isRequired(cfg, 'street', isPrimary)} error={ge.street && t('common.required')}>
                  {(id) => (
                    <input id={id} className="kiosk-input" value={g.street} onChange={(e) => patchGuest(i, { street: e.target.value })} />
                  )}
                </Field>
              )}
              {isVisible(cfg, 'document') && (
                <Field label={t('field.document')} required={isRequired(cfg, 'document', isPrimary)} error={ge.document && t('common.required')}>
                  {(id) => (
                    <select id={id} className="kiosk-input" value={g.document ?? ''} onChange={(e) => patchGuest(i, { document: e.target.value ? Number(e.target.value) : null })}>
                      <option value="">—</option>
                      {documentOptions.map((o) => (
                        <option key={o.value} value={o.value}>{t(o.key)}</option>
                      ))}
                    </select>
                  )}
                </Field>
              )}
              {isVisible(cfg, 'document_id') && (
                <Field label={t('field.document_id')} required={isRequired(cfg, 'document_id', isPrimary)} error={ge.document_id && t('common.required')}>
                  {(id) => (
                    <input id={id} className="kiosk-input" value={g.document_id} onChange={(e) => patchGuest(i, { document_id: e.target.value })} autoComplete="off" />
                  )}
                </Field>
              )}
              {isVisible(cfg, 'document_issuer') && (
                <Field label={t('field.document_issuer')} required={isRequired(cfg, 'document_issuer', isPrimary)} error={ge.document_issuer && t('common.required')}>
                  {(id) => (
                    <input id={id} className="kiosk-input" value={g.document_issuer} onChange={(e) => patchGuest(i, { document_issuer: e.target.value })} />
                  )}
                </Field>
              )}
              {isVisible(cfg, 'document_issue_date') && (
                <Field label={t('field.document_issue_date')} required={isRequired(cfg, 'document_issue_date', isPrimary)} error={ge.document_issue_date && t('common.required')}>
                  {(id) => (
                    <input id={id} type="date" className="kiosk-input" value={g.document_issue_date} onChange={(e) => patchGuest(i, { document_issue_date: e.target.value })} />
                  )}
                </Field>
              )}
            </div>
          </section>
        );
      })}
      <div className="flex justify-end">
        <button type="button" onClick={onContinue} disabled={busy} className="kiosk-btn-primary">
          {busy ? '…' : t('common.next')}
          <span aria-hidden>→</span>
        </button>
      </div>
      <MrzScanner
        open={scanner?.mode === 'camera'}
        onClose={() => setScanner(null)}
        onParsed={(fields) => {
          if (scanner) patchGuest(scanner.idx, mapMrzToGuest(fields));
          setScanner(null);
        }}
      />
      <ExternalScannerModal
        open={scanner?.mode === 'external'}
        onClose={() => setScanner(null)}
        onParsed={(fields) => {
          if (scanner) patchGuest(scanner.idx, mapMrzToGuest(fields));
          setScanner(null);
        }}
      />
    </div>
  );
};
