import { apiClient } from './client';
import type { FormPayload, GuestData, FirmData } from './types';

// All kiosk endpoints take an explicit reservationId in the body — no session
// or token required. Tenant pinning is enforced on the server via env
// KIOSK_GROUP_ID + the room.id_group on the reservation.
export const fetchForm = (reservationId: number) =>
  apiClient.post<FormPayload>('/form', { reservationId }).then((r) => r.data);

export const saveGuest = (reservationId: number, guestIndex: number, guest: GuestData) =>
  apiClient
    .post<{ success: boolean; guestId: number }>('/save-guest', {
      reservationId,
      guestIndex,
      guest,
    })
    .then((r) => r.data);

export const saveFirm = (reservationId: number, firm: FirmData) =>
  apiClient
    .post<{ success: boolean }>('/save-firm', { reservationId, firm })
    .then((r) => r.data);

export const submitCheckin = (
  reservationId: number,
  lookupMethod: string,
  language: string,
  deviceId?: string,
) =>
  apiClient
    .post<{ success: boolean; reservationId: number; checkedInAt: string }>('/submit', {
      reservationId,
      lookupMethod,
      language,
      deviceId,
    })
    .then((r) => r.data);
