import { create } from 'zustand';
import type { FormPayload, GuestData, FirmData, FieldConfig, PrestayConfig } from '@/api/types';
import { emptyFirm, emptyGuest } from '@/api/types';

// Holds the in-progress check-in form data through steps 1-3. We DO NOT persist
// this — kiosk sessions are short-lived, and partial state in memory is enough.
interface FormState {
  payload: FormPayload | null;
  guests: GuestData[];
  firm: FirmData;
  setPayload: (p: FormPayload) => void;
  patchGuest: (idx: number, patch: Partial<GuestData>) => void;
  setGuestId: (idx: number, id: number) => void;
  ensureGuestSlots: (n: number) => void;
  patchFirm: (patch: Partial<FirmData>) => void;
  reset: () => void;
}

export const useForm = create<FormState>((set) => ({
  payload: null,
  guests: [],
  firm: emptyFirm(),
  setPayload: (p) =>
    set(() => ({
      payload: p,
      guests:
        p.guests.length > 0
          ? p.guests
          : Array.from({ length: Math.max(1, p.reservation?.adults ?? 1) }, () => emptyGuest()),
      firm: p.firm ?? emptyFirm(),
    })),
  patchGuest: (idx, patch) =>
    set((s) => {
      const next = s.guests.slice();
      next[idx] = { ...next[idx], ...patch };
      return { guests: next };
    }),
  setGuestId: (idx, id) =>
    set((s) => {
      const next = s.guests.slice();
      if (next[idx]) next[idx] = { ...next[idx], id };
      return { guests: next };
    }),
  ensureGuestSlots: (n) =>
    set((s) => {
      if (s.guests.length >= n) return {};
      const next = s.guests.slice();
      while (next.length < n) next.push(emptyGuest());
      return { guests: next };
    }),
  patchFirm: (patch) => set((s) => ({ firm: { ...s.firm, ...patch } })),
  reset: () => set({ payload: null, guests: [], firm: emptyFirm() }),
}));

// Helper: typed FieldConfig lookup that ignores top-level booleans like edit/useMRZ.
export const fc = (cfg: PrestayConfig | undefined, key: string): FieldConfig | undefined => {
  const v = cfg?.[key];
  return v && typeof v === 'object' ? (v as FieldConfig) : undefined;
};

export const isVisible = (cfg: PrestayConfig | undefined, key: string) => {
  const f = fc(cfg, key);
  return Boolean(f?.use);
};

export const isRequired = (cfg: PrestayConfig | undefined, key: string, isPrimary: boolean) => {
  const f = fc(cfg, key);
  if (!f?.use) return false;
  return f.required === 1 || (f.required === 2 && isPrimary);
};
