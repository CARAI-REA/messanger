import type { MouseEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getFile } from '@/api/media'
import { initials } from '@/lib/format'

export function Avatar({
  name,
  url,
  fileId,
  size = 48,
  onClick,
}: {
  name?: string
  url?: string
  fileId?: string | null
  size?: number
  onClick?: (e: MouseEvent) => void
}) {
  const id = fileId?.trim() || ''
  const media = useQuery({
    queryKey: ['media', id],
    queryFn: () => getFile(id),
    enabled: !!id && !url,
    staleTime: 45 * 60_000,
    retry: 1,
  })

  const src = url || media.data?.getUrl
  const style = {
    width: size,
    height: size,
    fontSize: size * 0.34,
    cursor: onClick ? ('pointer' as const) : undefined,
  }

  if (src) {
    return (
      <img
        className="avatar"
        src={src}
        alt=""
        style={style}
        onClick={onClick}
        role={onClick ? 'button' : undefined}
      />
    )
  }
  return (
    <div className="avatar" style={style} onClick={onClick} role={onClick ? 'button' : undefined}>
      {initials(name)}
    </div>
  )
}
