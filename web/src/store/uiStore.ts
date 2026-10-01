import { create } from 'zustand'

type UIState = {
  activeChatId: number | null
  mobileView: 'list' | 'chat'
  newChatOpen: boolean
  searchOpen: boolean
  typingChatId: number | null
  toast: string | null
  setActiveChat: (id: number | null) => void
  setMobileView: (v: 'list' | 'chat') => void
  setNewChatOpen: (v: boolean) => void
  setSearchOpen: (v: boolean) => void
  setTypingChat: (id: number | null) => void
  showToast: (msg: string) => void
  clearToast: () => void
}

export const useUIStore = create<UIState>((set) => ({
  activeChatId: null,
  mobileView: 'list',
  newChatOpen: false,
  searchOpen: false,
  typingChatId: null,
  toast: null,
  setActiveChat: (id) => set({ activeChatId: id, mobileView: id ? 'chat' : 'list' }),
  setMobileView: (mobileView) => set({ mobileView }),
  setNewChatOpen: (newChatOpen) => set({ newChatOpen }),
  setSearchOpen: (searchOpen) => set({ searchOpen }),
  setTypingChat: (typingChatId) => set({ typingChatId }),
  showToast: (toast) => set({ toast }),
  clearToast: () => set({ toast: null }),
}))
