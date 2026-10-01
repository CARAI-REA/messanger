import { create } from 'zustand'
import { bindAuthStore, decodeJwtUserId } from '@/api/http'

type AuthState = {
  accessToken: string | null
  refreshToken: string | null
  userId: number
  hydrated: boolean
  hydrate: () => void
  setSession: (access: string, refresh: string, userId?: number) => void
  setTokens: (access: string, refresh: string) => void
  clear: () => void
}

const KEY = 'messanger.auth'

function setAccessCookie(token: string | null) {
  if (!token) {
    document.cookie = 'access_token=; path=/; Max-Age=0; SameSite=Lax'
    return
  }
  document.cookie = `access_token=${encodeURIComponent(token)}; path=/; SameSite=Lax`
}

export const useAuthStore = create<AuthState>((set, get) => ({
  accessToken: null,
  refreshToken: null,
  userId: 0,
  hydrated: false,
  hydrate: () => {
    try {
      const raw = localStorage.getItem(KEY)
      if (raw) {
        const parsed = JSON.parse(raw) as { accessToken: string; refreshToken: string; userId: number }
        setAccessCookie(parsed.accessToken)
        set({
          accessToken: parsed.accessToken,
          refreshToken: parsed.refreshToken,
          userId: parsed.userId || decodeJwtUserId(parsed.accessToken),
          hydrated: true,
        })
        return
      }
    } catch {
      /* ignore */
    }
    set({ hydrated: true })
  },
  setSession: (access, refresh, userId) => {
    const uid = userId || decodeJwtUserId(access)
    localStorage.setItem(KEY, JSON.stringify({ accessToken: access, refreshToken: refresh, userId: uid }))
    setAccessCookie(access)
    set({ accessToken: access, refreshToken: refresh, userId: uid })
  },
  setTokens: (access, refresh) => {
    const uid = get().userId || decodeJwtUserId(access)
    localStorage.setItem(KEY, JSON.stringify({ accessToken: access, refreshToken: refresh, userId: uid }))
    setAccessCookie(access)
    set({ accessToken: access, refreshToken: refresh, userId: uid })
  },
  clear: () => {
    localStorage.removeItem(KEY)
    setAccessCookie(null)
    set({ accessToken: null, refreshToken: null, userId: 0 })
  },
}))

bindAuthStore(() => ({
  accessToken: useAuthStore.getState().accessToken,
  refreshToken: useAuthStore.getState().refreshToken,
  userId: useAuthStore.getState().userId,
  setTokens: useAuthStore.getState().setTokens,
  clear: useAuthStore.getState().clear,
}))
