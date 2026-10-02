import { api } from './http'

export type ChatType = 'CHAT_TYPE_UNSPECIFIED' | 'CHAT_TYPE_DIRECT' | 'CHAT_TYPE_GROUP' | number

export type Chat = {
  chatId: number
  ownerId: number
  chatInfo?: { name?: string; description?: string; userIds?: number[] }
  lastMessageAt?: string
  lastMessageId?: number
  unreadCount?: number
  chatType?: ChatType
  lastMessagePreview?: string
  participantIds?: number[]
  peerUserId?: number
  avatarFileId?: string
  isPinned?: boolean
}

export type ChatRole = 'ROLE_USER' | 'ROLE_ADMIN' | 'ROLE_OWNER' | number

export type Message = {
  messageId: number
  senderId: number
  chatId: number
  text: string
  isPinned?: boolean
  sendAt?: string
  updatedAt?: string
  attachmentIds?: string[]
  replyToMessageId?: number
  pending?: boolean
}

/** Envoy grpc-json often encodes int64 as string — normalize everywhere. */
export function numId(v: unknown): number {
  const n = typeof v === 'number' ? v : Number(v)
  return Number.isFinite(n) ? n : 0
}

function normalizeChat(raw: Record<string, unknown>): Chat {
  const info = (raw.chatInfo || raw.chat_info || {}) as Record<string, unknown>
  const userIds = ((info.userIds || info.user_ids || []) as unknown[]).map(numId)
  const participantIds = ((raw.participantIds || raw.participant_ids || userIds) as unknown[]).map(
    numId,
  )
  return {
    chatId: numId(raw.chatId ?? raw.chat_id),
    ownerId: numId(raw.ownerId ?? raw.owner_id),
    chatInfo: {
      name: String(info.name ?? ''),
      description: String(info.description ?? ''),
      userIds,
    },
    lastMessageAt: (raw.lastMessageAt || raw.last_message_at) as string | undefined,
    lastMessageId: raw.lastMessageId || raw.last_message_id
      ? numId(raw.lastMessageId ?? raw.last_message_id)
      : undefined,
    unreadCount: numId(raw.unreadCount ?? raw.unread_count),
    chatType: (raw.chatType ?? raw.chat_type) as ChatType,
    lastMessagePreview: String(raw.lastMessagePreview ?? raw.last_message_preview ?? ''),
    participantIds,
    peerUserId: raw.peerUserId || raw.peer_user_id ? numId(raw.peerUserId ?? raw.peer_user_id) : 0,
    avatarFileId: String(raw.avatarFileId ?? raw.avatar_file_id ?? '') || undefined,
    isPinned: !!(raw.isPinned ?? raw.is_pinned),
  }
}

function normalizeMessage(raw: Record<string, unknown>): Message {
  return {
    messageId: numId(raw.messageId ?? raw.message_id),
    senderId: numId(raw.senderId ?? raw.sender_id),
    chatId: numId(raw.chatId ?? raw.chat_id),
    text: String(raw.text ?? ''),
    isPinned: !!(raw.isPinned ?? raw.is_pinned),
    sendAt: (raw.sendAt || raw.send_at) as string | undefined,
    updatedAt: (raw.updatedAt || raw.updated_at) as string | undefined,
    attachmentIds: ((raw.attachmentIds || raw.attachment_ids || []) as unknown[]).map(String),
    replyToMessageId: raw.replyToMessageId || raw.reply_to_message_id
      ? numId(raw.replyToMessageId ?? raw.reply_to_message_id)
      : undefined,
  }
}

export function isDirectChat(c?: Chat | null): boolean {
  if (!c) return false
  return c.chatType === 'CHAT_TYPE_DIRECT' || c.chatType === 1
}

export function isGroupAdmin(role?: ChatRole | null): boolean {
  if (role == null) return false
  return role === 'ROLE_ADMIN' || role === 'ROLE_OWNER' || role === 1 || role === 2
}

export async function listChats(limit = 50) {
  const data = await api<{ chats?: Record<string, unknown>[]; nextCursor?: string }>(
    `/api/v1/chats?limit=${limit}`,
  )
  return {
    chats: (data.chats ?? []).map((c) => normalizeChat(c)),
    nextCursor: data.nextCursor,
  }
}

