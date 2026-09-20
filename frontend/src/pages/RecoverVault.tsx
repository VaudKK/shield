import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ShieldCheck } from 'lucide-react'
import { useRecoverVault } from '@/hooks/useAuth'
import { ApiError } from '@/lib/api'

export function RecoverVault() {
  const navigate = useNavigate()
  const recoverMutation = useRecoverVault()
  const [vaultId, setVaultId] = useState('')
  const [recoveryKey, setRecoveryKey] = useState('')

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    recoverMutation.mutate(
      { vault_id: vaultId.trim(), recovery_key: recoveryKey.trim() },
      { onSuccess: () => navigate('/dashboard', { replace: true }) },
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-shield-50 px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-2">
          <ShieldCheck className="h-8 w-8 text-shield-700" strokeWidth={1.75} />
          <h1 className="text-xl font-semibold tracking-tight text-shield-950">Recover your vault</h1>
        </div>

        <form onSubmit={onSubmit} className="rounded-lg border border-shield-200 bg-white p-6">
          <div className="mb-4">
            <label htmlFor="vault_id" className="mb-1 block text-sm font-medium text-shield-900">
              Vault ID
            </label>
            <input
              id="vault_id"
              type="text"
              placeholder="SH-8F29-KD72"
              value={vaultId}
              onChange={(e) => setVaultId(e.target.value)}
              className="w-full rounded-md border border-shield-200 px-3 py-2 font-mono text-sm outline-none focus:border-shield-500"
              autoCapitalize="characters"
            />
          </div>

          <div className="mb-4">
            <label htmlFor="recovery_key" className="mb-1 block text-sm font-medium text-shield-900">
              Recovery key
            </label>
            <input
              id="recovery_key"
              type="text"
              placeholder="Q7XM-91PK-R4ZT-WL28"
              value={recoveryKey}
              onChange={(e) => setRecoveryKey(e.target.value)}
              className="w-full rounded-md border border-shield-200 px-3 py-2 font-mono text-sm outline-none focus:border-shield-500"
              autoCapitalize="characters"
            />
          </div>

          {recoverMutation.isError && (
            <p className="mb-4 text-sm text-status-rejected">
              {recoverMutation.error instanceof ApiError
                ? recoverMutation.error.message
                : 'Could not recover this vault. Please try again.'}
            </p>
          )}

          <button
            type="submit"
            disabled={recoverMutation.isPending || !vaultId.trim() || !recoveryKey.trim()}
            className="w-full rounded-md bg-shield-800 px-3 py-2 text-sm font-medium text-white hover:bg-shield-900 disabled:opacity-60"
          >
            {recoverMutation.isPending ? 'Recovering…' : 'Recover vault'}
          </button>
        </form>

        <p className="mt-4 text-center text-sm text-shield-500">
          Don't have a vault yet?{' '}
          <Link to="/create-vault" className="font-medium text-shield-800 hover:underline">
            Create one
          </Link>
        </p>
      </div>
    </div>
  )
}
