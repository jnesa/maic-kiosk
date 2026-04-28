/**
 * Wire shapes shared with the Go admin handler. Keep in sync with
 * internal/admin/http.go.
 */

export interface AdminUser {
  id: number;
  email: string;
  name: string;
  status: 'active' | 'disabled';
  created_at: string;
  last_login_at: string | null;
}

export interface Hotel {
  id: number;
  name: string;
  pmsapi_url: string;
  notes: string | null;
  created_at: string;
  updated_at: string;
  kiosk_count: number;
  kiosks?: Kiosk[];
}

export interface HotelInput {
  name: string;
  pmsapi_url: string;
  notes?: string;
}

export interface Kiosk {
  id: string;
  hotel_id: number;
  display_name: string;
  legacy_group_id: number | null;
  legacy_group_label: string | null;
  theme: string;
  languages: string[];
  device_key: string;
  hero_image: string | null;
  logo: string | null;
  support_phone: string | null;
  support_email: string | null;
  status: 'active' | 'disabled';
  created_at: string;
  updated_at: string;
}

export interface KioskInput {
  display_name: string;
  legacy_group_id?: number | null;
  legacy_group_label?: string;
  theme: string;
  languages: string[];
  hero_image?: string;
  logo?: string;
  support_phone?: string;
  support_email?: string;
}

export interface AuditEntry {
  ID: number;
  UserID: number | null;
  ActorEmail: string | null;
  Action: string;
  EntityType: string;
  EntityID: string;
  Payload: Record<string, unknown> | null;
  CreatedAt: string;
}
