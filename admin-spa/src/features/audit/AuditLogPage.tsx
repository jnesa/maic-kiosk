import { useQuery } from '@tanstack/react-query';
import { listAudit } from '@/api/audit';
import { fmtDateTime } from '@/utils/format';

export const AuditLogPage = () => {
  const { data: entries, isLoading } = useQuery({
    queryKey: ['audit', 50],
    queryFn: () => listAudit(50, 0),
  });

  return (
    <div>
      <header className="mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">Audit log</h1>
        <p className="mt-1 text-sm text-slate-500">
          Every admin write action. Most recent first. Secrets (device keys, password hashes)
          are stripped before they're logged.
        </p>
      </header>

      {isLoading ? (
        <div className="text-sm text-slate-500">Loading…</div>
      ) : entries && entries.length > 0 ? (
        <div className="card overflow-hidden">
          <table className="w-full text-sm">
            <thead className="border-b border-slate-200 bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-5 py-3 font-medium">When</th>
                <th className="px-5 py-3 font-medium">Actor</th>
                <th className="px-5 py-3 font-medium">Action</th>
                <th className="px-5 py-3 font-medium">Entity</th>
                <th className="px-5 py-3 font-medium">Payload</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {entries.map((e) => (
                <tr key={e.ID} className="hover:bg-slate-50/60">
                  <td className="whitespace-nowrap px-5 py-3 font-mono text-xs text-slate-600">
                    {fmtDateTime(e.CreatedAt)}
                  </td>
                  <td className="px-5 py-3 text-slate-700">{e.ActorEmail ?? '—'}</td>
                  <td className="px-5 py-3 font-medium text-slate-900">{e.Action}</td>
                  <td className="px-5 py-3 font-mono text-xs text-slate-600">
                    {e.EntityType}/{e.EntityID}
                  </td>
                  <td className="px-5 py-3">
                    {e.Payload ? (
                      <pre className="max-w-md overflow-x-auto rounded bg-slate-50 p-2 font-mono text-xs text-slate-700">
                        {JSON.stringify(e.Payload, null, 2)}
                      </pre>
                    ) : (
                      <span className="text-slate-400">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="card flex items-center justify-center px-5 py-16 text-sm text-slate-500">
          No actions recorded yet.
        </div>
      )}
    </div>
  );
};
