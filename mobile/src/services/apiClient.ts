import { useSessionStore } from '../store/sessionStore'

const API_BASE_URL = 'http://localhost:8080'

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PATCH'
  body?: unknown
  token?: string | null
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { token } = useSessionStore.getState()
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: options.method ?? 'GET',
    headers: {
      'Content-Type': 'application/json',
      ...(options.token ?? token ? { Authorization: `Bearer ${options.token ?? token}` } : {})
    },
    body: options.body ? JSON.stringify(options.body) : undefined
  })

  if (!response.ok) {
    throw new Error(`API error: ${response.status}`)
  }

  return (await response.json()) as T
}
