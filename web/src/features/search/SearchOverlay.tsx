import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { searchMessages } from '@/api/search'
import { useUIStore } from '@/store/uiStore'

export function SearchOverlay() {
  const open = useUIStore((s) => s.searchOpen)
  const setOpen = useUIStore((s) => s.setSearchOpen)
  const setActiveChat = useUIStore((s) => s.setActiveChat)
  const [q, setQ] = useState('')
  const [debounced, setDebounced] = useState('')

  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 250)
    return () => clearTimeout(t)
  }, [q])

  const { data, isFetching } = useQuery({
    queryKey: ['search', debounced],
    queryFn: () => searchMessages(debounced),
    enabled: open && debounced.length >= 2,
  })

  if (!open) return null

  const hits = data?.hits ?? []

  return (
    <div className="search-overlay" onClick={() => setOpen(false)}>
      <div className="search-panel" onClick={(e) => e.stopPropagation()}>
        <input
          autoFocus
          placeholder="Search messages…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => e.key === 'Escape' && setOpen(false)}
        />
        <div style={{ maxHeight: 360, overflow: 'auto' }}>
          {isFetching && <div style={{ padding: 16, color: 'var(--text-muted)' }}>Searching…</div>}
          {!isFetching && debounced.length >= 2 && !hits.length && (
            <div style={{ padding: 16, color: 'var(--text-muted)' }}>No results</div>
          )}
          {hits.map((h, i) => (
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
        </div>
      </div>
    </div>
  )
}
