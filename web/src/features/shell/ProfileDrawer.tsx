import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getMe, getUser, updateMe } from '@/api/auth'
import { getOrCreateDirect } from '@/api/chat'
import { completeUpload, initUpload } from '@/api/media'
import { useAuthStore } from '@/store/authStore'
import { useUIStore } from '@/store/uiStore'
import { Avatar } from '@/components/Avatar'
import type { ApiError } from '@/api/http'

export function ProfileDrawer() {
  const profileUserId = useUIStore((s) => s.profileUserId)
  const closeProfile = useUIStore((s) => s.closeProfile)
  const setActive = useUIStore((s) => s.setActiveChat)
  const showToast = useUIStore((s) => s.showToast)
  const onlineUsers = useUIStore((s) => s.onlineUsers)
  const myId = useAuthStore((s) => s.userId)
  const qc = useQueryClient()
  const [username, setUsername] = useState('')
  const [name, setName] = useState('')
  const [uploading, setUploading] = useState(false)
  const [localAvatarUrl, setLocalAvatarUrl] = useState<string | undefined>()
  const [editing, setEditing] = useState(false)

  const isOwn = !!profileUserId && profileUserId === myId

  const me = useQuery({
    queryKey: ['me'],
    queryFn: getMe,
    enabled: !!profileUserId && isOwn,
  })

  const other = useQuery({
    queryKey: ['user', profileUserId],
    queryFn: () => getUser(profileUserId!),
    enabled: !!profileUserId && !isOwn,
  })

  const user = isOwn ? me.data?.user : other.data?.user

  useEffect(() => {
    setLocalAvatarUrl(undefined)
    setEditing(false)
    setName('')
    setUsername('')
  }, [profileUserId])

  useEffect(() => {
    if (!user) return
    setName(user.userInfo?.name || '')
    setUsername(user.userInfo?.username || '')
  }, [user])

  const save = useMutation({
    mutationFn: () =>
      updateMe({
        ...(name.trim() ? { name: name.trim() } : {}),
        ...(username.trim() ? { username: username.trim().replace(/^@/, '') } : {}),
      }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ['me'] })
      if (myId) await qc.invalidateQueries({ queryKey: ['user', myId] })
      setEditing(false)
      showToast('Profile saved')
    },
    onError: (e) => showToast((e as unknown as ApiError).message || 'Failed'),
  })

  const messageMut = useMutation({
    mutationFn: () => getOrCreateDirect(profileUserId!),
    onSuccess: async (res) => {
      closeProfile()
      await qc.invalidateQueries({ queryKey: ['chats'] })
      setActive(res.chatId)
    },
    onError: (e) => showToast((e as unknown as ApiError).message || 'Failed'),
  })

  if (!profileUserId) return null

  const displayName = user?.userInfo?.name || user?.userInfo?.username || `User #${profileUserId}`
  const uname = user?.userInfo?.username
  const online = !!onlineUsers[profileUserId]

  async function onPickAvatar(file: File) {
    setUploading(true)
    try {
      const init = await initUpload(file.name, file.type || 'image/jpeg', file.size)
      if (!init.putUrl || !init.fileId) throw new Error('init upload failed')
      const put = await fetch(init.putUrl, {
        method: 'PUT',
        body: file,
        headers: { 'content-type': file.type || 'image/jpeg' },
      })
      if (!put.ok) throw new Error(`upload ${put.status}`)
      const done = await completeUpload(init.fileId)
      await updateMe({ avatarFileId: init.fileId })
      if (done.getUrl) setLocalAvatarUrl(done.getUrl)
      await qc.invalidateQueries({ queryKey: ['me'] })
      if (myId) await qc.invalidateQueries({ queryKey: ['user', myId] })
      await qc.invalidateQueries({ queryKey: ['media', init.fileId] })
      showToast('Avatar updated')
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Avatar upload failed')
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="profile-backdrop" onClick={closeProfile}>
      <aside className="profile-panel" onClick={(e) => e.stopPropagation()}>
        <div className="profile-top">
          <button type="button" className="icon-btn" onClick={closeProfile} title="Close">
            ←
          </button>
          <div className="profile-top-title">{isOwn ? 'Profile' : 'User info'}</div>
          {isOwn && !editing && (
            <button type="button" className="icon-btn" onClick={() => setEditing(true)} title="Edit">
              ✎
            </button>
          )}
          {isOwn && editing && (
            <button
              type="button"
              className="icon-btn"
              disabled={save.isPending}
              onClick={() => save.mutate()}
              title="Save"
            >
              ✓
            </button>
          )}
          {!isOwn && <span style={{ width: 40 }} />}
        </div>

        <div className="profile-hero">
          <div className="profile-avatar-wrap">
            <Avatar
              name={displayName}
              url={localAvatarUrl}
              fileId={user?.avatarFileId}
              size={120}
            />
            {isOwn && (
              <label className="profile-avatar-btn" title="Change photo">
                {uploading ? '…' : '📷'}
                <input
                  type="file"
                  accept="image/*"
                  hidden
                  disabled={uploading}
                  onChange={(e) => {
                    const f = e.target.files?.[0]
                    if (f) void onPickAvatar(f)
                    e.target.value = ''
                  }}
                />
              </label>
            )}
          </div>
          <h2 className="profile-name">{displayName}</h2>
          <div className="profile-sub">
            {isOwn ? (uname ? `@${uname}` : 'Set a username') : online ? 'online' : 'offline'}
          </div>
        </div>

        <div className="profile-section">
          {editing && isOwn ? (
            <div className="auth-form">
              <label>
                Name
                <input value={name} onChange={(e) => setName(e.target.value)} />
              </label>
              <label>
                Username
                <input
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="@username"
                />
              </label>
              <label>
                Email
                <input value={user?.userInfo?.email || ''} disabled />
              </label>
              <div className="modal-actions">
                <button
                  type="button"
                  className="btn btn-ghost"
                  onClick={() => {
                    setEditing(false)
                    setName(user?.userInfo?.name || '')
                    setUsername(user?.userInfo?.username || '')
                  }}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  className="btn btn-primary"
                  disabled={save.isPending}
                  onClick={() => save.mutate()}
                >
                  Save
                </button>
              </div>
            </div>
          ) : (
            <div className="profile-rows">
              {uname && (
                <div className="profile-row">
                  <div className="profile-row-label">Username</div>
                  <div className="profile-row-value">@{uname}</div>
                </div>
              )}
              {isOwn && user?.userInfo?.email && (
                <div className="profile-row">
                  <div className="profile-row-label">Email</div>
                  <div className="profile-row-value">{user.userInfo.email}</div>
                </div>
              )}
              {!isOwn && (
                <div className="profile-row">
                  <div className="profile-row-label">Status</div>
                  <div className="profile-row-value">{online ? 'online' : 'last seen recently'}</div>
                </div>
              )}
            </div>
          )}
        </div>

        {!isOwn && (
          <div className="profile-actions">
            <button
              type="button"
              className="btn btn-primary"
              disabled={messageMut.isPending}
              onClick={() => messageMut.mutate()}
            >
              Message
            </button>
          </div>
        )}

        {isOwn && !editing && (
          <div className="profile-actions">
            <button type="button" className="btn btn-ghost" onClick={() => setEditing(true)}>
              Edit profile
            </button>
            <label className="btn btn-ghost" style={{ textAlign: 'center', cursor: 'pointer' }}>
              {uploading ? 'Uploading…' : 'Set profile photo'}
              <input
                type="file"
                accept="image/*"
                hidden
                disabled={uploading}
                onChange={(e) => {
                  const f = e.target.files?.[0]
                  if (f) void onPickAvatar(f)
                  e.target.value = ''
                }}
              />
            </label>
          </div>
        )}
      </aside>
    </div>
  )
}
