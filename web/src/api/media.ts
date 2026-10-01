import { api } from './http'

export type MediaFile = {
  fileId: string
  getUrl?: string
  mime?: string
  status?: string
  sizeBytes?: number
}

function normalizeFile(raw: Record<string, unknown>): MediaFile {
  return {
    fileId: String(raw.fileId || raw.file_id || ''),
    getUrl: (raw.getUrl || raw.get_url || undefined) as string | undefined,
    mime: (raw.mime || undefined) as string | undefined,
    status: (raw.status || undefined) as string | undefined,
    sizeBytes: Number(raw.sizeBytes ?? raw.size_bytes ?? 0) || undefined,
  }
}

export async function initUpload(filename: string, mime: string, sizeBytes: number) {
  const data = await api<Record<string, unknown>>('/api/v1/media/uploads', {
    method: 'POST',
    body: JSON.stringify({ filename, mime, sizeBytes }),
  })
  return {
    fileId: String(data.fileId || data.file_id || ''),
    putUrl: String(data.putUrl || data.put_url || ''),
  }
}

export async function completeUpload(fileId: string) {
  const data = await api<Record<string, unknown>>(
    `/api/v1/media/uploads/${encodeURIComponent(fileId)}:complete`,
    {
      method: 'POST',
      body: JSON.stringify({ fileId }),
    },
  )
  return normalizeFile(data)
}

export async function getFile(fileId: string) {
  const data = await api<Record<string, unknown>>(
    `/api/v1/media/files/${encodeURIComponent(fileId)}`,
  )
  return normalizeFile(data)
}

export function isImageMime(mime?: string) {
  return !!mime && mime.toLowerCase().startsWith('image/')
}

export function filenameFromUrl(url?: string) {
  if (!url) return 'file'
  try {
    const path = new URL(url).pathname
    const part = decodeURIComponent(path.split('/').pop() || 'file')
    return part || 'file'
  } catch {
    return 'file'
  }
}
