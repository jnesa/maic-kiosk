import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { useAuth } from '@/store/auth';
import { logout } from '@/api/auth';
import { cn } from '@/utils/cn';

/** Two-column layout: left rail nav, main pane. Renders only after the
 *  user is known; RootGate redirects anonymous users to /login first. */
export const Layout = () => {
  const user = useAuth((s) => s.user);
  const setAnonymous = useAuth((s) => s.setAnonymous);
  const navigate = useNavigate();

  const onLogout = async () => {
    try {
      await logout();
    } finally {
      setAnonymous();
      navigate('/login', { replace: true });
    }
  };

  return (
    <div className="flex min-h-screen bg-slate-50 text-slate-900">
      <aside className="flex w-60 flex-col border-r border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-5 py-5">
          <div className="text-base font-semibold tracking-tight">newMasterCheckin</div>
          <div className="mt-0.5 text-xs text-slate-500">Internal admin</div>
        </div>
        <nav className="flex flex-col gap-1 p-3">
          <NavItem to="/hotels">Hotels &amp; kiosks</NavItem>
          <NavItem to="/audit-log">Audit log</NavItem>
        </nav>
        <div className="mt-auto border-t border-slate-200 p-3">
          {user && (
            <div className="mb-2 px-2 text-xs">
              <div className="truncate font-medium text-slate-800">{user.name}</div>
              <div className="truncate text-slate-500">{user.email}</div>
            </div>
          )}
          <button onClick={onLogout} className="btn-ghost w-full justify-start">
            Sign out
          </button>
        </div>
      </aside>
      <main className="flex-1 overflow-x-auto">
        <div className="mx-auto max-w-6xl px-8 py-8">
          <Outlet />
        </div>
      </main>
    </div>
  );
};

const NavItem = ({ to, children }: { to: string; children: React.ReactNode }) => (
  <NavLink
    to={to}
    className={({ isActive }) =>
      cn(
        'rounded-md px-3 py-2 text-sm font-medium transition-colors',
        isActive ? 'bg-indigo-50 text-indigo-700' : 'text-slate-700 hover:bg-slate-100',
      )
    }
  >
    {children}
  </NavLink>
);
