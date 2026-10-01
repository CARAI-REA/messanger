export function initials(name?: string): string {
  if (!name?.trim()) return '?'
  const parts = name.trim().split(/\s+/)
  const a = parts[0]?.[0] ?? ''
  const b = parts.length > 1 ? parts[parts.length - 1][0] : parts[0]?.[1] ?? ''
  return (a + b).toUpperCase()
}

export function formatTime(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  if (sameDay) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString([], { day: '2-digit', month: 'short' })
}

export function dayLabel(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  const today = new Date()
  const yday = new Date()
  yday.setDate(today.getDate() - 1)
  if (d.toDateString() === today.toDateString()) return 'Today'
  if (d.toDateString() === yday.toDateString()) return 'Yesterday'
  return d.toLocaleDateString([], { day: 'numeric', month: 'long', year: 'numeric' })
}
