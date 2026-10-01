import { api, decodeJwtUserId } from './http'

export type LoginResult = {
  accessToken: string
  refreshToken: string
  userId: number
}

export async function login(email: string, password: string): Promise<LoginResult> {
  const data = await api<{ accessToken: string; refreshToken: string }>(
    '/api/v1/auth/login',
    { method: 'POST', body: JSON.stringify({ email, password }) },
    { auth: false },
  )
  return {
    accessToken: data.accessToken,
    refreshToken: data.refreshToken,
    userId: decodeJwtUserId(data.accessToken),
  }
}

export async function register(name: string, email: string, password: string) {
  await api(
    '/api/v1/users',
    {
      method: 'POST',
      body: JSON.stringify({
        userInfo: { name, email },
        password,
        passwordConfirm: password,
      }),
    },
    { auth: false },
  )
  return login(email, password)
}

export async function logout(refreshToken: string) {
  try {
    await api('/api/v1/auth/logout', {
      method: 'POST',
      body: JSON.stringify({ refreshToken }),
    })
  } catch {
    /* ignore */
  }
}

export async function getUser(id: number) {
  return api<{ user: { id: number; userInfo?: { name?: string; email?: string } } }>(
    `/api/v1/users/${id}`,
  )
}
