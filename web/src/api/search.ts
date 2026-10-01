import { api } from './http'

export async function searchMessages(query: string, chatId?: number) {
  const q = new URLSearchParams({ query, limit: '20' })
  if (chatId) q.set('chatId', String(chatId))
  return api<{
    hits?: Array<{ chatId?: number; messageId?: number; text?: string; snippet?: string }>
  }>(`/api/v1/search/messages?${q}`)
}
