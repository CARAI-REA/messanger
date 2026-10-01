import { useEffect, useMemo, useRef, useState } from 'react'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getChat,
  listMessages,
  markRead,
  sendMessage,
  type Message,
} from '@/api/chat'
import { useAuthStore } from '@/store/authStore'
import { useUIStore } from '@/store/uiStore'
import { dayLabel, formatTime, initials } from '@/lib/format'
import { initUpload, completeUpload } from '@/api/media'

export function ChatPane({
  onTyping,
}: {
  onTyping: (chatId: number) => void
}) {
  const chatId = useUIStore((s) => s.activeChatId)
  const typingChatId = useUIStore((s) => s.typingChatId)
  const setMobileView = useUIStore((s) => s.setMobileView)
  const showToast = useUIStore((s) => s.showToast)
  const myId = useAuthStore((s) => s.userId)
  const qc = useQueryClient()
  const [text, setText] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const fileRef = useRef<HTMLInputElement>(null)

  const chatQuery = useQuery({
    queryKey: ['chat', chatId],
    queryFn: () => getChat(chatId!),
    enabled: !!chatId,
  })

  const messagesQuery = useInfiniteQuery({
    queryKey: ['messages', chatId],
    enabled: !!chatId,
    initialPageParam: undefined as number | undefined,
    queryFn: ({ pageParam }) => listMessages(chatId!, 40, pageParam),
    getNextPageParam: (last) => {
      if (!last.hasMore || !last.messages?.length) return undefined
      const oldest = last.messages.reduce((m, x) => Math.min(m, x.messageId), Infinity)
      return Number.isFinite(oldest) ? oldest : undefined
    },
  })

  const messages = useMemo(() => {
    const pages = messagesQuery.data?.pages ?? []
    const flat = pages.flatMap((p) => p.messages ?? [])
    // API returns newest-first typically; normalize ascending for UI
    const map = new Map<number, Message>()
    for (const m of flat) map.set(m.messageId, m)
    return Array.from(map.values()).sort((a, b) => a.messageId - b.messageId)
  }, [messagesQuery.data])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages.length, chatId])

  useEffect(() => {
    if (!chatId || !messages.length) return
    const last = messages[messages.length - 1]
    if (last.senderId !== myId) {
      markRead(chatId, last.messageId).catch(() => undefined)
    }
  }, [chatId, messages, myId])

  const sendMut = useMutation({
    mutationFn: async (payload: { text: string; attachmentIds?: string[] }) => {
      const key = crypto.randomUUID()
      return sendMessage(chatId!, payload.text, key, payload.attachmentIds ?? [])
    },
    onMutate: async (payload) => {
      const tempId = -Date.now()
      const optimistic: Message = {
        messageId: tempId,
        senderId: myId,
        chatId: chatId!,
        text: payload.text,
        sendAt: new Date().toISOString(),
        pending: true,
        attachmentIds: payload.attachmentIds,
      }
      await qc.cancelQueries({ queryKey: ['messages', chatId] })
      qc.setQueryData(['messages', chatId], (old: typeof messagesQuery.data) => {
        if (!old) {
          return {
            pages: [{ messages: [optimistic], hasMore: false }],
            pageParams: [undefined],
          }
        }
        const pages = [...old.pages]
        const first = pages[0]
        pages[0] = { ...first, messages: [...(first.messages ?? []), optimistic] }
        return { ...old, pages }
      })
      return { tempId }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['messages', chatId] })
      qc.invalidateQueries({ queryKey: ['chats'] })
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

  const title = chatQuery.data?.chat.chatInfo?.name || `Chat #${chatId}`
  const typing = typingChatId === chatId

  async function onSend() {
    const t = text.trim()
    if (!t || sendMut.isPending) return
    setText('')
    sendMut.mutate({ text: t })
  }

  async function onAttach(file: File) {
    try {
      const init = await initUpload(file.name, file.type || 'application/octet-stream', file.size)
      const put = await fetch(init.putUrl, {
        method: 'PUT',
        body: file,
        headers: { 'content-type': file.type || 'application/octet-stream' },
      })
      if (!put.ok) throw new Error('upload failed')
      await completeUpload(init.fileId)
      const caption = text.trim() || file.name
      setText('')
      sendMut.mutate({ text: caption, attachmentIds: [init.fileId] })
    } catch {
      showToast('Upload failed')
    }
  }

  return (
    <div className="chat-pane">
      <div className="chat-header">
        <button type="button" className="icon-btn" onClick={() => setMobileView('list')} title="Back">
          ←
        </button>
        <div className="avatar" style={{ width: 42, height: 42, fontSize: 15 }}>
          {initials(title)}
        </div>
        <div>
          <h2>{title}</h2>
          <div className="sub">{typing ? 'typing…' : 'online'}</div>
        </div>
      </div>

      <div
        className="messages"
        ref={listRef}
        onScroll={() => {
          const el = listRef.current
          if (!el || el.scrollTop > 40) return
          if (messagesQuery.hasNextPage && !messagesQuery.isFetchingNextPage) {
            messagesQuery.fetchNextPage()
          }
        }}
      >
        {messages.map((m, i) => {
          const prev = messages[i - 1]
          const showDay =
            !prev || dayLabel(prev.sendAt) !== dayLabel(m.sendAt)
          const mine = m.senderId === myId
          return (
            <div key={m.messageId}>
              {showDay && <div className="day-sep">{dayLabel(m.sendAt)}</div>}
              <div className={`bubble-row ${mine ? 'out' : 'in'}`}>
                <div className={`bubble ${mine ? 'out' : 'in'}`}>
                  <div className="bubble-text">{m.text}</div>
                  {!!m.attachmentIds?.length && (
                    <div className="bubble-meta">📎 {m.attachmentIds.length} file(s)</div>
                  )}
                  <div className="bubble-meta">
                    <span>{formatTime(m.sendAt)}</span>
                    {m.pending ? <span>…</span> : null}
                  </div>
                </div>
              </div>
            </div>
          )
        })}
        <div ref={bottomRef} />
      </div>

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
        <button className="send-btn" type="button" disabled={!text.trim()} onClick={() => void onSend()}>
          ➤
        </button>
      </div>
    </div>
  )
}
