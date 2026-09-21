import { useState } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { ShieldCheck, LayoutDashboard, FileStack, Send, LogOut, Menu, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useCurrentUser, useLogout } from '@/hooks/useAuth'

const NAV_ITEMS = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard, end: true },
  { to: '/dashboard/evidence', label: 'Evidence', icon: FileStack },
  { to: '/dashboard/disclosures', label: 'Disclosures', icon: Send },
]

export function AppShell() {
  const navigate = useNavigate()
  const { data: user } = useCurrentUser()
  const logoutMutation = useLogout()
  const [navOpen, setNavOpen] = useState(false)

  const handleLogout = () => {
    logoutMutation.mutate(undefined, {
      onSuccess: () => navigate('/', { replace: true }),
    })
  }

  const sidebarContent = (
    <>
      <div className="flex items-center justify-between gap-2 border-b border-shield-200 px-6 py-5">
        <div className="flex items-center gap-2">
          <ShieldCheck className="h-6 w-6 text-shield-700" strokeWidth={1.75} />
          <span className="text-lg font-semibold tracking-tight text-shield-950">Shield</span>
        </div>
        <button
          type="button"
          onClick={() => setNavOpen(false)}
          aria-label="Close menu"
          className="rounded-md p-1.5 text-shield-400 hover:bg-shield-50 hover:text-shield-700 lg:hidden"
        >
          <X className="h-5 w-5" strokeWidth={1.75} />
        </button>
      </div>
      <nav className="flex flex-1 flex-col gap-1 px-3 py-4">
        {NAV_ITEMS.map(({ to, label, icon: Icon, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            onClick={() => setNavOpen(false)}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                isActive
                  ? 'bg-shield-100 text-shield-900'
                  : 'text-shield-600 hover:bg-shield-50 hover:text-shield-900',
              )
            }
          >
            <Icon className="h-4 w-4" strokeWidth={1.75} />
            {label}
          </NavLink>
        ))}
      </nav>
      {user && (
        <div className="flex items-center justify-between gap-2 border-t border-shield-200 px-4 py-3">
          <div className="min-w-0">
            <p className="truncate text-sm font-medium text-shield-900">{user.display_name}</p>
            <p className="truncate font-mono text-xs text-shield-400">
              {user.vault_id ?? user.email}
            </p>
          </div>
          <button
            type="button"
            onClick={handleLogout}
            aria-label="Sign out"
            className="rounded-md p-2 text-shield-400 transition-colors hover:bg-shield-50 hover:text-shield-700"
          >
            <LogOut className="h-4 w-4" strokeWidth={1.75} />
          </button>
        </div>
      )}
      <div className="border-t border-shield-200 px-6 py-4 text-xs leading-relaxed text-shield-400">
        Original evidence is preserved privately and never altered.
      </div>
    </>
  )

  return (
    <div className="flex min-h-screen bg-shield-50">
      {/* Mobile top bar */}
      <div className="fixed inset-x-0 top-0 z-20 flex items-center gap-3 border-b border-shield-200 bg-white px-4 py-3 lg:hidden">
        <button
          type="button"
          onClick={() => setNavOpen(true)}
          aria-label="Open menu"
          className="rounded-md p-1.5 text-shield-600 hover:bg-shield-50"
        >
          <Menu className="h-5 w-5" strokeWidth={1.75} />
        </button>
        <ShieldCheck className="h-5 w-5 text-shield-700" strokeWidth={1.75} />
        <span className="text-base font-semibold tracking-tight text-shield-950">Shield</span>
      </div>

      {/* Mobile overlay */}
      {navOpen && (
        <div
          onClick={() => setNavOpen(false)}
          className="fixed inset-0 z-30 bg-black/30 lg:hidden"
          aria-hidden="true"
        />
      )}

      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-40 flex w-64 shrink-0 flex-col border-r border-shield-200 bg-white transition-transform duration-200 lg:static lg:translate-x-0',
          navOpen ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        {sidebarContent}
      </aside>

      <main className="flex-1 overflow-y-auto pt-14 lg:pt-0">
        <Outlet />
      </main>
    </div>
  )
}
