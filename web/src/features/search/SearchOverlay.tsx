import { useEffect, useMemo, useState } from 'react'
import { useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { getMe, getUser, searchUsers, type PublicUser } from '@/api/auth'
import { getOrCreateDirect, getChat, isDirectChat, listChats, type Chat } from '@/api/chat'
import { searchChats, searchMessages } from '@/api/search'
import { useAuthStore } from '@/store/authStore'
import { useUIStore } from '@/store/uiStore'
import { Avatar } from '@/components/Avatar'

type Tab = 'people' | 'chats' | 'messages'

function chatTitle(chat: Chat | undefined, peer?: PublicUser): string {
  if (!chat) return 'Chat'
  if (isDirectChat(chat)) {
    return (
      peer?.userInfo?.name ||
      (peer?.userInfo?.username ? `@${peer.userInfo.username}` : '') ||
      (chat.peerUserId ? `User #${chat.peerUserId}` : 'Direct')
    )
  }
  return chat.chatInfo?.name || `Group #${chat.chatId}`
}

export function SearchOverlay() {
  const open = useUIStore((s) => s.searchOpen)
  const setOpen = useUIStore((s) => s.setSearchOpen)
  const setActiveChat = useUIStore((s) => s.setActiveChat)
  const openChatAtMessage = useUIStore((s) => s.openChatAtMessage)
  const showToast = useUIStore((s) => s.showToast)
  const myId = useAuthStore((s) => s.userId)
  const [q, setQ] = useState('')
  const [debounced, setDebounced] = useState('')
  const [tab, setTab] = useState<Tab>('people')
  const qc = useQueryClient()

  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 250)
    return () => clearTimeout(t)
  }, [q])

  const enabled = open && debounced.length >= 2

  const chatsQuery = useQuery({
    queryKey: ['chats'],
    queryFn: () => listChats(80),
    enabled: open,
    staleTime: 30_000,
  })

  const meQuery = useQuery({
    queryKey: ['me'],
    queryFn: getMe,
    enabled: open,
    staleTime: 60_000,
  })

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

  const messageHits = messages.data?.hits ?? []

  const missingChatIds = useMemo(() => {
    const ids = new Set<number>()
    for (const h of messageHits) if (h.chatId) ids.add(h.chatId)
    for (const h of chats.data?.hits ?? []) if (h.chatId) ids.add(h.chatId)
    const known = new Set((chatsQuery.data?.chats ?? []).map((c) => c.chatId))
    return Array.from(ids).filter((id) => !known.has(id))
  }, [messageHits, chats.data?.hits, chatsQuery.data?.chats])

  const missingChatQueries = useQueries({
    queries: missingChatIds.map((id) => ({
      queryKey: ['chat', id],
      queryFn: () => getChat(id),
      staleTime: 60_000,
      enabled: open && tab === 'messages' && !!id,
    })),
  })

  const chatById = useMemo(() => {
    const map = new Map<number, Chat>()
    for (const c of chatsQuery.data?.chats ?? []) map.set(c.chatId, c)
    missingChatIds.forEach((id, i) => {
      const chat = missingChatQueries[i]?.data?.chat
      if (chat) map.set(id, chat)
    })
    return map
  }, [chatsQuery.data?.chats, missingChatIds, missingChatQueries])

  const enrichUserIds = useMemo(() => {
    const ids = new Set<number>()
    for (const h of messageHits) {
      if (h.senderId) ids.add(h.senderId)
      const chat = chatById.get(h.chatId)
      if (chat && isDirectChat(chat) && chat.peerUserId) ids.add(chat.peerUserId)
    }
    for (const h of chats.data?.hits ?? []) {
      const chat = chatById.get(h.chatId)
      if (chat && isDirectChat(chat) && chat.peerUserId) ids.add(chat.peerUserId)
    }
    ids.delete(Number(myId) || 0)
    return Array.from(ids)
  }, [messageHits, chats.data?.hits, chatById, myId])

  const userQueries = useQueries({
    queries: enrichUserIds.map((id) => ({
      queryKey: ['user', id],
      queryFn: () => getUser(id),
      staleTime: 60_000,
      enabled: open && !!id,
    })),
  })

  const usersById = useMemo(() => {
    const map = new Map<number, PublicUser>()
    enrichUserIds.forEach((id, i) => {
      const u = userQueries[i]?.data?.user
      if (u) map.set(Number(u.id) || id, u)
    })
    return map
  }, [enrichUserIds, userQueries])

  if (!open) return null

  return (
    <div className="search-overlay" onClick={() => setOpen(false)}>
      <div className="search-panel" onClick={(e) => e.stopPropagation()}>
        <input
          autoFocus
          placeholder="Search people, chats, messages…"
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
        <div className="search-results">
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
                <Avatar name={u.userInfo?.name || u.userInfo?.username} fileId={u.avatarFileId} size={42} />
                <div className="search-hit-main">
                  <div className="search-hit-title">{u.userInfo?.name || 'User'}</div>
                  <div className="search-hit-sub">@{u.userInfo?.username}</div>
                </div>
              </button>
            ))}

          {tab === 'chats' &&
            (chats.data?.hits ?? []).map((h, i) => {
              const chat = chatById.get(h.chatId)
              const peer =
                chat && isDirectChat(chat) && chat.peerUserId
                  ? usersById.get(chat.peerUserId)
                  : undefined
              const title = h.name || chatTitle(chat, peer)
              const avatarFileId = isDirectChat(chat)
                ? peer?.avatarFileId
                : chat?.avatarFileId
              return (
                <button
                  key={`${h.chatId}-${i}`}
                  type="button"
                  className="search-hit"
                  onClick={() => {
                    if (h.chatId) setActiveChat(h.chatId)
                    setOpen(false)
                  }}
                >
                  <Avatar name={title} fileId={avatarFileId} size={42} />
                  <div className="search-hit-main">
                    <div className="search-hit-title">{title}</div>
                    <div className="search-hit-sub">
                      {h.description || (isDirectChat(chat) ? 'Direct message' : 'Group')}
                    </div>
                  </div>
                </button>
              )
            })}

          {tab === 'messages' &&
            messageHits.map((h, i) => {
              const chat = chatById.get(h.chatId)
              const sender = usersById.get(h.senderId)
              const peer =
                chat && isDirectChat(chat) && chat.peerUserId
                  ? usersById.get(chat.peerUserId)
                  : undefined
              const inChat = chatTitle(chat, peer)
              const senderName =
                Number(h.senderId) === Number(myId)
                  ? 'You'
                  : sender?.userInfo?.name ||
                    (sender?.userInfo?.username ? `@${sender.userInfo.username}` : `User #${h.senderId}`)
              const me = meQuery.data?.user
              const avatarName =
                Number(h.senderId) === Number(myId)
                  ? me?.userInfo?.name || 'You'
                  : sender?.userInfo?.name || sender?.userInfo?.username || inChat
              const avatarFileId =
                Number(h.senderId) === Number(myId)
                  ? me?.avatarFileId
                  : sender?.avatarFileId ||
                    (isDirectChat(chat) ? peer?.avatarFileId : chat?.avatarFileId)
              const chatLabel = chat ? inChat : `Chat #${h.chatId}`
              return (
                <button
                  key={`${h.chatId}-${h.messageId}-${i}`}
                  type="button"
                  className="search-hit"
                  onClick={() => {
                    if (!h.chatId || !h.messageId) {
                      showToast('Invalid search result')
                      return
                    }
                    openChatAtMessage(h.chatId, h.messageId)
                  }}
                >
                  <Avatar name={avatarName} fileId={avatarFileId} size={42} />
                  <div className="search-hit-main">
                    <div className="search-hit-title">
                      <span>{senderName}</span>
                      <span className="search-hit-chat">in {chatLabel}</span>
                    </div>
                    <div className="search-hit-sub">{h.text || 'Message'}</div>
                  </div>
                </button>
              )
            })}

          {enabled &&
            ((tab === 'people' && !people.isFetching && !(people.data?.users ?? []).length) ||
              (tab === 'chats' && !chats.isFetching && !(chats.data?.hits ?? []).length) ||
              (tab === 'messages' && !messages.isFetching && !messageHits.length)) && (
              <div className="muted-pad">No results</div>
            )}
          {enabled &&
            ((tab === 'people' && people.isFetching) ||
              (tab === 'chats' && chats.isFetching) ||
              (tab === 'messages' && messages.isFetching)) && (
              <div className="muted-pad">Searching…</div>
            )}
        </div>
      </div>
    </div>
  )
}
