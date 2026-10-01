import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createChat } from '@/api/chat'
import { useAuthStore } from '@/store/authStore'
import { useUIStore } from '@/store/uiStore'
import type { ApiError } from '@/api/http'

export function NewChatModal() {
  const open = useUIStore((s) => s.newChatOpen)
  const setOpen = useUIStore((s) => s.setNewChatOpen)
  const setActiveChat = useUIStore((s) => s.setActiveChat)
  const showToast = useUIStore((s) => s.showToast)
  const myId = useAuthStore((s) => s.userId)
  const qc = useQueryClient()
  const [name, setName] = useState('')
  const [peerId, setPeerId] = useState('')
  const [error, setError] = useState('')

  const mutation = useMutation({
    mutationFn: async () => {
      const peer = Number(peerId)
      if (!peer || peer === myId) throw { message: 'Enter another user id' } as ApiError
      const ids = Array.from(new Set([myId, peer]))
      return createChat(name.trim() || `Chat with ${peer}`, '', ids)
    },
    onSuccess: async (res) => {
      await qc.invalidateQueries({ queryKey: ['chats'] })
      setActiveChat(res.chatId)
      setOpen(false)
      setName('')
      setPeerId('')
      showToast('Chat created')
    },
    onError: (err) => setError((err as unknown as ApiError).message || 'Failed'),
  })

  if (!open) return null

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    mutation.mutate()
  }

  return (
    <div className="modal-backdrop" onClick={() => setOpen(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>New chat</h3>
        <form className="auth-form" onSubmit={onSubmit}>
          <label>
            Title
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Optional" />
          </label>
          <label>
            Peer user id
            <input
              value={peerId}
              onChange={(e) => setPeerId(e.target.value)}
              placeholder="e.g. 2"
              required
            />
          </label>
          <div className="auth-error">{error}</div>
          <div className="modal-actions">
            <button type="button" className="btn btn-ghost" onClick={() => setOpen(false)}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={mutation.isPending}>
              Create
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
