import { ReactNode, useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '@/store/auth';
import { me } from '@/api/auth';
import { ApiError } from '@/api/client';

/**
 * Boots the admin SPA by fetching /me. Routes that aren't /login require
 * an authed user — RootGate redirects to /login on 401, and bounces a
 * logged-in user away from /login back to the dashboard.
 */
export const RootGate = ({ children }: { children: ReactNode }) => {
  const { status, setUser, setAnonymous } = useAuth();
  const navigate = useNavigate();
  const loc = useLocation();

  useEffect(() => {
    me()
      .then(setUser)
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) {
          setAnonymous();
        } else {
          // Network problem — treat as anonymous; user can retry login.
          setAnonymous();
        }
      });
  }, [setUser, setAnonymous]);

  useEffect(() => {
    if (status === 'anonymous' && loc.pathname !== '/login') {
      navigate('/login', { replace: true, state: { from: loc.pathname } });
    }
    if (status === 'authed' && loc.pathname === '/login') {
      navigate('/hotels', { replace: true });
    }
  }, [status, loc.pathname, navigate]);

  if (status === 'loading') {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-50 text-slate-500">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-indigo-600 border-t-transparent" />
      </div>
    );
  }
  return <>{children}</>;
};
