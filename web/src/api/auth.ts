import { api, decodeJwtUserId } from './http'

export type PublicUser = {
  id: number
  userInfo?: { name?: string; email?: string; username?: string }
  avatarFileId?: string
}

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

export async function register(
  name: string,
  username: string,
  email: string,
  password: string,
) {
  await api(
    '/api/v1/users',
    {
      method: 'POST',
      body: JSON.stringify({
        userInfo: { name, email, username },
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
  const data = await api<{ user: PublicUser }>(`/api/v1/users/${id}`)
  if (data?.user) {
    data.user.id = Number(data.user.id) || data.user.id
    if (!data.user.avatarFileId) data.user.avatarFileId = undefined
  }
  return data
}

export async function getMe() {
  const data = await api<{ user: PublicUser }>('/api/v1/users/me')
  if (data?.user) {
    data.user.id = Number(data.user.id) || data.user.id
    if (!data.user.avatarFileId) data.user.avatarFileId = undefined
  }
  return data
}

export async function getByUsername(username: string) {
  const u = username.replace(/^@/, '')
  return api<{ user: PublicUser }>(`/api/v1/users/by-username/${encodeURIComponent(u)}`)
}

export async function searchUsers(query: string, limit = 20) {
  const q = new URLSearchParams({ query, limit: String(limit) })
  const data = await api<{ users?: PublicUser[] }>(`/api/v1/users:search?${q}`)
  data.users = (data.users ?? []).map((u) => ({
    ...u,
    id: Number(u.id) || u.id,
    avatarFileId: u.avatarFileId || undefined,
  }))
  return data
}

export async function updateMe(patch: {
  name?: string
  username?: string
  avatarFileId?: string
}) {
  const body: Record<string, string> = {}
  if (patch.name !== undefined) body.name = patch.name
  if (patch.username !== undefined) body.username = patch.username
  if (patch.avatarFileId !== undefined) body.avatarFileId = patch.avatarFileId
  return api('/api/v1/users/me', { method: 'PATCH', body: JSON.stringify(body) })
}
