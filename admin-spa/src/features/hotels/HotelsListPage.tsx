import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { listHotels, createHotel } from '@/api/hotels';
import { fmtDateTime } from '@/utils/format';
import type { HotelInput } from '@/api/types';

export const HotelsListPage = () => {
  const { data: hotels, isLoading } = useQuery({ queryKey: ['hotels'], queryFn: listHotels });
  const [open, setOpen] = useState(false);

  return (
    <div>
      <header className="mb-6 flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Hotels</h1>
          <p className="mt-1 text-sm text-slate-500">
            One row per legacy PMSApi subdomain. Each hotel has one or more kiosks.
          </p>
        </div>
        <button onClick={() => setOpen(true)} className="btn-primary">
          + Add hotel
        </button>
      </header>

      {isLoading ? (
        <div className="text-sm text-slate-500">Loading…</div>
      ) : hotels && hotels.length > 0 ? (
        <div className="card overflow-hidden">
          <table className="w-full text-sm">
            <thead className="border-b border-slate-200 bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-5 py-3 font-medium">Name</th>
                <th className="px-5 py-3 font-medium">PMSApi URL</th>
                <th className="px-5 py-3 font-medium">Kiosks</th>
                <th className="px-5 py-3 font-medium">Created</th>
                <th className="px-5 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {hotels.map((h) => (
                <tr key={h.id} className="hover:bg-slate-50/60">
                  <td className="px-5 py-3 font-medium text-slate-900">{h.name}</td>
                  <td className="px-5 py-3 font-mono text-xs text-slate-600">{h.pmsapi_url}</td>
                  <td className="px-5 py-3 tabular-nums text-slate-600">{h.kiosk_count}</td>
                  <td className="px-5 py-3 text-slate-500">{fmtDateTime(h.created_at)}</td>
                  <td className="px-5 py-3 text-right">
                    <Link to={`/hotels/${h.id}`} className="btn-ghost">
                      Open →
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <EmptyState onAdd={() => setOpen(true)} />
      )}

      {open && <CreateHotelModal onClose={() => setOpen(false)} />}
    </div>
  );
};

const EmptyState = ({ onAdd }: { onAdd: () => void }) => (
  <div className="card flex flex-col items-center justify-center gap-3 px-5 py-16 text-center">
    <div className="text-base font-medium text-slate-800">No hotels yet</div>
    <p className="max-w-md text-sm text-slate-500">
      Add a hotel to connect a legacy PMSApi subdomain. You'll then create kiosk URLs under it,
      one per <code className="font-mono">g_group</code> or one for the whole subdomain.
    </p>
    <button onClick={onAdd} className="btn-primary mt-2">
      + Add the first hotel
    </button>
  </div>
);

const CreateHotelModal = ({ onClose }: { onClose: () => void }) => {
  const qc = useQueryClient();
  const [form, setForm] = useState<HotelInput>({ name: '', pmsapi_url: '', notes: '' });
  const [err, setErr] = useState<string | null>(null);
  const m = useMutation({
    mutationFn: () => createHotel(form),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['hotels'] });
      onClose();
    },
    onError: (e: Error) => setErr(e.message),
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 px-6">
      <div className="w-full max-w-md card">
        <div className="card-header">
          <h2 className="text-base font-semibold">New hotel</h2>
          <button onClick={onClose} className="btn-ghost">
            ✕
          </button>
        </div>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            m.mutate();
          }}
          className="card-body space-y-4"
        >
          <div>
            <label className="label" htmlFor="name">
              Display name
            </label>
            <input
              id="name"
              className="input"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder="smart moov"
              required
            />
          </div>
          <div>
            <label className="label" htmlFor="url">
              PMSApi URL
            </label>
            <input
              id="url"
              type="url"
              className="input font-mono text-xs"
              value={form.pmsapi_url}
              onChange={(e) => setForm({ ...form, pmsapi_url: e.target.value })}
              placeholder="https://pms.maiccube.com"
              required
            />
            <p className="hint">
              The legacy domain the kiosk forwards to. Must serve the <code>/api/kiosk/*</code>
              {' '}patch.
            </p>
          </div>
          <div>
            <label className="label" htmlFor="notes">
              Notes (optional)
            </label>
            <textarea
              id="notes"
              className="input min-h-[80px]"
              value={form.notes ?? ''}
              onChange={(e) => setForm({ ...form, notes: e.target.value })}
            />
          </div>
          {err && (
            <p className="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700 ring-1 ring-red-100">
              {err}
            </p>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <button type="button" onClick={onClose} className="btn-secondary">
              Cancel
            </button>
            <button type="submit" disabled={m.isPending} className="btn-primary">
              {m.isPending ? 'Creating…' : 'Create hotel'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
