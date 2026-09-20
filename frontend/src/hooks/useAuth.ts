import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getMe, login, logout, register, type User } from '@/lib/auth-api'
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
