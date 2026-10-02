import { api } from './http'
import { numId } from './chat'

export type SearchMessageHit = {
  chatId: number
  messageId: number
  senderId: number
  text: string
  score?: number
}

export type SearchChatHit = {
  chatId: number
  name: string
  description?: string
  score?: number
}

function normalizeMessageHit(raw: Record<string, unknown>): SearchMessageHit {
  return {
    chatId: numId(raw.chatId ?? raw.chat_id),
    messageId: numId(raw.messageId ?? raw.message_id),
    senderId: numId(raw.senderId ?? raw.sender_id),
    text: String(raw.text ?? raw.snippet ?? ''),
    score: raw.score != null ? Number(raw.score) : undefined,
  }
}

function normalizeChatHit(raw: Record<string, unknown>): SearchChatHit {
  return {
    chatId: numId(raw.chatId ?? raw.chat_id),
    name: String(raw.name ?? ''),
    description: String(raw.description ?? raw.snippet ?? '') || undefined,
    score: raw.score != null ? Number(raw.score) : undefined,
  }
}

export async function searchMessages(query: string, chatId?: number) {
  const q = new URLSearchParams({ query, limit: '20' })
  if (chatId) q.set('chatId', String(chatId))
  const data = await api<{ hits?: Record<string, unknown>[] }>(`/api/v1/search/messages?${q}`)
  return {
    hits: (data.hits ?? []).map(normalizeMessageHit).filter((h) => h.messageId && h.chatId),
  }
}

export async function searchChats(query: string) {
  const q = new URLSearchParams({ query, limit: '20' })
  const data = await api<{ hits?: Record<string, unknown>[] }>(`/api/v1/search/chats?${q}`)
  return {
    hits: (data.hits ?? []).map(normalizeChatHit).filter((h) => h.chatId),
  }
}
