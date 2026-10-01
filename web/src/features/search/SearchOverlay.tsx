import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { searchUsers } from '@/api/auth'
import { getOrCreateDirect } from '@/api/chat'
import { searchChats, searchMessages } from '@/api/search'
import { useUIStore } from '@/store/uiStore'
import { useQueryClient } from '@tanstack/react-query'

type Tab = 'people' | 'chats' | 'messages'

export function SearchOverlay() {
  const open = useUIStore((s) => s.searchOpen)
  const setOpen = useUIStore((s) => s.setSearchOpen)
  const setActiveChat = useUIStore((s) => s.setActiveChat)
  const showToast = useUIStore((s) => s.showToast)
  const [q, setQ] = useState('')
  const [debounced, setDebounced] = useState('')
  const [tab, setTab] = useState<Tab>('people')
  const qc = useQueryClient()

  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 250)
    return () => clearTimeout(t)
  }, [q])

  const enabled = open && debounced.length >= 2

  const people = useQuery({
    queryKey: ['search-people', debounced],
    queryFn: () => searchUsers(debounced),
    enabled: enabled && tab === 'people',
  })
  const chats = useQuery({
    queryKey: ['search-chats', debounced],
    queryFn: () => searchChats(debounced),
    enabled: enabled && tab === 'chats',
  })
  const messages = useQuery({
    queryKey: ['search-messages', debounced],
    queryFn: () => searchMessages(debounced),
    enabled: enabled && tab === 'messages',
  })

  if (!open) return null

  return (
    <div className="search-overlay" onClick={() => setOpen(false)}>
      <div className="search-panel" onClick={(e) => e.stopPropagation()}>
        <input
          autoFocus
          placeholder="Search…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => e.key === 'Escape' && setOpen(false)}
        />
        <div className="search-tabs">
          {(['people', 'chats', 'messages'] as Tab[]).map((t) => (
            <button
              key={t}
              type="button"
              className={tab === t ? 'active' : ''}
              onClick={() => setTab(t)}
            >
              {t}
            </button>
          ))}
        </div>
        <div style={{ maxHeight: 360, overflow: 'auto' }}>
          {tab === 'people' &&
            (people.data?.users ?? []).map((u) => (
              <button
                key={u.id}
                type="button"
                className="search-hit"
                onClick={async () => {
                  try {
                    const res = await getOrCreateDirect(u.id)
                    await qc.invalidateQueries({ queryKey: ['chats'] })
                    setActiveChat(res.chatId)
                    setOpen(false)
                  } catch {
                    showToast('Could not open chat')
                  }
                }}
              >
                <div style={{ fontWeight: 500 }}>{u.userInfo?.name}</div>
                <div style={{ color: 'var(--text-muted)', fontSize: 13 }}>
                  @{u.userInfo?.username}
                </div>
              </button>
            ))}
          {tab === 'chats' &&
            (chats.data?.hits ?? []).map((h, i) => (
              <button
                key={`${h.chatId}-${i}`}
                type="button"
                className="search-hit"
                onClick={() => {
                  if (h.chatId) setActiveChat(h.chatId)
                  setOpen(false)
                }}
              >
                <div style={{ fontWeight: 500 }}>{h.name || `Chat #${h.chatId}`}</div>
                <div style={{ color: 'var(--text-muted)', fontSize: 13 }}>{h.snippet}</div>
              </button>
            ))}
          {tab === 'messages' &&
            (messages.data?.hits ?? []).map((h, i) => (
              <button
                key={`${h.chatId}-${h.messageId}-${i}`}
                type="button"
                className="search-hit"
                onClick={() => {
                  if (h.chatId) setActiveChat(h.chatId)
                  setOpen(false)
                }}
              >
                <div style={{ fontWeight: 500 }}>Chat #{h.chatId}</div>
                <div style={{ color: 'var(--text-muted)', fontSize: 13 }}>
                  {h.snippet || h.text || 'Message'}
                </div>
              </button>
            ))}
          {enabled &&
            ((tab === 'people' && !people.isFetching && !(people.data?.users ?? []).length) ||
              (tab === 'chats' && !chats.isFetching && !(chats.data?.hits ?? []).length) ||
              (tab === 'messages' &&
                !messages.isFetching &&
                !(messages.data?.hits ?? []).length)) && (
              <div style={{ padding: 16, color: 'var(--text-muted)' }}>No results</div>
            )}
        </div>
      </div>
    </div>
  )
}
