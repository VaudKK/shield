import { apiFetch } from '@/lib/api'

export interface User {
  id: string
  email?: string
  vault_id?: string
  display_name: string
  created_at: string
}

export function getMe() {
  return apiFetch<User>('/auth/me')
}

export function register(input: { email: string; password: string; display_name: string }) {
  return apiFetch<User>('/auth/register', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function login(input: { email: string; password: string }) {
  return apiFetch<User>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function logout() {
  return apiFetch<void>('/auth/logout', { method: 'POST' })
}
