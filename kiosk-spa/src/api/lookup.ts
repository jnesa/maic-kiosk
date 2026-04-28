import { apiClient } from './client';
import type { LookupResponse } from './types';

export type LookupRequest =
  | { lastName: string }
  | { lastName: string; arrivalDate: string }
  | { reservationId: string };

export const lookupReservation = (req: LookupRequest) =>
  apiClient.post<LookupResponse>('/lookup', req).then((r) => r.data);

export const selectCandidate = (candidateToken: string, candidateId: string) =>
  apiClient
    .post<LookupResponse>('/select', { candidateToken, candidateId })
    .then((r) => r.data);
