import { NavLink, Outlet } from 'react-router-dom'
import {
  ShieldCheck,
  LayoutDashboard,
  FileStack,
  History,
  Send,
  Activity,
  Settings,
} from 'lucide-react'
import { cn } from '@/lib/utils'

const NAV_ITEMS = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard, end: true },
  { to: '/evidence', label: 'Evidence', icon: FileStack },
  { to: '/timeline', label: 'Timeline', icon: History },
  { to: '/disclosures', label: 'Disclosures', icon: Send },
  { to: '/activity', label: 'Activity', icon: Activity },
  { to: '/settings', label: 'Settings', icon: Settings },
]

export function AppShell() {
  return (
    <div className="flex min-h-screen bg-shield-50">
      <aside className="flex w-64 shrink-0 flex-col border-r border-shield-200 bg-white">
        <div className="flex items-center gap-2 border-b border-shield-200 px-6 py-5">
          <ShieldCheck className="h-6 w-6 text-shield-700" strokeWidth={1.75} />
          <span className="text-lg font-semibold tracking-tight text-shield-950">
            Shield
          </span>
        </div>
        <nav className="flex flex-1 flex-col gap-1 px-3 py-4">
          {NAV_ITEMS.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
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
        <div className="border-t border-shield-200 px-6 py-4 text-xs leading-relaxed text-shield-400">
          Original evidence is preserved privately and never altered.
        </div>
      </aside>
      <main className="flex-1 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  )
}
