import { Navigate, Outlet } from 'react-router-dom'
import { ShieldCheck } from 'lucide-react'
import { useCurrentUser } from '@/hooks/useAuth'

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
    return <Navigate to="/login" replace />
  }

  return <Outlet />
}
