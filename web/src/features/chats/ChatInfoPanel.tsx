import { useState } from 'react'
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { getUser, searchUsers } from '@/api/auth'
import {
  addMember,
  getChat,
  isDirectChat,
  isGroupAdmin,
  removeMember,
  updateChat,
} from '@/api/chat'
import { completeUpload, initUpload } from '@/api/media'
import { useAuthStore } from '@/store/authStore'
import { useUIStore } from '@/store/uiStore'
import { Avatar } from '@/components/Avatar'
import type { ApiError } from '@/api/http'

export function ChatInfoPanel() {
  const chatId = useUIStore((s) => s.activeChatId)
  const setOpen = useUIStore((s) => s.setInfoOpen)
  const showToast = useUIStore((s) => s.showToast)
  const openProfile = useUIStore((s) => s.openProfile)
  const myId = useAuthStore((s) => s.userId)
  const onlineUsers = useUIStore((s) => s.onlineUsers)
  const qc = useQueryClient()
  const [name, setName] = useState('')
  const [addQ, setAddQ] = useState('')
  const [uploading, setUploading] = useState(false)

  const chatQuery = useQuery({
    queryKey: ['chat', chatId],
    queryFn: () => getChat(chatId!),
    enabled: !!chatId,
  })

  const members = chatQuery.data?.participantIds ?? []
  const memberQueries = useQueries({
    queries: members.map((id) => ({
      queryKey: ['user', id],
      queryFn: () => getUser(id),
      staleTime: 60_000,
    })),
  })

  const search = useQuery({
    queryKey: ['info-add-search', addQ],
    queryFn: () => searchUsers(addQ),
    enabled: addQ.trim().length >= 2 && !isDirectChat(chatQuery.data?.chat),
  })

  const renameMut = useMutation({
    mutationFn: () => updateChat(chatId!, { name: name.trim() }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['chat', chatId] })
      qc.invalidateQueries({ queryKey: ['chats'] })
      showToast('Renamed')
    },
    onError: (e) => showToast((e as unknown as ApiError).message || 'Failed'),
  })

  if (!chatId || !chatQuery.data) return null
  const chat = chatQuery.data.chat
  const direct = isDirectChat(chat)
  const canManage = !direct && isGroupAdmin(chatQuery.data.myRole)
  const ownerId = chat.ownerId

  return (
    <div className="info-panel">
      <div className="info-panel-top">
        <h3>{direct ? 'Chat info' : 'Group info'}</h3>
        <button type="button" className="icon-btn" onClick={() => setOpen(false)}>
          ×
        </button>
      </div>
      {!direct && (
        <div className="auth-form" style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 12 }}>
            <Avatar name={chat.chatInfo?.name} fileId={chat.avatarFileId} size={72} />
          </div>
          {canManage && (
            <label className="btn btn-ghost" style={{ textAlign: 'center', cursor: 'pointer' }}>
              {uploading ? 'Uploading…' : 'Upload group avatar'}
              <input
                type="file"
                accept="image/*"
                hidden
                disabled={uploading}
                onChange={async (e) => {
                  const f = e.target.files?.[0]
                  if (!f) return
                  setUploading(true)
                  try {
                    const init = await initUpload(f.name, f.type || 'image/jpeg', f.size)
                    if (!init.putUrl || !init.fileId) throw new Error('init upload failed')
                    const put = await fetch(init.putUrl, {
                      method: 'PUT',
                      body: f,
                      headers: { 'content-type': f.type || 'image/jpeg' },
                    })
                    if (!put.ok) throw new Error(`upload ${put.status}`)
                    await completeUpload(init.fileId)
                    await updateChat(chatId, { avatarFileId: init.fileId })
                    await qc.invalidateQueries({ queryKey: ['chat', chatId] })
                    await qc.invalidateQueries({ queryKey: ['chats'] })
                    await qc.invalidateQueries({ queryKey: ['media', init.fileId] })
                    showToast('Group avatar updated')
                  } catch (err) {
                    showToast(err instanceof Error ? err.message : 'Upload failed')
                  } finally {
                    setUploading(false)
                    e.target.value = ''
                  }
                }}
              />
            </label>
          )}
          {canManage ? (
            <>
              <label>
                Group name
                <input
                  defaultValue={chat.chatInfo?.name || ''}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Name"
                />
              </label>
              <button
                type="button"
                className="btn btn-primary"
                disabled={!name.trim() || renameMut.isPending}
                onClick={() => renameMut.mutate()}
              >
                Save name
              </button>
            </>
          ) : (
            <div className="user-pick-name" style={{ textAlign: 'center' }}>
              {chat.chatInfo?.name || `Group #${chatId}`}
            </div>
          )}
        </div>
      )}
      <div className="info-section-title">Members ({members.length})</div>
      <div className="user-pick-list">
        {members.map((id, i) => {
          const u = memberQueries[i]?.data?.user
          const isOwner = id === ownerId
          return (
            <div key={id} className="user-pick" style={{ cursor: 'default' }}>
              <Avatar
                name={u?.userInfo?.name || u?.userInfo?.username}
                fileId={u?.avatarFileId}
                size={40}
                onClick={() => openProfile(id)}
              />
              <div style={{ flex: 1 }}>
                <div className="user-pick-name">
                  {u?.userInfo?.name || `User #${id}`}
                  {id === myId ? ' (you)' : ''}
                  {isOwner ? ' · admin' : ''}
                </div>
                <div className="user-pick-user">
                  @{u?.userInfo?.username || id}
                  {onlineUsers[id] ? ' · online' : ''}
                </div>
              </div>
              {canManage && id !== myId && !isOwner && (
                <button
                  type="button"
                  className="btn btn-ghost"
                  onClick={async () => {
                    try {
                      await removeMember(chatId, id)
                      qc.invalidateQueries({ queryKey: ['chat', chatId] })
                    } catch (err) {
                      showToast((err as ApiError).message || 'Remove failed')
                    }
                  }}
                >
                  Remove
                </button>
              )}
            </div>
          )
        })}
      </div>
      {canManage && (
        <>
          <div className="info-section-title">Add member</div>
          <input
            className="search-input"
            placeholder="@username"
            value={addQ}
            onChange={(e) => setAddQ(e.target.value)}
          />
          {(search.data?.users ?? [])
            .filter((u) => !members.includes(u.id))
            .map((u) => (
              <button
                key={u.id}
                type="button"
                className="user-pick"
                onClick={async () => {
                  try {
                    await addMember(chatId, u.id)
                    setAddQ('')
                    qc.invalidateQueries({ queryKey: ['chat', chatId] })
                    showToast('Added')
                  } catch (err) {
                    showToast((err as ApiError).message || 'Add failed')
                  }
                }}
              >
                <Avatar name={u.userInfo?.name} fileId={u.avatarFileId} size={40} />
                <div>
                  <div className="user-pick-name">{u.userInfo?.name}</div>
                  <div className="user-pick-user">@{u.userInfo?.username}</div>
                </div>
              </button>
            ))}
        </>
      )}
    </div>
  )
}
