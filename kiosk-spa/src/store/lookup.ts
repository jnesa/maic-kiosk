import { create } from 'zustand';
import type { Candidate } from '@/api/types';

// Lives only across the lookup chain (Stage A → Stage B). Holds the candidate
// token returned by the server so the user's tap on Stage B can be redeemed.
interface LookupState {
  candidateToken: string;
  candidates: Candidate[];
  lastName: string;
  setCandidate: (token: string, list: Candidate[], lastName: string) => void;
  clear: () => void;
}

export const useLookup = create<LookupState>((set) => ({
  candidateToken: '',
  candidates: [],
  lastName: '',
  setCandidate: (token, list, lastName) =>
    set({ candidateToken: token, candidates: list, lastName }),
  clear: () => set({ candidateToken: '', candidates: [], lastName: '' }),
}));
