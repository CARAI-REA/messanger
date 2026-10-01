import { useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { logout } from '@/api/auth'
import { useAuthStore } from '@/store/authStore'
import { useUIStore } from '@/store/uiStore'
import { ChatList } from '@/features/chats/ChatList'
import { NewChatModal } from '@/features/chats/NewChatModal'
import { ChatPane } from '@/features/messages/ChatPane'
import { SearchOverlay } from '@/features/search/SearchOverlay'
import { RealtimeClient, payloadLooksLikeMessageCreated } from '@/ws/client'

export function MessengerShell() {
  const accessToken = useAuthStore((s) => s.accessToken)
  const refreshToken = useAuthStore((s) => s.refreshToken)
  const clear = useAuthStore((s) => s.clear)
  const mobileView = useUIStore((s) => s.mobileView)
  const setNewChatOpen = useUIStore((s) => s.setNewChatOpen)
  const setSearchOpen = useUIStore((s) => s.setSearchOpen)
  const setTypingChat = useUIStore((s) => s.setTypingChat)
  const toast = useUIStore((s) => s.toast)
  const clearToast = useUIStore((s) => s.clearToast)
  const [filter, setFilter] = useState('')
  const qc = useQueryClient()
  const rt = useRef<RealtimeClient | null>(null)
  const typingTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const narrow = useMediaQuery('(max-width: 900px)')

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
          if (ev.type === 'typing' && ev.chat_id) {
            if (ev.user_id && ev.user_id === myId) return
            setTypingChat(ev.chat_id)
            if (typingTimer.current) clearTimeout(typingTimer.current)
            typingTimer.current = setTimeout(() => setTypingChat(null), 3000)
            return
          }
          if (ev.type === 'chat.realtime' && ev.chat_id) {
            void qc.invalidateQueries({ queryKey: ['messages', ev.chat_id] })
            void qc.invalidateQueries({ queryKey: ['chats'] })
            void payloadLooksLikeMessageCreated(ev.payload)
          }
        },
      },
    )
    rt.current = client
    client.connect()
    return () => client.close()
  }, [accessToken, qc, setTypingChat])

  function onTyping(chatId: number) {
    rt.current?.sendTyping(chatId)
  }

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
          <input
            className="search-input"
            placeholder="Search"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            onFocus={() => setSearchOpen(true)}
          />
          <button type="button" className="icon-btn" title="New chat" onClick={() => setNewChatOpen(true)}>
            ✎
          </button>
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
        <ChatList filter={filter} />
      </aside>
      <ChatPane onTyping={onTyping} />
      <NewChatModal />
      <SearchOverlay />
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
