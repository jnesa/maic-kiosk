import { create } from 'zustand';
import type { ReservationSummary } from '@/api/types';

/*
 * Session = the active reservation the kiosk is checking in. Mirrored to
 * sessionStorage so an accidental tab refresh on the kiosk doesn't kick the
 * guest. Idle reset (90 s) clears it.
 */
const STORAGE_KEY = 'kiosk.session.v2';

interface Persisted {
  reservation: ReservationSummary | null;
  lookupMethod: string;
}

const load = (): Persisted | null => {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    return raw ? (JSON.parse(raw) as Persisted) : null;
  } catch {
    return null;
  }
};

const save = (p: Persisted | null) => {
  try {
    if (!p) sessionStorage.removeItem(STORAGE_KEY);
    else sessionStorage.setItem(STORAGE_KEY, JSON.stringify(p));
  } catch {
    /* sessionStorage disabled — keep state in memory only */
  }
};

interface SessionState {
  reservation: ReservationSummary | null;
  lookupMethod: string;
  /** True once /lookup has resolved a single reservation and the user is in /checkin/*. */
  hasReservation: () => boolean;
  start: (reservation: ReservationSummary, lookupMethod: string) => void;
  reset: () => void;
}

const initial = load();

export const useSession = create<SessionState>((set, get) => ({
  reservation: initial?.reservation ?? null,
  lookupMethod: initial?.lookupMethod ?? '',
  hasReservation: () => Boolean(get().reservation),
  start: (reservation, lookupMethod) => {
    save({ reservation, lookupMethod });
    set({ reservation, lookupMethod });
  },
  reset: () => {
    save(null);
    set({ reservation: null, lookupMethod: '' });
  },
}));
