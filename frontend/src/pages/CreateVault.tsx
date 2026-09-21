import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ShieldCheck, AlertTriangle, Copy, Check } from 'lucide-react'
import { useCreateVault } from '@/hooks/useAuth'
import { ApiError } from '@/lib/api'
import type { CreateVaultResult } from '@/lib/vault-api'

function CopyableField({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard access denied or unavailable; the value is still selectable.
    }
  }

  return (
    <div>
      <p className="text-xs font-medium uppercase tracking-wide text-shield-500">{label}</p>
      <div className="mt-1 flex items-stretch gap-2">
        <p className="flex-1 overflow-x-auto rounded-md bg-shield-50 px-3 py-2 font-mono text-sm text-shield-950">
          {value}
        </p>
        <button
          type="button"
          onClick={handleCopy}
          aria-label={`Copy ${label}`}
          className="flex shrink-0 items-center justify-center rounded-md border border-shield-200 bg-white px-2.5 text-shield-600 transition-colors hover:border-shield-400 hover:text-shield-900"
        >
          {copied ? (
            <Check className="h-4 w-4 text-status-approved" strokeWidth={1.75} />
          ) : (
            <Copy className="h-4 w-4" strokeWidth={1.75} />
          )}
        </button>
      </div>
    </div>
  )
}

export function CreateVault() {
  const navigate = useNavigate()
  const createMutation = useCreateVault()
  const [result, setResult] = useState<CreateVaultResult | null>(null)
  const [confirmed, setConfirmed] = useState(false)

  // Create the vault once, on mount.
  useEffect(() => {
    if (result || createMutation.isPending || createMutation.isError) return
    createMutation.mutate(undefined, { onSuccess: setResult })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  if (createMutation.isError) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-shield-50 px-4">
        <div className="w-full max-w-sm rounded-lg border border-shield-200 bg-white p-6 text-center">
          <p className="text-sm text-status-rejected">
            {createMutation.error instanceof ApiError
              ? createMutation.error.message
              : 'Could not create a vault. Please try again.'}
          </p>
          <button
            type="button"
            onClick={() => navigate('/')}
            className="mt-4 text-sm font-medium text-shield-700 hover:underline"
          >
            Back to start
          </button>
        </div>
      </div>
    )
  }

  if (!result) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-shield-50">
        <ShieldCheck className="h-8 w-8 animate-pulse text-shield-300" strokeWidth={1.75} />
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-shield-50 px-4">
      <div className="w-full max-w-md rounded-lg border border-shield-200 bg-white p-6">
        <h1 className="text-lg font-semibold text-shield-950">Your secure vault is ready</h1>

        <div className="mt-4 space-y-3">
          <CopyableField label="Vault ID" value={result.vault_id} />
          <CopyableField label="Recovery Key" value={result.recovery_key} />
        </div>

        <div className="mt-4 flex items-start gap-2 rounded-md border border-status-review/30 bg-amber-50 px-3 py-2.5 text-xs text-shield-700">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-status-review" strokeWidth={1.75} />
          <p>
            <strong>Save your recovery key now.</strong> Shield does not store it and cannot show
            it to you again. You will need both your Vault ID and this key to recover access on
            another device.
          </p>
        </div>

        <label className="mt-4 flex items-start gap-2 text-sm text-shield-800">
          <input
            type="checkbox"
            checked={confirmed}
            onChange={(e) => setConfirmed(e.target.checked)}
            className="mt-0.5 h-4 w-4 rounded border-shield-300"
          />
          I've saved my recovery key
        </label>

        <button
          type="button"
          onClick={() => navigate('/dashboard', { replace: true })}
          disabled={!confirmed}
          className="mt-4 w-full rounded-md bg-shield-800 px-4 py-2.5 text-sm font-medium text-white hover:bg-shield-900 disabled:opacity-50"
        >
          Continue to dashboard
        </button>
      </div>
    </div>
  )
}
