import { create } from 'zustand'

type UIState = {
  activeChatId: number | null
  mobileView: 'list' | 'chat'
  newChatOpen: boolean
  newGroupOpen: boolean
  searchOpen: boolean
  infoOpen: boolean
  /** null = closed; number = user id to show */
  profileUserId: number | null
  /** chatId -> userIds currently typing */
  typingByChat: Record<number, number[]>
  replyToId: number | null
  onlineUsers: Record<number, boolean>
  toast: string | null
  setActiveChat: (id: number | null) => void
  setMobileView: (v: 'list' | 'chat') => void
  setNewChatOpen: (v: boolean) => void
  setNewGroupOpen: (v: boolean) => void
  setSearchOpen: (v: boolean) => void
  setInfoOpen: (v: boolean) => void
  openProfile: (userId: number) => void
  closeProfile: () => void
  setUserTyping: (chatId: number, userId: number) => void
  clearUserTyping: (chatId: number, userId: number) => void
  setReplyTo: (id: number | null) => void
  setOnlineUsers: (users: Record<number, boolean>) => void
  showToast: (msg: string) => void
  clearToast: () => void
}

export const useUIStore = create<UIState>((set) => ({
  activeChatId: null,
  mobileView: 'list',
  newChatOpen: false,
  newGroupOpen: false,
  searchOpen: false,
  infoOpen: false,
  profileUserId: null,
  typingByChat: {},
  replyToId: null,
  onlineUsers: {},
  toast: null,
  setActiveChat: (id) => {
    const activeChatId = id == null ? null : Number(id)
    set({
      activeChatId: activeChatId && Number.isFinite(activeChatId) ? activeChatId : null,
      mobileView: activeChatId ? 'chat' : 'list',
      infoOpen: false,
      replyToId: null,
    })
  },
  setMobileView: (mobileView) => set({ mobileView }),
  setNewChatOpen: (newChatOpen) => set({ newChatOpen }),
  setNewGroupOpen: (newGroupOpen) => set({ newGroupOpen }),
  setSearchOpen: (searchOpen) => set({ searchOpen }),
  setInfoOpen: (infoOpen) => set({ infoOpen }),
  openProfile: (userId) => {
    const id = Number(userId)
    if (!id || !Number.isFinite(id)) return
    set({ profileUserId: id, infoOpen: false })
  },
  closeProfile: () => set({ profileUserId: null }),
  setUserTyping: (chatId, userId) => {
    const cid = Number(chatId)
    const uid = Number(userId)
    if (!cid || !uid) return
    set((state) => {
      const prev = state.typingByChat[cid] ?? []
      if (prev.includes(uid)) return state
      return {
        typingByChat: { ...state.typingByChat, [cid]: [...prev, uid] },
      }
    })
  },
  clearUserTyping: (chatId, userId) => {
    const cid = Number(chatId)
    const uid = Number(userId)
    if (!cid || !uid) return
    set((state) => {
      const prev = state.typingByChat[cid] ?? []
      if (!prev.includes(uid)) return state
      const next = prev.filter((id) => id !== uid)
      const typingByChat = { ...state.typingByChat }
      if (next.length) typingByChat[cid] = next
      else delete typingByChat[cid]
      return { typingByChat }
    })
  },
  setReplyTo: (id) => set({ replyToId: id == null ? null : Number(id) || null }),
  setOnlineUsers: (onlineUsers) => set({ onlineUsers }),
  showToast: (toast) => set({ toast }),
  clearToast: () => set({ toast: null }),
}))
