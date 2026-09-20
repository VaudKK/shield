import { Link } from 'react-router-dom'
import { ShieldCheck, Lock, KeyRound } from 'lucide-react'

export function Landing() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-shield-50 px-4">
      <div className="w-full max-w-md text-center">
        <ShieldCheck className="mx-auto mb-4 h-10 w-10 text-shield-700" strokeWidth={1.5} />
        <h1 className="text-2xl font-semibold tracking-tight text-shield-950">Shield</h1>
        <p className="mt-2 text-sm text-shield-600">
          Preserve evidence. Protect your identity. Decide what to share.
        </p>

        <div className="mt-8 flex flex-col gap-3">
          <Link
            to="/create-vault"
            className="rounded-md bg-shield-800 px-4 py-3 text-sm font-medium text-white hover:bg-shield-900"
          >
            Create Secure Vault
          </Link>
          <Link
            to="/recover-vault"
            className="rounded-md border border-shield-300 bg-white px-4 py-3 text-sm font-medium text-shield-800 hover:bg-shield-50"
          >
            Recover Existing Vault
          </Link>
        </div>

        <div className="mt-8 space-y-2 text-left text-xs text-shield-500">
          <p className="flex items-start gap-2">
            <Lock className="mt-0.5 h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
            No account is required. Your original evidence is preserved privately and is never
            replaced by a redacted copy.
          </p>
          <p className="flex items-start gap-2">
            <KeyRound className="mt-0.5 h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
            You control what gets disclosed. Shield does not determine whether an incident
            occurred.
          </p>
        </div>
      </div>
    </div>
  )
}
