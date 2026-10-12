import { useCallback, useEffect, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getMe, logout } from '@/api/auth'
import { useAuthStore } from '@/store/authStore'
import { useUIStore } from '@/store/uiStore'
import { ChatList } from '@/features/chats/ChatList'
import { NewMessageModal } from '@/features/chats/NewMessageModal'
import { NewGroupModal } from '@/features/chats/NewGroupModal'
import { ChatPane } from '@/features/messages/ChatPane'
import { SearchOverlay } from '@/features/search/SearchOverlay'
import { ProfileDrawer } from '@/features/shell/ProfileDrawer'
import { RealtimeClient, applyRealtimeToCache } from '@/ws/client'

export function MessengerShell() {
  const accessToken = useAuthStore((s) => s.accessToken)
  const refreshToken = useAuthStore((s) => s.refreshToken)
  const clear = useAuthStore((s) => s.clear)
  const mobileView = useUIStore((s) => s.mobileView)
  const setNewChatOpen = useUIStore((s) => s.setNewChatOpen)
  const setNewGroupOpen = useUIStore((s) => s.setNewGroupOpen)
  const setSearchOpen = useUIStore((s) => s.setSearchOpen)
  const openProfile = useUIStore((s) => s.openProfile)
  const setUserTyping = useUIStore((s) => s.setUserTyping)
  const clearUserTyping = useUIStore((s) => s.clearUserTyping)
  const setOnlineUsers = useUIStore((s) => s.setOnlineUsers)
  const toast = useUIStore((s) => s.toast)
  const clearToast = useUIStore((s) => s.clearToast)
  const [filter, setFilter] = useState('')
  const [menuOpen, setMenuOpen] = useState(false)
  const qc = useQueryClient()
  const rt = useRef<RealtimeClient | null>(null)
  const typingTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())
  const narrow = useMediaQuery('(max-width: 900px)')

  const me = useQuery({ queryKey: ['me'], queryFn: getMe })

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setSearchOpen(true)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [setSearchOpen])

  useEffect(() => {
    if (!toast) return
    const t = setTimeout(clearToast, 2400)
    return () => clearTimeout(t)
  }, [toast, clearToast])

  useEffect(() => {
    if (!accessToken) return
    const myId = Number(useAuthStore.getState().userId || 0)
    const client = new RealtimeClient(
      () => useAuthStore.getState().accessToken,
      {
        onEvent: (ev) => {
          if (ev.type === 'presence' && ev.users) {
            const mapped: Record<number, boolean> = {}
            for (const [k, v] of Object.entries(ev.users)) {
              mapped[Number(k)] = !!v
            }
            setOnlineUsers({ ...useUIStore.getState().onlineUsers, ...mapped })
            return
          }
          if (ev.type === 'typing' && ev.chat_id) {
            const chatId = Number(ev.chat_id)
            const userId = Number(ev.user_id || 0)
            if (!chatId || !userId || userId === myId) return
            setUserTyping(chatId, userId)
            const key = `${chatId}:${userId}`
            const prev = typingTimers.current.get(key)
            if (prev) clearTimeout(prev)
            typingTimers.current.set(
              key,
              setTimeout(() => {
                clearUserTyping(chatId, userId)
                typingTimers.current.delete(key)
              }, 3000),
            )
            return
          }
          if (ev.type === 'chat.realtime' && ev.chat_id) {
            const chatId = Number(ev.chat_id)
            const actorId = Number(ev.actor_id || ev.message?.sender_id || 0)
            if (ev.event === 'message_created' && chatId && actorId) {
              clearUserTyping(chatId, actorId)
              const key = `${chatId}:${actorId}`
              const t = typingTimers.current.get(key)
              if (t) {
                clearTimeout(t)
                typingTimers.current.delete(key)
              }
            }
            applyRealtimeToCache(qc, ev)
          }
        },
      },
    )
    rt.current = client
    client.connect()
    return () => {
      client.close()
      for (const t of typingTimers.current.values()) clearTimeout(t)
      typingTimers.current.clear()
    }
  }, [accessToken, qc, setUserTyping, clearUserTyping, setOnlineUsers])

  const onTyping = useCallback((chatId: number) => {
    rt.current?.sendTyping(chatId)
  }, [])

  const onPresenceSubscribe = useCallback((userIds: number[]) => {
    rt.current?.subscribePresence(userIds)
  }, [])

  const shellClass =
    narrow && mobileView === 'chat'
      ? 'shell mobile-chat'
      : narrow && mobileView === 'list'
        ? 'shell mobile-list'
        : 'shell'

  return (
    <div className={shellClass}>
      <aside className="sidebar">
        <div className="sidebar-brand">
          <div className="sidebar-brand-mark">M</div>
          <div>
            <div className="sidebar-brand-title">Messanger</div>
            {me.data?.user?.userInfo?.username ? (
              <div className="sidebar-brand-sub">@{me.data.user.userInfo.username}</div>
            ) : (
              <div className="sidebar-brand-sub">Your conversations</div>
            )}
          </div>
        </div>
        <div className="sidebar-top">
          <button
            type="button"
            className="icon-btn"
            title="Profile"
            onClick={() => {
              const id = useAuthStore.getState().userId || me.data?.user?.id
              if (id) openProfile(Number(id))
            }}
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden>
              <circle cx="12" cy="8" r="3.5" stroke="currentColor" strokeWidth="1.8" />
              <path
                d="M5 19.5c1.8-3.2 4.2-4.8 7-4.8s5.2 1.6 7 4.8"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
              />
            </svg>
          </button>
          <input
            className="search-input"
            placeholder="Search people & chats"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            onFocus={() => setSearchOpen(true)}
          />
          <div className="menu-wrap">
            <button
              type="button"
              className="icon-btn"
              title="New"
              onClick={() => setMenuOpen((v) => !v)}
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden>
                <path
                  d="M12 5v14M5 12h14"
                  stroke="currentColor"
                  strokeWidth="1.9"
                  strokeLinecap="round"
                />
              </svg>
            </button>
            {menuOpen && (
              <div className="dropdown">
                <button
                  type="button"
                  onClick={() => {
                    setMenuOpen(false)
                    setNewChatOpen(true)
                  }}
                >
                  New message
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setMenuOpen(false)
                    setNewGroupOpen(true)
                  }}
                >
                  New group
                </button>
              </div>
            )}
          </div>
          <button
            type="button"
            className="icon-btn"
            title="Logout"
            onClick={async () => {
              if (refreshToken) await logout(refreshToken)
              clear()
            }}
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden>
              <path
                d="M10 7V5a2 2 0 0 1 2-2h7v18h-7a2 2 0 0 1-2-2v-2"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
              />
              <path
                d="M4 12h10M10 8l4 4-4 4"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </button>
        </div>
        <ChatList filter={filter} />
      </aside>
      <ChatPane onTyping={onTyping} onPresenceSubscribe={onPresenceSubscribe} />
      <NewMessageModal />
      <NewGroupModal />
      <SearchOverlay />
      <ProfileDrawer />
      {toast && <div className="toast">{toast}</div>}
    </div>
  )
}

function useMediaQuery(query: string) {
  const [match, setMatch] = useState(() =>
    typeof window !== 'undefined' ? window.matchMedia(query).matches : false,
  )
  useEffect(() => {
    const m = window.matchMedia(query)
    const fn = () => setMatch(m.matches)
    fn()
    m.addEventListener('change', fn)
    return () => m.removeEventListener('change', fn)
  }, [query])
  return match
}
