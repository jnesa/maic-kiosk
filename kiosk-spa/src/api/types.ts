/*
 * Wire shapes shared with the Go backend. Ported verbatim from
 * GuestSPA/src/api/prestay.ts:4-149 so the existing legacy validators carry
 * over. Keep this file in sync with internal/domain/reservation.go.
 */

export interface FieldOption {
  value: string;
  langs: Record<string, string>;
}

export interface FieldConfig {
  use: boolean;
  /** 0 = optional, 1 = always required, 2 = required for primary guest only. */
  required: number;
  options?: Record<string, FieldOption>;
  children?: {
    open: boolean;
    opener: boolean;
    items: Record<string, FieldConfig & { required_on_parent?: boolean }>;
  };
}

export interface PrestayConfig {
  [key: string]: FieldConfig | boolean | undefined;
}

export interface GuestData {
  id: number | null;
  title: string;
  fname: string;
  lname: string;
  dob: string;
  country: string;
  city: string;
  postal: string;
  street: string;
  house_number: string;
  document: number | null;
  document_id: string;
  document_issuer: string;
  document_issue_date: string;
  nationality: string;
  phone: string;
  traveltime_changed: boolean;
  traveltime_arrival: string;
  traveltime_departure: string;
  specialtravel: boolean;
  special_travel_event_id: string;
  businesstravel: boolean;
  annualcard: boolean;
  annualcard_number: string;
  handicap: boolean;
  handicap_needhelp: boolean;
  handicap_number: string;
  handicap_is_help: boolean;
}

export const emptyGuest = (): GuestData => ({
  id: null,
  title: '',
  fname: '',
  lname: '',
  dob: '',
  country: '',
  city: '',
  postal: '',
  street: '',
  house_number: '',
  document: null,
  document_id: '',
  document_issuer: '',
  document_issue_date: '',
  nationality: '',
  phone: '',
  traveltime_changed: false,
  traveltime_arrival: '',
  traveltime_departure: '',
  specialtravel: false,
  special_travel_event_id: '',
  businesstravel: false,
  annualcard: false,
  annualcard_number: '',
  handicap: false,
  handicap_needhelp: false,
  handicap_number: '',
  handicap_is_help: false,
});

export interface FirmData {
  compname: string;
  vatid: string;
  address: string;
  city: string;
  arrival: string;
  arrival_via: string;
  arrival_with_car: boolean;
  phone: string;
  email: string;
  useFirmForBilling: boolean;
  useAnotherBillingAddress: boolean;
  billing_address: string;
  transfer: boolean;
  transferText: string;
  babyBed: boolean;
  babyBedText: string;
  dogPackage: boolean;
  dogPackageText: string;
  alergies: boolean;
  alergiesText: string;
  accessible: boolean;
  additionalLinens: boolean;
  additionalLinensAmount: string;
  preferedCommunication: string;
  signature: string;
}

export const emptyFirm = (): FirmData => ({
  compname: '',
  vatid: '',
  address: '',
  city: '',
  arrival: '',
  arrival_via: '',
  arrival_with_car: false,
  phone: '',
  email: '',
  useFirmForBilling: false,
  useAnotherBillingAddress: false,
  billing_address: '',
  transfer: false,
  transferText: '',
  babyBed: false,
  babyBedText: '',
  dogPackage: false,
  dogPackageText: '',
  alergies: false,
  alergiesText: '',
  accessible: false,
  additionalLinens: false,
  additionalLinensAmount: '',
  preferedCommunication: '',
  signature: '',
});

export interface ReservationSummary {
  id: number;
  code: string;
  firstName: string;
  lastName: string;
  arrival: string;
  departure: string;
  adults: number;
  children: number;
  roomName?: string;
  /** Present when the legacy kiosk API returns it (PMSApi `KioskController::reservationSummary`). */
  groupId?: number;
  prestayDone: boolean;
}

export interface FormPayload {
  config: PrestayConfig;
  guests: GuestData[];
  firm: FirmData | null;
  submitted: boolean;
  reservation: ReservationSummary | null;
}

export interface Candidate {
  candidateId: string;
  firstName: string;
}

export type LookupResponse =
  | { result: 'matched'; token: string; expiresAt: string; reservation: ReservationSummary }
  | { result: 'ambiguous'; candidateToken: string; candidates: Candidate[] }
  | { result: 'not_found' };
