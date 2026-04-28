import { apiClient } from './client';

/**
 * PublicProperty mirrors the backend's PublicConfig — only the fields safe
 * to expose to the browser. No device_key, no PMSApi URL.
 */
export interface PublicProperty {
  slug: string;
  name: string;
  subtitle?: string;
  theme: string;
  languages?: string[];
  heroImage?: string;
  logo?: string;
  supportPhone?: string;
  supportEmail?: string;
}

export const fetchProperty = (): Promise<PublicProperty> =>
  apiClient.get<PublicProperty>('/config').then((r) => r.data);
