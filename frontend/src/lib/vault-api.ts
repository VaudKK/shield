import { apiFetch } from '@/lib/api'
import type { User } from '@/lib/auth-api'

export interface CreateVaultResult {
  vault_id: string
  recovery_key: string
}

export function createVault() {
  return apiFetch<CreateVaultResult>('/vaults/', { method: 'POST' })
}

export function recoverVault(input: { vault_id: string; recovery_key: string }) {
  return apiFetch<User>('/vaults/recover', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
