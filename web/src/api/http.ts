export type ApiError = { status: number; message: string }

function decodeJwtUserId(accessToken: string): number {
  try {
    const payload = accessToken.split('.')[1]
    const json = JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/')))
    const raw = json.user_id ?? json.uid ?? json.sub ?? '0'
    return Number(raw) || 0
  } catch {
    return 0
  }
}

type AuthSnap = {
  accessToken: string | null
  refreshToken: string | null
  userId: number
  setTokens: (access: string, refresh: string) => void
  clear: () => void
}

let authGetter: (() => AuthSnap) | null = null

export function bindAuthStore(getter: () => AuthSnap) {
  authGetter = getter
}

async function refreshAccess(): Promise<string | null> {
  const auth = authGetter?.()
  if (!auth?.refreshToken) return null
  const res = await fetch('/api/v1/auth/access', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ refreshToken: auth.refreshToken }),
  })
  if (!res.ok) {
    auth.clear()
    return null
  }
  const data = (await res.json()) as { accessToken?: string }
  if (!data.accessToken) {
    auth.clear()
    return null
  }
  auth.setTokens(data.accessToken, auth.refreshToken)
  return data.accessToken
}

export async function api<T>(
  path: string,
  init: RequestInit = {},
  opts: { auth?: boolean } = { auth: true },
): Promise<T> {
  const headers = new Headers(init.headers)
  if (!headers.has('content-type') && init.body) {
    headers.set('content-type', 'application/json')
  }
  if (opts.auth !== false) {
    const token = authGetter?.().accessToken
    if (token) headers.set('authorization', `Bearer ${token}`)
  }

  let res = await fetch(path, { ...init, headers })
  if (res.status === 401 && opts.auth !== false) {
    const next = await refreshAccess()
    if (next) {
      headers.set('authorization', `Bearer ${next}`)
      res = await fetch(path, { ...init, headers })
    }
  }

  if (!res.ok) {
    let message = res.statusText
    try {
      const body = await res.json()
      message = body.message || body.error || JSON.stringify(body)
    } catch {
      /* ignore */
    }
    const err: ApiError = { status: res.status, message }
    throw err
  }

  if (res.status === 204) return undefined as T
  const text = await res.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}

export { decodeJwtUserId }
