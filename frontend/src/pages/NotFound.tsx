import { Link } from 'react-router-dom'
import { ShieldQuestion } from 'lucide-react'

export function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-3 bg-shield-50 px-4 text-center">
      <ShieldQuestion className="h-8 w-8 text-shield-400" strokeWidth={1.75} />
      <h1 className="text-lg font-semibold text-shield-950">Page not found</h1>
      <p className="max-w-sm text-sm text-shield-500">
        The page you're looking for doesn't exist or may have moved.
      </p>
      <Link
        to="/"
        className="mt-2 rounded-md bg-shield-800 px-4 py-2 text-sm font-medium text-white hover:bg-shield-900"
      >
        Back to start
      </Link>
    </div>
  )
}