export async function getChat(chatId: number) {
  const data = await api<{
    chat: Record<string, unknown>
    participantIds?: unknown[]
    myRole?: ChatRole
    my_role?: ChatRole
  }>(`/api/v1/chats/${chatId}`)
  const chat = normalizeChat(data.chat || {})
  const participantIds = (data.participantIds ?? chat.participantIds ?? []).map(numId)
  const myRole = (data.myRole ?? data.my_role) as ChatRole | undefined
  return { chat: { ...chat, participantIds }, participantIds, myRole }
}

export async function createGroup(name: string, description: string, userIds: number[]) {
  const data = await api<{ chatId?: unknown }>('/api/v1/chats', {
    method: 'POST',
    body: JSON.stringify({ chatInfo: { name, description, userIds } }),
  })
  return { chatId: numId(data.chatId) }
}

/** @deprecated use createGroup */
export async function createChat(name: string, description: string, userIds: number[]) {
  return createGroup(name, description, userIds)
}

export async function getOrCreateDirect(peerUserId: number) {
  const data = await api<{ chatId?: unknown }>('/api/v1/chats:direct', {
    method: 'POST',
    body: JSON.stringify({ peerUserId }),
  })
  return { chatId: numId(data.chatId) }
}

export async function updateChat(
  chatId: number,
  patch: { name?: string; description?: string; avatarFileId?: string },
) {
  return api(`/api/v1/chats/${chatId}`, {
    method: 'PATCH',
    body: JSON.stringify(patch),
  })
}

export async function addMember(chatId: number, userId: number) {
  return api(`/api/v1/chats/${chatId}/members`, {
    method: 'POST',
    body: JSON.stringify({ chatId, userId, role: 'ROLE_USER' }),
  })
}

export async function removeMember(chatId: number, userId: number) {
  return api(`/api/v1/chats/${chatId}/members/${userId}`, { method: 'DELETE' })
}

export async function listMessages(chatId: number, limit = 40, beforeId?: number) {
  const q = new URLSearchParams({ limit: String(limit) })
  if (beforeId) q.set('beforeId', String(beforeId))
  const data = await api<{ messages?: Record<string, unknown>[]; hasMore?: boolean }>(
    `/api/v1/chats/${chatId}/messages?${q}`,
  )
  return {
    messages: (data.messages ?? []).map((m) => normalizeMessage(m)),
    hasMore: !!data.hasMore,
  }
}

export async function sendMessage(
  chatId: number,
  text: string,
  idempotencyKey: string,
  attachmentIds: string[] = [],
  replyToMessageId?: number,
) {
  const data = await api<{ messageId?: unknown }>(`/api/v1/chats/${chatId}/messages`, {
    method: 'POST',
    body: JSON.stringify({
      chatId,
      text,
      idempotencyKey,
      attachmentIds,
      ...(replyToMessageId ? { replyToMessageId } : {}),
    }),
  })
  return { messageId: numId(data.messageId) }
}

export async function editMessage(messageId: number, text: string) {
  return api(`/api/v1/messages/${messageId}`, {
    method: 'PATCH',
    body: JSON.stringify({ messageId, text }),
  })
}

export async function deleteMessage(messageId: number) {
  return api(`/api/v1/messages/${messageId}`, { method: 'DELETE' })
}

export async function pinMessage(messageId: number, isPinned: boolean) {
  return api(`/api/v1/messages/${messageId}:pin`, {
    method: 'POST',
    body: JSON.stringify({ messageId, isPinned }),
  })
}

export async function pinChat(chatId: number, isPinned: boolean) {
  return api(`/api/v1/chats/${chatId}:pin`, {
    method: 'POST',
    body: JSON.stringify({ chatId, isPinned }),
  })
}

export async function markRead(chatId: number, messageId: number) {
  return api(`/api/v1/chats/${chatId}:read`, {
    method: 'POST',
    body: JSON.stringify({ chatId, messageId }),
  })
}
