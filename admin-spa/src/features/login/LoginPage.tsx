import { FormEvent, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { login } from '@/api/auth';
import { ApiError } from '@/api/client';
import { useAuth } from '@/store/auth';

export const LoginPage = () => {
  const setUser = useAuth((s) => s.setUser);
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      const { user } = await login(email.trim(), password);
      setUser(user);
      navigate('/hotels', { replace: true });
    } catch (ex) {
      if (ex instanceof ApiError && ex.status === 401) {
        setErr('Wrong email or password.');
      } else {
        setErr('Something went wrong. Please try again.');
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-50 px-6">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <div className="text-lg font-semibold tracking-tight text-slate-900">
            newMasterCheckin
          </div>
          <div className="text-sm text-slate-500">Internal admin</div>
        </div>
        <div className="card">
          <form onSubmit={onSubmit} className="card-body space-y-4">
            <div>
              <label className="label" htmlFor="email">
                Email
              </label>
              <input
                id="email"
                type="email"
                autoComplete="email"
                autoFocus
                className="input"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <div>
              <label className="label" htmlFor="password">
                Password
              </label>
              <input
                id="password"
                type="password"
                autoComplete="current-password"
                className="input"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            {err && (
              <p className="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700 ring-1 ring-red-100">
                {err}
              </p>
            )}
            <button type="submit" disabled={busy} className="btn-primary w-full">
              {busy ? 'Signing in…' : 'Sign in'}
            </button>
          </form>
        </div>
        <p className="mt-4 text-center text-xs text-slate-500">
          Operators are provisioned via the <code className="font-mono">admin add-user</code> CLI.
        </p>
      </div>
    </div>
  );
};
