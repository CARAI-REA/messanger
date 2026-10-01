import { useQuery } from '@tanstack/react-query'
import { listChats, type Chat } from '@/api/chat'
import { useUIStore } from '@/store/uiStore'
import { formatTime, initials } from '@/lib/format'

function ChatRow({ chat, active }: { chat: Chat; active: boolean }) {
  const setActive = useUIStore((s) => s.setActiveChat)
  const title = chat.chatInfo?.name || `Chat #${chat.chatId}`
  return (
    <button
      type="button"
      className={`chat-item${active ? ' active' : ''}`}
      onClick={() => setActive(chat.chatId)}
    >
      <div className="avatar">{initials(title)}</div>
      <div className="chat-item-main">
        <div className="chat-item-title">{title}</div>
        <div className="chat-item-preview">
          {chat.chatInfo?.description || 'Tap to open conversation'}
        </div>
      </div>
      <div className="chat-item-meta">
        <span className="chat-item-time">{formatTime(chat.lastMessageAt)}</span>
        {!!chat.unreadCount && chat.unreadCount > 0 && (
          <span className="badge">{chat.unreadCount > 99 ? '99+' : chat.unreadCount}</span>
        )}
      </div>
    </button>
  )
}

export function ChatList({ filter }: { filter: string }) {
  const activeChatId = useUIStore((s) => s.activeChatId)
  const { data, isLoading } = useQuery({
    queryKey: ['chats'],
    queryFn: () => listChats(80),
  })

  const chats = (data?.chats ?? []).filter((c) => {
    if (!filter.trim()) return true
    const title = c.chatInfo?.name || String(c.chatId)
    return title.toLowerCase().includes(filter.toLowerCase())
  })

  if (isLoading) {
    return (
      <div className="chat-list" style={{ padding: 12, display: 'grid', gap: 10 }}>
        {[1, 2, 3, 4, 5].map((i) => (
          <div key={i} className="skeleton" style={{ height: 64 }} />
        ))}
      </div>
    )
  }

  if (!chats.length) {
    return (
      <div className="chat-list" style={{ padding: 24, color: 'var(--text-muted)' }}>
        No chats yet. Create one with the + button.
      </div>
    )
  }

  return (
    <div className="chat-list">
      {chats.map((c) => (
        <ChatRow key={c.chatId} chat={c} active={c.chatId === activeChatId} />
      ))}
    </div>
  )
}
