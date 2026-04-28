import { create } from 'zustand';
import type { AdminUser } from '@/api/types';

/**
 * Currently logged-in admin. Populated by RootGate on boot via /me;
 * cleared on logout. Components shouldn't talk directly to /me — they
 * should read from this store.
 */
interface AuthState {
  user: AdminUser | null;
  status: 'loading' | 'authed' | 'anonymous';
  setUser: (u: AdminUser) => void;
  setAnonymous: () => void;
}

export const useAuth = create<AuthState>((set) => ({
  user: null,
  status: 'loading',
  setUser: (u) => set({ user: u, status: 'authed' }),
  setAnonymous: () => set({ user: null, status: 'anonymous' }),
}));
