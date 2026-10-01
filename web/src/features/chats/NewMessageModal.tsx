import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { searchUsers, type PublicUser } from '@/api/auth'
import { getOrCreateDirect } from '@/api/chat'
import { useUIStore } from '@/store/uiStore'
import { Avatar } from '@/components/Avatar'
import type { ApiError } from '@/api/http'

export function NewMessageModal() {
  const open = useUIStore((s) => s.newChatOpen)
  const setOpen = useUIStore((s) => s.setNewChatOpen)
  const setActive = useUIStore((s) => s.setActiveChat)
  const showToast = useUIStore((s) => s.showToast)
  const [q, setQ] = useState('')
  const [debounced, setDebounced] = useState('')
  const qc = useQueryClient()

  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 200)
    return () => clearTimeout(t)
  }, [q])

  const { data, isFetching } = useQuery({
    queryKey: ['users-search', debounced],
    queryFn: () => searchUsers(debounced),
    enabled: open && debounced.length >= 1,
  })

  const mut = useMutation({
    mutationFn: (user: PublicUser) => getOrCreateDirect(user.id),
    onSuccess: async (res) => {
      await qc.invalidateQueries({ queryKey: ['chats'] })
      setActive(res.chatId)
      setOpen(false)
      setQ('')
      showToast('Chat opened')
    },
    onError: (err) => showToast((err as unknown as ApiError).message || 'Failed'),
  })

  if (!open) return null
  const users = data?.users ?? []

  return (
    <div className="modal-backdrop" onClick={() => setOpen(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>New message</h3>
        <input
          className="search-input"
          autoFocus
          placeholder="Search @username or name"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <div className="user-pick-list">
          {isFetching && <div className="muted-pad">Searching…</div>}
          {!isFetching && debounced && !users.length && (
            <div className="muted-pad">No users found</div>
          )}
          {users.map((u) => (
            <button
              key={u.id}
              type="button"
              className="user-pick"
              disabled={mut.isPending}
              onClick={() => mut.mutate(u)}
            >
              <Avatar
                name={u.userInfo?.name || u.userInfo?.username}
                fileId={u.avatarFileId}
                size={40}
              />
              <div>
                <div className="user-pick-name">{u.userInfo?.name || 'User'}</div>
                <div className="user-pick-user">@{u.userInfo?.username || u.id}</div>
              </div>
            </button>
          ))}
        </div>
        <div className="modal-actions">
          <button type="button" className="btn btn-ghost" onClick={() => setOpen(false)}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  )
}
