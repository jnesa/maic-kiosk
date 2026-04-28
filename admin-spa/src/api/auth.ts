import { api } from './client';
import type { AdminUser } from './types';

export const login = (email: string, password: string) =>
  api.post<{ user: AdminUser }>('/auth/login', { email, password });

export const logout = () => api.post<void>('/auth/logout');

export const me = () => api.get<AdminUser>('/me');
