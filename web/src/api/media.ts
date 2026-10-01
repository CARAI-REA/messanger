import { api } from './http'

export async function initUpload(filename: string, mime: string, sizeBytes: number) {
  return api<{ fileId: string; putUrl: string }>('/api/v1/media/uploads', {
    method: 'POST',
    body: JSON.stringify({ filename, mime, sizeBytes }),
  })
}

export async function completeUpload(fileId: string) {
  return api(`/api/v1/media/uploads/${encodeURIComponent(fileId)}:complete`, {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export async function getFile(fileId: string) {
  return api<{ fileId: string; getUrl?: string; mime?: string }>(
    `/api/v1/media/files/${encodeURIComponent(fileId)}`,
  )
}
