import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getMe, login, logout, register, type User } from '@/lib/auth-api'
import { createVault, recoverVault } from '@/lib/vault-api'
import { ApiError } from '@/lib/api'

const ME_QUERY_KEY = ['me']

export function useCurrentUser() {
  return useQuery<User, ApiError>({
    queryKey: ME_QUERY_KEY,
    queryFn: getMe,
    retry: false,
  })
}

export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: login,
    onSuccess: (user) => queryClient.setQueryData(ME_QUERY_KEY, user),
  })
}

export function useRegister() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: register,
    onSuccess: (user) => queryClient.setQueryData(ME_QUERY_KEY, user),
  })
}

export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: logout,
    onSuccess: () => queryClient.setQueryData(ME_QUERY_KEY, null),
  })
}

export function useCreateVault() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: createVault,
    // The create-vault response is {vault_id, recovery_key}, not a full
    // User — a session is already active server-side, so just refetch /me
    // rather than trying to reconstruct a User object here.
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ME_QUERY_KEY }),
  })
}

export function useRecoverVault() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: recoverVault,
    onSuccess: (user) => queryClient.setQueryData(ME_QUERY_KEY, user),
  })
}
