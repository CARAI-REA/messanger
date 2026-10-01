import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import {
  useInfiniteQuery,
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
  type InfiniteData,
} from '@tanstack/react-query'
import {
  deleteMessage,
  editMessage,
  getChat,
  isDirectChat,
  listMessages,
  markRead,
  pinMessage,
  sendMessage,
  type Message,
} from '@/api/chat'
import { getMe, getUser, type PublicUser } from '@/api/auth'
import { initUpload, completeUpload } from '@/api/media'
import { useAuthStore } from '@/store/authStore'
import { useUIStore } from '@/store/uiStore'
import { dayLabel, formatTime } from '@/lib/format'
import { Avatar } from '@/components/Avatar'
import { ChatInfoPanel } from '@/features/chats/ChatInfoPanel'
import { MessageAttachments } from '@/features/messages/MessageAttachments'
import { upsertChatMessage } from '@/ws/client'

const PAGE_SIZE = 20

type MessagesPages = InfiniteData<{ messages: Message[]; hasMore: boolean }, number | undefined>

export function ChatPane({
  onTyping,
  onPresenceSubscribe,
}: {
  onTyping: (chatId: number) => void
  onPresenceSubscribe: (userIds: number[]) => void
}) {
  const chatId = useUIStore((s) => s.activeChatId)
  const typingChatId = useUIStore((s) => s.typingChatId)
  const setMobileView = useUIStore((s) => s.setMobileView)
  const showToast = useUIStore((s) => s.showToast)
  const infoOpen = useUIStore((s) => s.infoOpen)
  const setInfoOpen = useUIStore((s) => s.setInfoOpen)
  const openProfile = useUIStore((s) => s.openProfile)
  const replyToId = useUIStore((s) => s.replyToId)
  const setReplyTo = useUIStore((s) => s.setReplyTo)
  const onlineUsers = useUIStore((s) => s.onlineUsers)
  const myId = useAuthStore((s) => s.userId)
  const qc = useQueryClient()
  const [text, setText] = useState('')
  const [menuId, setMenuId] = useState<number | null>(null)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [jumpToId, setJumpToId] = useState<number | null>(null)
  const [highlightId, setHighlightId] = useState<number | null>(null)
  const [pinIndex, setPinIndex] = useState(0)
  const listRef = useRef<HTMLDivElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const fileRef = useRef<HTMLInputElement>(null)
  const stickBottom = useRef(true)
  const historyReady = useRef(false)
  const loadingOlder = useRef(false)
  const olderScrollHeight = useRef(0)

  const pinToBottom = () => {
    const el = listRef.current
    if (!el || !stickBottom.current) return
    el.scrollTop = el.scrollHeight
  }

  const scrollToMessage = (messageId: number) => {
    const container = listRef.current
    if (!container) return false
    const el = container.querySelector(`[data-mid="${messageId}"]`) as HTMLElement | null
    if (!el) return false
    stickBottom.current = false
    historyReady.current = true
    const cRect = container.getBoundingClientRect()
    const eRect = el.getBoundingClientRect()
    const next =
      container.scrollTop + (eRect.top - cRect.top) - Math.min(120, container.clientHeight / 4)
    container.scrollTo({ top: Math.max(0, next), behavior: 'smooth' })
    setHighlightId(messageId)
    window.setTimeout(() => setHighlightId((cur) => (cur === messageId ? null : cur)), 1800)
    return true
  }

  const chatQuery = useQuery({
    queryKey: ['chat', chatId],
    queryFn: () => getChat(chatId!),
    enabled: !!chatId,
  })

  const peerId = chatQuery.data?.chat.peerUserId
  const peerQuery = useQuery({
    queryKey: ['user', peerId],
    queryFn: () => getUser(peerId!),
    enabled: !!peerId && isDirectChat(chatQuery.data?.chat),
  })

  const meQuery = useQuery({
    queryKey: ['me'],
    queryFn: getMe,
    staleTime: 60_000,
  })

  useEffect(() => {
    const ids = chatQuery.data?.participantIds?.filter((id) => id !== myId) ?? []
    if (ids.length) onPresenceSubscribe(ids)
  }, [chatQuery.data?.participantIds, myId, onPresenceSubscribe])

  // Open with only the newest page — older history loads on scroll up.
  useEffect(() => {
    if (!chatId) return
    stickBottom.current = true
    historyReady.current = false
    loadingOlder.current = false
    setPinIndex(0)
    setJumpToId(null)
    setHighlightId(null)
    setMenuId(null)
    setEditingId(null)
    setText('')
    qc.setQueryData<MessagesPages>(['messages', chatId], (old) => {
      if (!old?.pages?.length) return old
      return {
        pages: [old.pages[0]],
        pageParams: [old.pageParams[0]],
      }
    })
  }, [chatId, qc])

  const messagesQuery = useInfiniteQuery({
    queryKey: ['messages', chatId],
    enabled: !!chatId,
    initialPageParam: undefined as number | undefined,
    queryFn: ({ pageParam }) => listMessages(chatId!, PAGE_SIZE, pageParam),
    getNextPageParam: (last) => {
      if (!last.hasMore || !last.messages?.length) return undefined
      const oldest = last.messages.reduce((m, x) => Math.min(m, x.messageId), Infinity)
      return Number.isFinite(oldest) ? oldest : undefined
    },
    staleTime: 30_000,
  })

  const messages = useMemo(() => {
    const pages = messagesQuery.data?.pages ?? []
    const flat = pages.flatMap((p) => p.messages ?? [])
    const map = new Map<number, Message>()
    for (const m of flat) map.set(Number(m.messageId), { ...m, messageId: Number(m.messageId) })
    return Array.from(map.values()).sort((a, b) => Number(a.messageId) - Number(b.messageId))
  }, [messagesQuery.data])

  const byId = useMemo(() => new Map(messages.map((m) => [m.messageId, m])), [messages])
  const pinned = useMemo(() => messages.filter((m) => m.isPinned), [messages])
  const lastMessage = messages[messages.length - 1]
  const lastMessageId = lastMessage?.messageId

  const senderIds = useMemo(() => {
    const ids = new Set<number>()
    for (const id of chatQuery.data?.participantIds ?? []) {
      const n = Number(id)
      if (n) ids.add(n)
    }
    for (const m of messages) {
      const n = Number(m.senderId)
      if (n) ids.add(n)
    }
    if (peerId) ids.add(peerId)
    if (myId) ids.add(myId)
    return Array.from(ids)
  }, [chatQuery.data?.participantIds, messages, myId, peerId])

  const senderQueries = useQueries({
    queries: senderIds.map((id) => ({
      queryKey: ['user', id],
      queryFn: () => getUser(id),
      staleTime: 60_000,
      enabled: !!id && id !== myId,
    })),
  })

  const sendersById = useMemo(() => {
    const map = new Map<number, PublicUser>()
    senderIds.forEach((id, i) => {
      const u = senderQueries[i]?.data?.user
      if (u) map.set(Number(u.id) || id, u)
    })
    const me = meQuery.data?.user
    if (me && myId) {
      map.set(myId, {
        ...me,
        id: myId,
        avatarFileId: me.avatarFileId || undefined,
      })
    }
    return map
  }, [senderIds, senderQueries, meQuery.data?.user, myId])

  // Glue to bottom while opening / receiving / sending; re-pin when content grows.
  useLayoutEffect(() => {
    const root = listRef.current
    const content = contentRef.current
    if (!root || !content || !chatId) return
    if (jumpToId || loadingOlder.current) return

    stickBottom.current = true

    const pin = () => {
      if (!stickBottom.current) return
      root.scrollTop = root.scrollHeight
    }

    pin()
    const ro = new ResizeObserver(() => pin())
    ro.observe(content)
    const t = window.setTimeout(() => {
      pin()
      historyReady.current = true
    }, 0)
    return () => {
      ro.disconnect()
      window.clearTimeout(t)
    }
  }, [chatId, lastMessageId, messagesQuery.isFetched, jumpToId])

  // Preserve viewport when older pages are prepended.
  useLayoutEffect(() => {
    if (!loadingOlder.current || !listRef.current) return
    const el = listRef.current
    el.scrollTop = el.scrollTop + (el.scrollHeight - olderScrollHeight.current)
    loadingOlder.current = false
  }, [messagesQuery.data?.pages.length])

  useEffect(() => {
    if (!jumpToId) return
    if (scrollToMessage(jumpToId)) {
      setJumpToId(null)
      return
    }
    if (messagesQuery.hasNextPage && !messagesQuery.isFetchingNextPage) {
      const el = listRef.current
      if (el) {
        loadingOlder.current = true
        olderScrollHeight.current = el.scrollHeight
      }
      void messagesQuery.fetchNextPage()
      return
    }
    if (!messagesQuery.isFetchingNextPage) {
      showToast('Pinned message not in loaded history')
      setJumpToId(null)
    }
  }, [
    jumpToId,
    messages,
    messagesQuery.hasNextPage,
    messagesQuery.isFetchingNextPage,
    messagesQuery,
    showToast,
  ])

  useEffect(() => {
    if (!chatId || !lastMessageId || !messages.length) return
    const last = messages[messages.length - 1]
    if (last.senderId !== myId) {
      markRead(chatId, last.messageId).catch(() => undefined)
    }
  }, [chatId, lastMessageId, myId, messages])

  const sendMut = useMutation({
    mutationFn: async (payload: {
      text: string
      attachmentIds?: string[]
      replyTo?: number
    }) => {
      const key = crypto.randomUUID()
      return sendMessage(
        chatId!,
        payload.text,
        key,
        payload.attachmentIds ?? [],
        payload.replyTo,
      ).then((res) => ({ ...res, ...payload }))
    },
    onSuccess: (res) => {
      setReplyTo(null)
      stickBottom.current = true
      if (res.messageId && chatId && myId) {
        upsertChatMessage(qc, {
          messageId: res.messageId,
          senderId: myId,
          chatId,
          text: res.text,
          attachmentIds: res.attachmentIds ?? [],
          replyToMessageId: res.replyTo,
          sendAt: new Date().toISOString(),
        })
      }
      qc.invalidateQueries({ queryKey: ['chats'] })
      requestAnimationFrame(() => pinToBottom())
    },
    onError: () => showToast('Failed to send'),
  })

  if (!chatId) {
    return (
      <div className="chat-pane">
        <div className="empty-pane">
          <div className="mark">M</div>
          <h2>Messanger</h2>
          <p>Select a chat to start messaging</p>
        </div>
      </div>
    )
  }

  const chat = chatQuery.data?.chat
  const direct = isDirectChat(chat)
  const title = direct
    ? peerQuery.data?.user?.userInfo?.name ||
      (peerQuery.data?.user?.userInfo?.username
        ? `@${peerQuery.data.user.userInfo.username}`
        : `Chat #${chatId}`)
    : chat?.chatInfo?.name || `Group #${chatId}`

  const peerOnline = peerId ? !!onlineUsers[peerId] : false
  const typing = typingChatId === chatId
  const subtitle = typing ? 'typing…' : direct ? (peerOnline ? 'online' : 'offline') : 'group'

  const replyMsg = replyToId ? byId.get(replyToId) : undefined

  async function onSend() {
    const t = text.trim()
    if ((!t && !editingId) || sendMut.isPending) return
    if (editingId) {
      try {
        await editMessage(editingId, t)
        setEditingId(null)
        setText('')
        qc.invalidateQueries({ queryKey: ['messages', chatId] })
      } catch {
        showToast('Edit failed')
      }
      return
    }
    stickBottom.current = true
    setText('')
    sendMut.mutate({ text: t, replyTo: replyToId ?? undefined })
  }

  async function onAttach(file: File) {
    try {
      stickBottom.current = true
      const mime = file.type || 'application/octet-stream'
      const init = await initUpload(file.name, mime, file.size)
      if (!init.putUrl || !init.fileId) throw new Error('init upload failed')
      const put = await fetch(init.putUrl, {
        method: 'PUT',
        body: file,
        headers: { 'content-type': mime },
      })
      if (!put.ok) throw new Error(`upload ${put.status}`)
      await completeUpload(init.fileId)
      const caption = text.trim()
      setText('')
      sendMut.mutate({
        text: caption,
        attachmentIds: [init.fileId],
        replyTo: replyToId ?? undefined,
      })
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Upload failed'
      showToast(msg)
    }
  }

  return (
    <div className="chat-pane">
      <div className="chat-header">
        <button
          type="button"
          className="icon-btn"
          onClick={() => setMobileView('list')}
          title="Back"
        >
          ←
        </button>
        <Avatar
          name={title}
          fileId={direct ? peerQuery.data?.user?.avatarFileId : chat?.avatarFileId}
          size={42}
          onClick={(e) => {
            e.stopPropagation()
            if (direct && peerId) openProfile(peerId)
            else setInfoOpen(true)
          }}
        />
        <div
          style={{ flex: 1, minWidth: 0, cursor: 'pointer' }}
          onClick={() => setInfoOpen(true)}
        >
          <h2>{title}</h2>
          <div className="sub">{subtitle}</div>
        </div>
      </div>

      {!!pinned.length && (
        <button
          type="button"
          className="pinned-strip"
          title="Go to pinned message"
          onClick={() => {
            if (!pinned.length) return
            const idx = pinIndex % pinned.length
            const targetId = pinned[idx].messageId
            setPinIndex((idx + 1) % pinned.length)
            if (!scrollToMessage(targetId)) {
              setJumpToId(targetId)
            }
          }}
        >
          📌 {(pinned[pinIndex % pinned.length]?.text || 'Pinned').slice(0, 80)}
          {pinned.length > 1 ? ` (${(pinIndex % pinned.length) + 1}/${pinned.length})` : ''}
        </button>
      )}

      <div
        className="messages"
        ref={listRef}
        onScroll={() => {
          const el = listRef.current
          if (!el) return
          const dist = el.scrollHeight - el.scrollTop - el.clientHeight
          stickBottom.current = dist < 72
          if (!historyReady.current || loadingOlder.current) return
          if (el.scrollTop > 60) return
          if (messagesQuery.hasNextPage && !messagesQuery.isFetchingNextPage) {
            loadingOlder.current = true
            olderScrollHeight.current = el.scrollHeight
            void messagesQuery.fetchNextPage()
          }
        }}
        onClick={() => setMenuId(null)}
      >
        <div className="messages-inner" ref={contentRef}>
          {messagesQuery.isFetchingNextPage && (
            <div className="messages-loading">Loading earlier messages…</div>
          )}
          {messages.map((m, i) => {
            const prev = messages[i - 1]
            const next = messages[i + 1]
            const showDay = !prev || dayLabel(prev.sendAt) !== dayLabel(m.sendAt)
            const mine = m.senderId === myId
            const replied = m.replyToMessageId ? byId.get(m.replyToMessageId) : undefined
            const sender = sendersById.get(Number(m.senderId))
            const senderName =
              sender?.userInfo?.name ||
              (sender?.userInfo?.username ? `@${sender.userInfo.username}` : `User #${m.senderId}`)
            const clusterEnd =
              !next ||
              next.senderId !== m.senderId ||
              dayLabel(next.sendAt) !== dayLabel(m.sendAt)
            const clusterStart =
              !prev ||
              prev.senderId !== m.senderId ||
              dayLabel(prev.sendAt) !== dayLabel(m.sendAt)
            const showAvatar = clusterEnd
            const showSenderName = !direct && clusterStart
            const avatar = (
              <div className="bubble-avatar">
                {showAvatar ? (
                  <Avatar
                    name={senderName}
                    fileId={sender?.avatarFileId}
                    size={36}
                    onClick={(e) => {
                      e.stopPropagation()
                      openProfile(Number(m.senderId))
                    }}
                  />
                ) : null}
              </div>
            )
            return (
              <div key={m.messageId} data-mid={m.messageId}>
                {showDay && <div className="day-sep">{dayLabel(m.sendAt)}</div>}
                <div className={`bubble-row ${mine ? 'out' : 'in'}${clusterEnd ? ' tail' : ''}`}>
                  {!mine && avatar}
                  <div
                    className={`bubble ${mine ? 'out' : 'in'}${highlightId === m.messageId ? ' highlight' : ''}`}
                    onContextMenu={(e) => {
                      e.preventDefault()
                      setMenuId(m.messageId)
                    }}
                    onClick={(e) => {
                      e.stopPropagation()
                      setMenuId(menuId === m.messageId ? null : m.messageId)
                    }}
                  >
                    {showSenderName && <div className="bubble-sender">{senderName}</div>}
                    {replied && (
                      <div className="reply-quote">{replied.text.slice(0, 100)}</div>
                    )}
                    {m.text ? <div className="bubble-text">{m.text}</div> : null}
                    <MessageAttachments ids={m.attachmentIds} />
                    <div className="bubble-meta">
                      {m.isPinned ? <span>📌</span> : null}
                      <span>{formatTime(m.sendAt)}</span>
                      {m.pending ? <span>…</span> : null}
                    </div>
                    {menuId === m.messageId && (
                      <div className="msg-menu">
                        <button
                          type="button"
                          onClick={() => {
                            setReplyTo(m.messageId)
                            setMenuId(null)
                          }}
                        >
                          Reply
                        </button>
                        {mine && (
                          <button
                            type="button"
                            onClick={() => {
                              setEditingId(m.messageId)
                              setText(m.text)
                              setMenuId(null)
                            }}
                          >
                            Edit
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={async () => {
                            try {
                              await pinMessage(m.messageId, !m.isPinned)
                              qc.invalidateQueries({ queryKey: ['messages', chatId] })
                            } catch {
                              showToast('Pin failed')
                            }
                            setMenuId(null)
                          }}
                        >
                          {m.isPinned ? 'Unpin' : 'Pin'}
                        </button>
                        {mine && (
                          <button
                            type="button"
                            onClick={async () => {
                              try {
                                await deleteMessage(m.messageId)
                                qc.invalidateQueries({ queryKey: ['messages', chatId] })
                              } catch {
                                showToast('Delete failed')
                              }
                              setMenuId(null)
                            }}
                          >
                            Delete
                          </button>
                        )}
                      </div>
                    )}
                  </div>
                  {mine && avatar}
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {(replyMsg || editingId) && (
        <div className="composer-banner">
          <span>
            {editingId ? 'Editing' : 'Replying'}:{' '}
            {(replyMsg || byId.get(editingId!))?.text.slice(0, 60)}
          </span>
          <button
            type="button"
            className="icon-btn"
            onClick={() => {
              setReplyTo(null)
              setEditingId(null)
              if (editingId) setText('')
            }}
          >
            ×
          </button>
        </div>
      )}

      <div className="composer">
        <button
          type="button"
          className="icon-btn"
          title="Attach"
          onClick={() => fileRef.current?.click()}
        >
          ＋
        </button>
        <input
          ref={fileRef}
          type="file"
          accept="image/*,*/*"
          hidden
          onChange={(e) => {
            const f = e.target.files?.[0]
            if (f) void onAttach(f)
            e.target.value = ''
          }}
        />
        <textarea
          rows={1}
          placeholder="Message"
          value={text}
          onChange={(e) => {
            setText(e.target.value)
            onTyping(chatId)
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              void onSend()
            }
          }}
        />
        <button
          className="send-btn"
          type="button"
          disabled={!text.trim()}
          onClick={() => void onSend()}
        >
          ➤
        </button>
      </div>

      {infoOpen && <ChatInfoPanel />}
    </div>
  )
}
