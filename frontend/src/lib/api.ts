const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

export class ApiError extends Error {
  code: string
  status: number

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp('(?:^|; )' + name + '=([^;]*)'))
  return match ? decodeURIComponent(match[1]) : null
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let code = 'UNKNOWN_ERROR'
    let message = 'Something went wrong. Please try again.'
    try {
      const body = await res.json()
      code = body?.error?.code ?? code
      message = body?.error?.message ?? message
    } catch {
      // response had no JSON body
    }
    throw new ApiError(res.status, code, message)
  }

  if (res.status === 204) {
    return undefined as T
  }

  return res.json() as Promise<T>
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const method = (init?.method ?? 'GET').toUpperCase()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init?.headers as Record<string, string> | undefined),
  }

  // The double-submit CSRF cookie is non-HttpOnly specifically so the
  // frontend can read it here and echo it back on state-changing requests.
  if (method !== 'GET' && method !== 'HEAD') {
    const csrf = getCookie('shield_csrf')
    if (csrf) headers['X-CSRF-Token'] = csrf
  }

  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    ...init,
    headers,
  })

  return handleResponse<T>(res)
}

// apiUpload is a separate path from apiFetch because file uploads use
// multipart/form-data, where the browser must set the Content-Type
// (including its boundary) itself.
export async function apiUpload<T>(path: string, formData: FormData): Promise<T> {
  const csrf = getCookie('shield_csrf')

  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: csrf ? { 'X-CSRF-Token': csrf } : undefined,
    body: formData,
  })

  return handleResponse<T>(res)
}

export interface HealthStatus {
  status: string
  database: string
  version: string
  time: string
}

export function getHealth() {
  return fetch('/health').then((res) => {
    if (!res.ok) throw new Error('Health check failed')
    return res.json() as Promise<HealthStatus>
  })
}
