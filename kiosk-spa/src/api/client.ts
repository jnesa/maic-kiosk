import axios, { AxiosError, AxiosInstance } from 'axios';

/*
 * Multi-tenant kiosk API client.
 *
 * The kiosk SPA is hosted on a single domain (e.g. checkin.maiccube.com).
 * Each property is selected by URL path:
 *   /smart-moov/...  → property slug "smart-moov"
 *   /pareus/...      → property slug "pareus"
 *
 * The browser only ever talks to the kiosk's own backend at
 * /api/kiosk/v1/<slug>/<endpoint>. The Go proxy holds the per-property
 * device key and forwards to the legacy PMSApi behind the scenes — the
 * device key never reaches the browser.
 */

/**
 * Reads the property slug from the current URL path. The slug is the
 * first non-empty path segment that isn't a reserved app route. We allow
 * fallback to a `?slug=` query param so a tester or the backend's
 * "/properties" landing can deep-link without rewriting URLs.
 */
export const propertySlug = (): string => {
  const fromPath = window.location.pathname.split('/').filter(Boolean)[0] ?? '';
  if (fromPath && !RESERVED_TOP_LEVEL.has(fromPath)) return fromPath;
  const fromQuery = new URLSearchParams(window.location.search).get('slug') ?? '';
  return fromQuery;
};

// First-segment paths that must NOT be interpreted as kiosk slugs. These
// are reserved app-level routes (admin panel, API surface, static
// asset/locale folders, health endpoints) that should never collide with
// generated kiosk UUIDs.
const RESERVED_TOP_LEVEL = new Set(['', 'admin', 'api', 'assets', 'locales', 'health']);

const slug = propertySlug();
const apiBase = `/api/kiosk/v1/${slug}`;

export const apiClient: AxiosInstance = axios.create({
  baseURL: apiBase,
  timeout: 15_000,
  headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
});

apiClient.interceptors.response.use(
  (res) => res,
  (err: AxiosError) => Promise.reject(err),
);
