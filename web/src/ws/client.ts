import type { InfiniteData, QueryClient } from '@tanstack/react-query'
import type { Message } from '@/api/chat'

export type RealtimeEnvelope = {
  type: string
  event?: string
  event_id?: string
  chat_id?: number
  actor_id?: number
  user_id?: number
  payload?: string
  users?: Record<string, boolean>
  message?: {
    message_id?: number
    sender_id?: number
    text?: string
    attachment_ids?: string[]
    send_at?: string
    updated_at?: string
    is_pinned?: boolean
    chat_id?: number
  }
}

type Handlers = {
  onEvent: (ev: RealtimeEnvelope) => void
  onOpen?: () => void
  onClose?: () => void
}

function setAccessCookie(token: string) {
  document.cookie = `access_token=${encodeURIComponent(token)}; path=/; SameSite=Lax`
}

export class RealtimeClient {
  private ws: WebSocket | null = null
  private timer: ReturnType<typeof setTimeout> | null = null
  private closed = false
  private attempt = 0

  constructor(
    private getToken: () => string | null,
    private handlers: Handlers,
  ) {}

  connect() {
    this.closed = false
    const token = this.getToken()
    if (!token) return
    setAccessCookie(token)

    // Always pass token for local/dev gateway (cookie Auth is also set for nginx).
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const qs = `?token=${encodeURIComponent(token)}`
    const url = `${proto}://${location.host}/ws${qs}`

    if (this.ws) {
      try {
        this.ws.onclose = null
        this.ws.close()
      } catch {
        /* ignore */
      }
    }

    this.ws = new WebSocket(url)
    this.ws.binaryType = 'arraybuffer'

    this.ws.onopen = () => {
      this.attempt = 0
      this.handlers.onOpen?.()
    }
    this.ws.onmessage = (msg) => {
      try {
        const text =
          typeof msg.data === 'string'
            ? msg.data
            : new TextDecoder().decode(msg.data as ArrayBuffer)
        const ev = JSON.parse(text) as RealtimeEnvelope
        this.handlers.onEvent(ev)
      } catch {
        /* ignore */
      }
    }
    this.ws.onclose = () => {
      this.handlers.onClose?.()
      this.scheduleReconnect()
    }
    this.ws.onerror = () => this.ws?.close()
  }

  private scheduleReconnect() {
    if (this.closed) return
    const delay = Math.min(10000, 500 * 2 ** this.attempt++)
    this.timer = setTimeout(() => this.connect(), delay)
  }

  sendTyping(chatId: number) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type: 'typing', chat_id: chatId }))
    }
  }

  subscribePresence(userIds: number[]) {
    if (this.ws?.readyState === WebSocket.OPEN && userIds.length) {
      this.ws.send(JSON.stringify({ type: 'presence.subscribe', user_ids: userIds }))
    }
  }

  close() {
    this.closed = true
    if (this.timer) clearTimeout(this.timer)
    this.ws?.close()
    this.ws = null
  }
}

type MessagesPages = InfiniteData<{ messages: Message[]; hasMore: boolean }, number | undefined>

function patchMessagesCache(
  qc: QueryClient,
  chatId: number,
  updater: (old: MessagesPages | undefined) => MessagesPages | undefined,
) {
  qc.setQueriesData<MessagesPages>(
    {
      predicate: (query) =>
        query.queryKey[0] === 'messages' && Number(query.queryKey[1]) === chatId,
    },
    (old) => updater(old),
  )
  // Canonical numeric key for open chat.
  qc.setQueryData<MessagesPages>(['messages', chatId], (old) => updater(old))
}

export function upsertChatMessage(qc: QueryClient, incoming: Message) {
  const chatId = Number(incoming.chatId)
  if (!chatId || !incoming.messageId) return
  patchMessagesCache(qc, chatId, (old) => {
    if (!old?.pages?.length) {
      return {
        pages: [{ messages: [incoming], hasMore: false }],
        pageParams: [undefined],
      }
    }
    const exists = old.pages.some((p) =>
      (p.messages ?? []).some((m) => Number(m.messageId) === Number(incoming.messageId)),
    )
    if (exists) return old
    const pages = old.pages.map((p, i) =>
      i === 0 ? { ...p, messages: [incoming, ...(p.messages ?? [])] } : p,
    )
    return { ...old, pages }
  })
}

function patchChatPreview(qc: QueryClient, incoming: Message, bumpUnread: boolean) {
  const chatId = Number(incoming.chatId)
  qc.setQueryData(['chats'], (old: { chats?: Array<Record<string, unknown>> } | undefined) => {
    if (!old?.chats) return old
    const preview =
      incoming.text || (incoming.attachmentIds?.length ? '🖼 Photo' : '')
    return {
      ...old,
      chats: old.chats.map((c) =>
        Number(c.chatId) === chatId
          ? {
              ...c,
              lastMessagePreview: preview || c.lastMessagePreview,
              lastMessageAt: incoming.sendAt,
              lastMessageId: incoming.messageId,
              unreadCount: bumpUnread
                ? Number(c.unreadCount || 0) + 1
                : Number(c.unreadCount || 0),
            }
          : c,
      ),
    }
  })
}

export function applyRealtimeToCache(qc: QueryClient, ev: RealtimeEnvelope) {
  const chatId = Number(ev.chat_id)
  if (!chatId) return

  if (ev.event === 'message_created' && ev.message?.message_id) {
    const incoming: Message = {
      messageId: Number(ev.message.message_id),
      senderId: Number(ev.message.sender_id || ev.actor_id || 0),
      chatId,
      text: ev.message.text || '',
      sendAt: ev.message.send_at || new Date().toISOString(),
      attachmentIds: ev.message.attachment_ids || [],
    }
    upsertChatMessage(qc, incoming)
    const myId = Number(
      (qc.getQueryData(['me']) as { user?: { id?: number | string } } | undefined)?.user?.id || 0,
    )
    const mine = myId > 0 && incoming.senderId === myId
    patchChatPreview(qc, incoming, !mine)
    return
  }

  if (ev.event === 'message_edited' && ev.message?.message_id) {
    const mid = Number(ev.message.message_id)
    const text = ev.message.text || ''
    patchMessagesCache(qc, chatId, (old) => {
      if (!old) return old
      return {
        ...old,
        pages: old.pages.map((p) => ({
          ...p,
          messages: (p.messages ?? []).map((m) =>
            Number(m.messageId) === mid
              ? { ...m, text, updatedAt: ev.message?.updated_at || m.updatedAt }
              : m,
          ),
        })),
      }
    })
    return
  }

  if (ev.event === 'message_deleted' && ev.message?.message_id) {
    const mid = Number(ev.message.message_id)
    patchMessagesCache(qc, chatId, (old) => {
      if (!old) return old
      return {
        ...old,
        pages: old.pages.map((p) => ({
          ...p,
          messages: (p.messages ?? []).filter((m) => Number(m.messageId) !== mid),
        })),
      }
    })
    return
  }

  if (ev.event === 'message_pinned' && ev.message?.message_id) {
    const mid = Number(ev.message.message_id)
    const pinned = !!ev.message.is_pinned
    patchMessagesCache(qc, chatId, (old) => {
      if (!old) return old
      return {
        ...old,
        pages: old.pages.map((p) => ({
          ...p,
          messages: (p.messages ?? []).map((m) =>
            Number(m.messageId) === mid ? { ...m, isPinned: pinned } : m,
          ),
        })),
      }
    })
    return
  }

  void qc.invalidateQueries({ queryKey: ['messages'] })
  void qc.invalidateQueries({ queryKey: ['chats'] })
}
