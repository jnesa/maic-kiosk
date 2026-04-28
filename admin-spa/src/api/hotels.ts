import { api } from './client';
import type { Hotel, HotelInput, Kiosk, KioskInput } from './types';

export const listHotels = () =>
  api.get<{ hotels: Hotel[] }>('/hotels').then((r) => r.hotels);

export const createHotel = (input: HotelInput) => api.post<Hotel>('/hotels', input);

export const getHotel = (id: number) => api.get<Hotel>(`/hotels/${id}`);

export const updateHotel = (id: number, patch: Partial<HotelInput>) =>
  api.patch<Hotel>(`/hotels/${id}`, patch);

export const deleteHotel = (id: number) => api.delete<void>(`/hotels/${id}`);

export const createKiosk = (hotelID: number, input: KioskInput) =>
  api.post<Kiosk>(`/hotels/${hotelID}/kiosks`, input);

export const getKiosk = (id: string) => api.get<Kiosk>(`/kiosks/${id}`);

export const updateKiosk = (id: string, patch: Partial<KioskInput>) =>
  api.patch<Kiosk>(`/kiosks/${id}`, patch);

export const rotateKioskKey = (id: string) =>
  api.post<{ id: string; device_key: string }>(`/kiosks/${id}/rotate-key`);

export const disableKiosk = (id: string) => api.post<Kiosk>(`/kiosks/${id}/disable`);
export const enableKiosk = (id: string) => api.post<Kiosk>(`/kiosks/${id}/enable`);
export const deleteKiosk = (id: string) => api.delete<void>(`/kiosks/${id}`);
