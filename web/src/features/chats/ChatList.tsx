import { useQueries, useQuery } from '@tanstack/react-query'
import { getUser } from '@/api/auth'
import { listChats, isDirectChat, type Chat } from '@/api/chat'
import { useAuthStore } from '@/store/authStore'
import { useUIStore } from '@/store/uiStore'
import { formatTime } from '@/lib/format'
import { Avatar } from '@/components/Avatar'

function chatTitle(chat: Chat, peerName?: string): string {
  if (isDirectChat(chat)) {
    return peerName || (chat.peerUserId ? `User #${chat.peerUserId}` : 'Direct')
  }
  return chat.chatInfo?.name || `Group #${chat.chatId}`
}

function ChatRow({
  chat,
  active,
  peerName,
  peerUsername,
  peerAvatarFileId,
  peerId,
}: {
  chat: Chat
  active: boolean
  peerName?: string
  peerUsername?: string
  peerAvatarFileId?: string
  peerId?: number
}) {
  const setActive = useUIStore((s) => s.setActiveChat)
  const openProfile = useUIStore((s) => s.openProfile)
  const title = chatTitle(chat, peerName)
  const preview =
    chat.lastMessagePreview ||
    (isDirectChat(chat) ? (peerUsername ? `@${peerUsername}` : 'Start chatting') : 'Group chat')

  return (
    <button
      type="button"
      className={`chat-item${active ? ' active' : ''}`}
      onClick={() => setActive(chat.chatId)}
    >
      <Avatar
        name={title}
        fileId={isDirectChat(chat) ? peerAvatarFileId : chat.avatarFileId}
        size={48}
        onClick={
          isDirectChat(chat) && peerId
            ? (e) => {
                e.stopPropagation()
                openProfile(peerId)
              }
            : undefined
        }
      />
      <div className="chat-item-main">
        <div className="chat-item-title">{title}</div>
        <div className="chat-item-preview">{preview}</div>
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
  const myId = useAuthStore((s) => s.userId)
  const { data, isLoading } = useQuery({
    queryKey: ['chats'],
    queryFn: () => listChats(80),
  })

  const chats = data?.chats ?? []
  const peerIds = Array.from(
    new Set(
      chats
        .filter(isDirectChat)
        .map((c) => c.peerUserId)
        .filter((id): id is number => !!id && id !== myId),
    ),
  )

  const peerQueries = useQueries({
    queries: peerIds.map((id) => ({
      queryKey: ['user', id],
      queryFn: () => getUser(id),
      staleTime: 60_000,
    })),
  })

  const peerMap = new Map<
    number,
    { name?: string; username?: string; avatarFileId?: string }
  >()
  peerIds.forEach((id, i) => {
    const u = peerQueries[i]?.data?.user
    if (u) {
      peerMap.set(id, {
        name: u.userInfo?.name,
        username: u.userInfo?.username,
        avatarFileId: u.avatarFileId || undefined,
      })
    }
  })

  const filtered = chats.filter((c) => {
    if (!filter.trim()) return true
    const peer = c.peerUserId ? peerMap.get(c.peerUserId) : undefined
    const title = chatTitle(c, peer?.name)
    const hay = `${title} ${peer?.username || ''} ${c.lastMessagePreview || ''}`.toLowerCase()
    return hay.includes(filter.toLowerCase())
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

  if (!filtered.length) {
    return (
      <div className="chat-list" style={{ padding: 24, color: 'var(--text-muted)' }}>
        No chats yet. Tap ✎ to message someone by @username.
      </div>
    )
  }

  return (
    <div className="chat-list">
      {filtered.map((c) => {
        const peer = c.peerUserId ? peerMap.get(c.peerUserId) : undefined
        return (
          <ChatRow
            key={c.chatId}
            chat={c}
            active={c.chatId === activeChatId}
            peerName={peer?.name}
            peerUsername={peer?.username}
            peerAvatarFileId={peer?.avatarFileId}
            peerId={c.peerUserId}
          />
        )
      })}
    </div>
  )
}
