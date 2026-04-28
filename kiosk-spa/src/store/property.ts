import { create } from 'zustand';
import type { PublicProperty } from '@/api/property';

/**
 * Active property state.
 *
 * Loaded once at boot from /api/kiosk/v1/<slug>/config. Powers the theme,
 * the wordmark in the header, and the language list. Components that
 * render before this resolves should defer to the loading screen.
 */
interface PropertyState {
  property: PublicProperty | null;
  status: 'idle' | 'loading' | 'ready' | 'error';
  error: string | null;
  setProperty: (p: PublicProperty) => void;
  setError: (msg: string) => void;
  setLoading: () => void;
}

export const useProperty = create<PropertyState>((set) => ({
  property: null,
  status: 'idle',
  error: null,
  setProperty: (p) => set({ property: p, status: 'ready', error: null }),
  setError: (msg) => set({ status: 'error', error: msg }),
  setLoading: () => set({ status: 'loading', error: null }),
}));
