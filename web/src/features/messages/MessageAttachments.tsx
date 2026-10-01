import { useQuery } from '@tanstack/react-query'
import { filenameFromUrl, getFile, isImageMime } from '@/api/media'

function AttachmentItem({ fileId }: { fileId: string }) {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['media', fileId],
    queryFn: () => getFile(fileId),
    staleTime: 45 * 60_000,
    retry: 1,
  })

  if (isLoading) {
    return <div className="bubble-meta">Loading file…</div>
  }
  if (isError || !data?.getUrl) {
    return <div className="bubble-meta">📎 File unavailable</div>
  }

  const name = filenameFromUrl(data.getUrl)
  if (isImageMime(data.mime)) {
    return (
      <a className="bubble-attach" href={data.getUrl} target="_blank" rel="noreferrer">
        <img src={data.getUrl} alt={name} />
        <span className="bubble-attach-dl">Open / download</span>
      </a>
    )
  }

  return (
    <a className="bubble-file" href={data.getUrl} target="_blank" rel="noreferrer" download={name}>
      📎 {name}
    </a>
  )
}

export function MessageAttachments({ ids }: { ids?: string[] }) {
  if (!ids?.length) return null
  return (
    <div className="bubble-attachments">
      {ids.map((id) => (
        <AttachmentItem key={id} fileId={id} />
      ))}
    </div>
  )
}
