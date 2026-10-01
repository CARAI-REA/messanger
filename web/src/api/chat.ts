import { api } from './http'

export type Chat = {
  chatId: number
  ownerId: number
  chatInfo?: { name?: string; description?: string; userIds?: number[] }
  lastMessageAt?: string
  lastMessageId?: number
  unreadCount?: number
}

export type Message = {
  messageId: number
  senderId: number
  chatId: number
  text: string
  isPinned?: boolean
  sendAt?: string
  updatedAt?: string
  attachmentIds?: string[]
  pending?: boolean
}

export async function listChats(limit = 50) {
  return api<{ chats: Chat[]; nextCursor?: string }>(`/api/v1/chats?limit=${limit}`)
}

export async function getChat(chatId: number) {
  return api<{ chat: Chat; participantIds: number[] }>(`/api/v1/chats/${chatId}`)
}

export async function createChat(name: string, description: string, userIds: number[]) {
  return api<{ chatId: number }>('/api/v1/chats', {
    method: 'POST',
    body: JSON.stringify({ chatInfo: { name, description, userIds } }),
  })
}

export async function listMessages(chatId: number, limit = 40, beforeId?: number) {
  const q = new URLSearchParams({ limit: String(limit) })
  if (beforeId) q.set('beforeId', String(beforeId))
  return api<{ messages: Message[]; hasMore: boolean }>(
    `/api/v1/chats/${chatId}/messages?${q}`,
  )
}

export async function sendMessage(
  chatId: number,
  text: string,
  idempotencyKey: string,
  attachmentIds: string[] = [],
) {
  return api<{ messageId: number }>(`/api/v1/chats/${chatId}/messages`, {
    method: 'POST',
    body: JSON.stringify({ chatId, text, idempotencyKey, attachmentIds }),
  })
}

export async function markRead(chatId: number, messageId: number) {
  return api(`/api/v1/chats/${chatId}:read`, {
    method: 'POST',
    body: JSON.stringify({ chatId, messageId }),
  })
}
