/**
 * Tiny fetch-based admin API client.
 *
 * Always uses `credentials: 'include'` so the session cookie set by the
 * Go login endpoint flows on every subsequent request. Throws ApiError
 * for non-2xx responses so React Query can surface a friendly message.
 */

const API_BASE = '/api/admin/v1';

export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

interface ErrorBody {
  error?: { code?: string; message?: string };
}

async function request<T>(
  method: 'GET' | 'POST' | 'PATCH' | 'DELETE',
  path: string,
  body?: unknown,
): Promise<T> {
  const init: RequestInit = {
    method,
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    },
  };
  if (body !== undefined) init.body = JSON.stringify(body);

  const res = await fetch(`${API_BASE}${path}`, init);

  if (res.status === 204) return undefined as T;
  const text = await res.text();
  const parsed: unknown = text ? JSON.parse(text) : null;

  if (!res.ok) {
    const err = (parsed as ErrorBody)?.error;
    throw new ApiError(res.status, err?.code ?? 'http_error', err?.message ?? `HTTP ${res.status}`);
  }
  return parsed as T;
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  patch: <T>(path: string, body?: unknown) => request<T>('PATCH', path, body),
  delete: <T>(path: string) => request<T>('DELETE', path),
};
