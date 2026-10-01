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
  const setTypingChat = useUIStore((s) => s.setTypingChat)
  const setOnlineUsers = useUIStore((s) => s.setOnlineUsers)
  const toast = useUIStore((s) => s.toast)
  const clearToast = useUIStore((s) => s.clearToast)
  const [filter, setFilter] = useState('')
  const [menuOpen, setMenuOpen] = useState(false)
  const qc = useQueryClient()
  const rt = useRef<RealtimeClient | null>(null)
  const typingTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
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
    const myId = useAuthStore.getState().userId
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
            if (ev.user_id && ev.user_id === myId) return
            setTypingChat(ev.chat_id)
            if (typingTimer.current) clearTimeout(typingTimer.current)
            typingTimer.current = setTimeout(() => setTypingChat(null), 3000)
            return
          }
          if (ev.type === 'chat.realtime' && ev.chat_id) {
            applyRealtimeToCache(qc, ev)
          }
        },
      },
    )
    rt.current = client
    client.connect()
    return () => client.close()
  }, [accessToken, qc, setTypingChat, setOnlineUsers])

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
            ●
          </button>
          <input
            className="search-input"
            placeholder="Search"
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
              ✎
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
            ⎋
          </button>
        </div>
        {me.data?.user?.userInfo?.username && (
          <div className="sidebar-me">@{me.data.user.userInfo.username}</div>
        )}
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
