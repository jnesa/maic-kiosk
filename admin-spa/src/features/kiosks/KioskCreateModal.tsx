import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createKiosk } from '@/api/hotels';
import type { Kiosk, KioskInput } from '@/api/types';

const BUNDLED_THEMES = ['smart-moov', 'pareus'];
const ALL_LANGS = ['en', 'de', 'it', 'fr'];

interface Props {
  hotelID: number;
  onClose: () => void;
  onCreated: (k: Kiosk) => void;
}

export const KioskCreateModal = ({ hotelID, onClose, onCreated }: Props) => {
  const qc = useQueryClient();
  const [form, setForm] = useState<KioskInput & { legacy_group_id_str: string }>({
    display_name: '',
    legacy_group_id: null,
    legacy_group_id_str: '',
    legacy_group_label: '',
    theme: 'smart-moov',
    languages: ['en', 'de', 'it'],
    hero_image: '',
    logo: '',
    support_phone: '',
    support_email: '',
  });
  const [err, setErr] = useState<string | null>(null);

  const m = useMutation({
    mutationFn: () => {
      const groupID = form.legacy_group_id_str.trim() === ''
        ? null
        : Number(form.legacy_group_id_str);
      if (groupID !== null && Number.isNaN(groupID)) {
        throw new Error('legacy_group_id must be a number');
      }
      const payload: KioskInput = {
        display_name: form.display_name,
        legacy_group_id: groupID,
        legacy_group_label: form.legacy_group_label,
        theme: form.theme,
        languages: form.languages,
        hero_image: form.hero_image,
        logo: form.logo,
        support_phone: form.support_phone,
        support_email: form.support_email,
      };
      return createKiosk(hotelID, payload);
    },
    onSuccess: (k) => {
      void qc.invalidateQueries({ queryKey: ['hotel', hotelID] });
      onCreated(k);
      onClose();
    },
    onError: (e: Error) => setErr(e.message),
  });

  const toggleLang = (l: string) => {
    setForm((f) => ({
      ...f,
      languages: f.languages.includes(l)
        ? f.languages.filter((x) => x !== l)
        : [...f.languages, l],
    }));
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 px-6">
      <div className="w-full max-w-xl card max-h-[92vh] overflow-y-auto">
        <div className="card-header sticky top-0 bg-white">
          <h2 className="text-base font-semibold">New kiosk</h2>
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
            <label className="label" htmlFor="display_name">
              Display name
            </label>
            <input
              id="display_name"
              className="input"
              value={form.display_name}
              onChange={(e) => setForm({ ...form, display_name: e.target.value })}
              placeholder="smart moov — lobby"
              required
            />
            <p className="hint">Shown in this dashboard. Guests don't see it.</p>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="label" htmlFor="group_id">
                Legacy group id
              </label>
              <input
                id="group_id"
                inputMode="numeric"
                className="input"
                value={form.legacy_group_id_str}
                onChange={(e) => setForm({ ...form, legacy_group_id_str: e.target.value })}
                placeholder="leave blank for single-tenant"
              />
              <p className="hint">
                The <code>g_group.id</code> on the legacy PMSApi. Blank if the subdomain doesn't
                use groups.
              </p>
            </div>
            <div>
              <label className="label" htmlFor="group_label">
                Group label
              </label>
              <input
                id="group_label"
                className="input"
                value={form.legacy_group_label}
                onChange={(e) => setForm({ ...form, legacy_group_label: e.target.value })}
                placeholder="Hotel A"
              />
              <p className="hint">For your dashboard. Optional.</p>
            </div>
          </div>

          <div>
            <label className="label" htmlFor="theme">
              Theme
            </label>
            <select
              id="theme"
              className="input"
              value={form.theme}
              onChange={(e) => setForm({ ...form, theme: e.target.value })}
            >
              {BUNDLED_THEMES.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </div>

          <div>
            <span className="label">Languages</span>
            <div className="flex flex-wrap gap-2">
              {ALL_LANGS.map((l) => {
                const active = form.languages.includes(l);
                return (
                  <button
                    key={l}
                    type="button"
                    onClick={() => toggleLang(l)}
                    className={
                      'rounded-md px-3 py-1.5 text-sm font-medium ring-1 transition ' +
                      (active
                        ? 'bg-indigo-50 text-indigo-700 ring-indigo-200'
                        : 'bg-white text-slate-600 ring-slate-200 hover:bg-slate-50')
                    }
                  >
                    {l.toUpperCase()}
                  </button>
                );
              })}
            </div>
          </div>

          <details className="rounded-lg border border-slate-200 bg-slate-50/50 px-4 py-3">
            <summary className="cursor-pointer text-sm font-medium text-slate-700">
              Optional assets &amp; support contact
            </summary>
            <div className="mt-3 space-y-3">
              <div>
                <label className="label" htmlFor="hero">
                  Hero image URL
                </label>
                <input
                  id="hero"
                  className="input font-mono text-xs"
                  value={form.hero_image}
                  onChange={(e) => setForm({ ...form, hero_image: e.target.value })}
                  placeholder="/themes/smart-moov/hero.jpg"
                />
              </div>
              <div>
                <label className="label" htmlFor="logo">
                  Logo URL
                </label>
                <input
                  id="logo"
                  className="input font-mono text-xs"
                  value={form.logo}
                  onChange={(e) => setForm({ ...form, logo: e.target.value })}
                  placeholder="/themes/smart-moov/logo.svg"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="label" htmlFor="phone">
                    Support phone
                  </label>
                  <input
                    id="phone"
                    className="input"
                    value={form.support_phone}
                    onChange={(e) => setForm({ ...form, support_phone: e.target.value })}
                  />
                </div>
                <div>
                  <label className="label" htmlFor="email">
                    Support email
                  </label>
                  <input
                    id="email"
                    type="email"
                    className="input"
                    value={form.support_email}
                    onChange={(e) => setForm({ ...form, support_email: e.target.value })}
                  />
                </div>
              </div>
            </div>
          </details>

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
              {m.isPending ? 'Creating…' : 'Create kiosk'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
