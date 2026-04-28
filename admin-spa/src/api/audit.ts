import { api } from './client';
import type { AuditEntry } from './types';

export const listAudit = (limit = 50, before = 0) => {
  const q = new URLSearchParams({ limit: String(limit) });
  if (before > 0) q.set('before', String(before));
  return api.get<{ entries: AuditEntry[] | null }>(`/audit-log?${q.toString()}`).then((r) => r.entries ?? []);
};
