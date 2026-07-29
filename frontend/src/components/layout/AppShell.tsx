import { useEffect } from 'react'
import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom'
import { LayoutDashboard, TrendingUp, FileText, Settings, LogOut } from 'lucide-react'
import { useUser } from '../../hooks/useUser'
import { api } from '../../api/client'

const navItems = [
  { to: '/dashboard', label: 'Dashboard', Icon: LayoutDashboard },
  { to: '/insights', label: 'Insights', Icon: TrendingUp },
  { to: '/reports', label: 'Reports', Icon: FileText },
  { to: '/settings', label: 'Settings', Icon: Settings },
]

export function AppShell() {
  const { user } = useUser()
  const navigate = useNavigate()
  const location = useLocation()

  // Redirect new users to onboarding on first mount
  useEffect(() => {
    if (location.pathname === '/onboarding') return
    api
      .get<{ completed: boolean }>('/api/onboarding')
      .then((data) => {
        if (!data.completed) navigate('/onboarding', { replace: true })
      })
      .catch(() => {
        // Silently ignore — auth redirect handled by api client on 401
      })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function handleSignOut() {
    try {
      await api.post('/api/auth/logout')
    } finally {
      navigate('/login')
    }
  }

  return (
    <div className="flex h-screen bg-slate-950 overflow-hidden">
      {/* Sidebar */}
      <aside className="flex w-64 flex-col border-r border-slate-800 bg-slate-950 shrink-0">
        {/* Logo */}
        <div className="flex h-16 items-center gap-2 px-6 border-b border-slate-800">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-br from-violet-500 to-purple-600">
            <span className="text-xs font-bold text-white">A</span>
          </div>
          <span className="font-bold text-white">AI Stats</span>
        </div>

        {/* Navigation */}
        <nav className="flex-1 space-y-1 px-3 py-4">
          {navItems.map(({ to, label, Icon }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                `flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors ${
                  isActive
                    ? 'bg-slate-800 text-white'
                    : 'text-slate-400 hover:bg-slate-800 hover:text-white'
                }`
              }
            >
              <Icon className="h-4 w-4 shrink-0" />
              {label}
            </NavLink>
          ))}
        </nav>

        {/* User section */}
        <div className="border-t border-slate-800 p-3">
          <div className="flex items-center gap-3 rounded-lg px-3 py-2">
            {user?.image ? (
              <img
                src={user.image}
                alt={user.name ?? 'User'}
                className="h-8 w-8 rounded-full"
              />
            ) : (
              <div className="h-8 w-8 rounded-full bg-slate-700 flex items-center justify-center text-xs font-medium text-slate-300">
                {user?.name?.[0]?.toUpperCase() ?? '?'}
              </div>
            )}
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-white">{user?.name ?? 'Loading...'}</p>
              <p className="truncate text-xs text-slate-500">{user?.email}</p>
            </div>
          </div>
          <button
            onClick={handleSignOut}
            className="mt-1 flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm text-slate-400 hover:bg-slate-800 hover:text-white transition-colors"
          >
            <LogOut className="h-4 w-4" />
            Sign out
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  )
}
