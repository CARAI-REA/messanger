import { useEffect, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { searchUsers, type PublicUser } from '@/api/auth'
import { createGroup } from '@/api/chat'
import { useUIStore } from '@/store/uiStore'
import { Avatar } from '@/components/Avatar'
import type { ApiError } from '@/api/http'

export function NewGroupModal() {
  const open = useUIStore((s) => s.newGroupOpen)
  const setOpen = useUIStore((s) => s.setNewGroupOpen)
  const setActive = useUIStore((s) => s.setActiveChat)
  const showToast = useUIStore((s) => s.showToast)
  const [title, setTitle] = useState('')
  const [q, setQ] = useState('')
  const [debounced, setDebounced] = useState('')
  const [picked, setPicked] = useState<PublicUser[]>([])
  const [error, setError] = useState('')
  const qc = useQueryClient()

  useEffect(() => {
    const t = setTimeout(() => setDebounced(q.trim()), 200)
    return () => clearTimeout(t)
  }, [q])

  const { data } = useQuery({
    queryKey: ['users-search-group', debounced],
    queryFn: () => searchUsers(debounced),
    enabled: open && debounced.length >= 1,
  })

  const mut = useMutation({
    mutationFn: () =>
      createGroup(
        title.trim(),
        '',
        picked.map((p) => p.id),
      ),
    onSuccess: async (res) => {
      await qc.invalidateQueries({ queryKey: ['chats'] })
      setActive(res.chatId)
      setOpen(false)
      setTitle('')
      setPicked([])
      setQ('')
      showToast('Group created')
    },
    onError: (err) => setError((err as unknown as ApiError).message || 'Failed'),
  })

  if (!open) return null

  function toggle(u: PublicUser) {
    setPicked((prev) =>
      prev.some((p) => p.id === u.id) ? prev.filter((p) => p.id !== u.id) : [...prev, u],
    )
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (!title.trim()) {
      setError('Group name required')
      return
    }
    if (!picked.length) {
      setError('Pick at least one member')
      return
    }
    mut.mutate()
  }

  const users = (data?.users ?? []).filter((u) => !picked.some((p) => p.id === u.id))

  return (
    <div className="modal-backdrop" onClick={() => setOpen(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>New group</h3>
        <form className="auth-form" onSubmit={onSubmit}>
          <label>
            Title
            <input value={title} onChange={(e) => setTitle(e.target.value)} required />
          </label>
          <input
            className="search-input"
            placeholder="Add members by @username"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
          {!!picked.length && (
            <div className="chip-row">
              {picked.map((p) => (
                <button key={p.id} type="button" className="chip" onClick={() => toggle(p)}>
                  @{p.userInfo?.username || p.id} ×
                </button>
              ))}
            </div>
          )}
          <div className="user-pick-list">
            {users.map((u) => (
              <button key={u.id} type="button" className="user-pick" onClick={() => toggle(u)}>
                <Avatar
                  name={u.userInfo?.name || u.userInfo?.username}
                  fileId={u.avatarFileId}
                  size={40}
                />
                <div>
                  <div className="user-pick-name">{u.userInfo?.name}</div>
                  <div className="user-pick-user">@{u.userInfo?.username}</div>
                </div>
              </button>
            ))}
          </div>
          <div className="auth-error">{error}</div>
          <div className="modal-actions">
            <button type="button" className="btn btn-ghost" onClick={() => setOpen(false)}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={mut.isPending}>
              Create
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
