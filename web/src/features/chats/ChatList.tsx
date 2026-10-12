import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { getUser } from '@/api/auth'
import { listChats, isDirectChat, pinChat, type Chat } from '@/api/chat'
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

function sortChats(chats: Chat[]): Chat[] {
  return [...chats].sort((a, b) => {
    if (!!a.isPinned !== !!b.isPinned) return a.isPinned ? -1 : 1
    const at = a.lastMessageAt ? new Date(a.lastMessageAt).getTime() : 0
    const bt = b.lastMessageAt ? new Date(b.lastMessageAt).getTime() : 0
    return bt - at
  })
}

function ChatRow({
  chat,
  active,
  peerName,
  peerUsername,
  peerAvatarFileId,
  peerId,
  onTogglePin,
  pinPending,
}: {
  chat: Chat
  active: boolean
  peerName?: string
  peerUsername?: string
  peerAvatarFileId?: string
  peerId?: number
  onTogglePin: () => void
  pinPending: boolean
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
      className={`chat-item${active ? ' active' : ''}${chat.isPinned ? ' pinned' : ''}`}
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
        <div className="chat-item-title">
          {chat.isPinned ? <span className="chat-pin-mark" aria-hidden /> : null}
          {title}
        </div>
        <div className="chat-item-preview">{preview}</div>
      </div>
      <div className="chat-item-meta">
        <span className="chat-item-time">{formatTime(chat.lastMessageAt)}</span>
        <div className="chat-item-meta-row">
          <button
            type="button"
            className={`chat-pin-btn${chat.isPinned ? ' on' : ''}`}
            title={chat.isPinned ? 'Unpin chat' : 'Pin chat'}
            disabled={pinPending}
            onClick={(e) => {
              e.stopPropagation()
              onTogglePin()
            }}
          >
            {chat.isPinned ? 'Unpin' : 'Pin'}
          </button>
          {!!chat.unreadCount && chat.unreadCount > 0 && (
            <span className="badge">{chat.unreadCount > 99 ? '99+' : chat.unreadCount}</span>
          )}
        </div>
      </div>
    </button>
  )
}

export function ChatList({ filter }: { filter: string }) {
  const qc = useQueryClient()
  const activeChatId = useUIStore((s) => s.activeChatId)
  const myId = useAuthStore((s) => s.userId)
  const { data, isLoading } = useQuery({
    queryKey: ['chats'],
    queryFn: () => listChats(80),
  })

  const pinMut = useMutation({
    mutationFn: ({ chatId, isPinned }: { chatId: number; isPinned: boolean }) =>
      pinChat(chatId, isPinned),
    onMutate: async ({ chatId, isPinned }) => {
      await qc.cancelQueries({ queryKey: ['chats'] })
      const prev = qc.getQueryData<{ chats?: Chat[] }>(['chats'])
      qc.setQueryData<{ chats?: Chat[] }>(['chats'], (old) => {
        if (!old?.chats) return old
        return {
          ...old,
          chats: sortChats(
            old.chats.map((c) => (c.chatId === chatId ? { ...c, isPinned } : c)),
          ),
        }
      })
      return { prev }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) qc.setQueryData(['chats'], ctx.prev)
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: ['chats'] })
    },
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

  const filtered = sortChats(
    chats.filter((c) => {
      if (!filter.trim()) return true
      const peer = c.peerUserId ? peerMap.get(c.peerUserId) : undefined
      const title = chatTitle(c, peer?.name)
      const hay = `${title} ${peer?.username || ''} ${c.lastMessagePreview || ''}`.toLowerCase()
      return hay.includes(filter.toLowerCase())
    }),
  )

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
            pinPending={pinMut.isPending && pinMut.variables?.chatId === c.chatId}
            onTogglePin={() =>
              pinMut.mutate({ chatId: c.chatId, isPinned: !c.isPinned })
            }
          />
        )
      })}
    </div>
  )
}
