import { Navigate, Outlet } from 'react-router-dom'
import { ShieldCheck } from 'lucide-react'
import { useCurrentUser } from '@/hooks/useAuth'

// Gates the dashboard behind an active session — a vault session (the
// default path) or an email/password session (the optional path, see
// Login.tsx/Register.tsx). Either way, "no session" sends the user back to
// the landing page to create or recover a vault, never to a login screen.
export function RequireAuth() {
  const { data: user, isLoading, isError } = useCurrentUser()

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-shield-50">
        <ShieldCheck className="h-8 w-8 animate-pulse text-shield-300" strokeWidth={1.75} />
      </div>
    )
  }

  if (isError || !user) {
    return <Navigate to="/" replace />
  }

  return <Outlet />
}
