import { useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getHotel, deleteHotel } from '@/api/hotels';
import { fmtDateTime } from '@/utils/format';
import { KioskCreateModal } from '@/features/kiosks/KioskCreateModal';
import { KioskDetailCard } from '@/features/kiosks/KioskDetailCard';
import type { Kiosk } from '@/api/types';

export const HotelDetailPage = () => {
  const { id } = useParams<{ id: string }>();
  const hotelID = Number(id);
  const navigate = useNavigate();
  const qc = useQueryClient();
  const { data: hotel, isLoading } = useQuery({
    queryKey: ['hotel', hotelID],
    queryFn: () => getHotel(hotelID),
    enabled: hotelID > 0,
  });
  const [showCreate, setShowCreate] = useState(false);
  const [openKiosk, setOpenKiosk] = useState<Kiosk | null>(null);
  const del = useMutation({
    mutationFn: () => deleteHotel(hotelID),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['hotels'] });
      navigate('/hotels');
    },
  });

  if (isLoading || !hotel) return <div className="text-sm text-slate-500">Loading…</div>;

  return (
    <div>
      <div className="mb-4 text-sm">
        <Link to="/hotels" className="text-slate-500 hover:text-slate-700">
          ← Hotels
        </Link>
      </div>

      <header className="mb-6 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">{hotel.name}</h1>
          <div className="mt-1 font-mono text-xs text-slate-500">{hotel.pmsapi_url}</div>
          {hotel.notes && (
            <p className="mt-2 max-w-xl text-sm text-slate-600">{hotel.notes}</p>
          )}
        </div>
        <button
          onClick={() => {
            if (confirm('Delete this hotel and all its kiosks? This cannot be undone.')) {
              del.mutate();
            }
          }}
          className="btn-danger"
        >
          Delete hotel
        </button>
      </header>

      <section>
        <div className="mb-3 flex items-end justify-between">
          <div>
            <h2 className="text-lg font-semibold tracking-tight">Kiosks</h2>
            <p className="mt-0.5 text-sm text-slate-500">
              One row per check-in URL. Use one per <code className="font-mono">g_group</code>{' '}
              or one for the whole subdomain.
            </p>
          </div>
          <button onClick={() => setShowCreate(true)} className="btn-primary">
            + Add kiosk
          </button>
        </div>

        {hotel.kiosks && hotel.kiosks.length > 0 ? (
          <div className="card overflow-hidden">
            <table className="w-full text-sm">
              <thead className="border-b border-slate-200 bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-5 py-3 font-medium">Display name</th>
                  <th className="px-5 py-3 font-medium">Group</th>
                  <th className="px-5 py-3 font-medium">Theme</th>
                  <th className="px-5 py-3 font-medium">Status</th>
                  <th className="px-5 py-3 font-medium">Created</th>
                  <th className="px-5 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {hotel.kiosks.map((k) => (
                  <tr key={k.id} className="hover:bg-slate-50/60">
                    <td className="px-5 py-3">
                      <div className="font-medium text-slate-900">{k.display_name}</div>
                      <div className="font-mono text-xs text-slate-500">{k.id}</div>
                    </td>
                    <td className="px-5 py-3 text-slate-600">
                      {k.legacy_group_id != null
                        ? `#${k.legacy_group_id}${k.legacy_group_label ? ` · ${k.legacy_group_label}` : ''}`
                        : '— (single tenant)'}
                    </td>
                    <td className="px-5 py-3 text-slate-600">{k.theme}</td>
                    <td className="px-5 py-3">
                      <span className={k.status === 'active' ? 'badge-active' : 'badge-disabled'}>
                        {k.status}
                      </span>
                    </td>
                    <td className="px-5 py-3 text-slate-500">{fmtDateTime(k.created_at)}</td>
                    <td className="px-5 py-3 text-right">
                      <button onClick={() => setOpenKiosk(k)} className="btn-ghost">
                        Details →
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="card flex flex-col items-center justify-center gap-3 px-5 py-12 text-center">
            <p className="text-sm text-slate-500">No kiosks yet for this hotel.</p>
            <button onClick={() => setShowCreate(true)} className="btn-primary">
              + Add the first kiosk
            </button>
          </div>
        )}
      </section>

      {showCreate && (
        <KioskCreateModal hotelID={hotelID} onClose={() => setShowCreate(false)} onCreated={(k) => setOpenKiosk(k)} />
      )}
      {openKiosk && (
        <KioskDetailCard
          kiosk={openKiosk}
          onClose={() => setOpenKiosk(null)}
          onChange={() => {
            void qc.invalidateQueries({ queryKey: ['hotel', hotelID] });
          }}
        />
      )}
    </div>
  );
};
