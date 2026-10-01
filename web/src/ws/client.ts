export type RealtimeEnvelope = {
  type: string
  event_id?: string
  chat_id?: number
  actor_id?: number
  user_id?: number
  payload?: string
}

type Handlers = {
  onEvent: (ev: RealtimeEnvelope) => void
  onOpen?: () => void
  onClose?: () => void
}

function setAccessCookie(token: string) {
  document.cookie = `access_token=${encodeURIComponent(token)}; path=/; SameSite=Lax`
}

export class RealtimeClient {
  private ws: WebSocket | null = null
  private timer: ReturnType<typeof setTimeout> | null = null
  private closed = false
  private attempt = 0

  constructor(
    private getToken: () => string | null,
    private handlers: Handlers,
  ) {}

  connect() {
    this.closed = false
    const token = this.getToken()
    if (!token) return
    setAccessCookie(token)

    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    // Vite dev proxy cannot inject Authorization; use query token (allowed in local gateway).
    const qs = import.meta.env.DEV ? `?token=${encodeURIComponent(token)}` : ''
    const url = `${proto}://${location.host}/ws${qs}`
    this.ws = new WebSocket(url)
    this.ws.binaryType = 'arraybuffer'

    this.ws.onopen = () => {
      this.attempt = 0
      this.handlers.onOpen?.()
    }
    this.ws.onmessage = (msg) => {
      try {
        const text =
          typeof msg.data === 'string'
            ? msg.data
            : new TextDecoder().decode(msg.data as ArrayBuffer)
        const ev = JSON.parse(text) as RealtimeEnvelope
        this.handlers.onEvent(ev)
      } catch {
        /* ignore */
      }
    }
    this.ws.onclose = () => {
      this.handlers.onClose?.()
      this.scheduleReconnect()
    }
    this.ws.onerror = () => this.ws?.close()
  }

  private scheduleReconnect() {
    if (this.closed) return
    const delay = Math.min(10000, 500 * 2 ** this.attempt++)
    this.timer = setTimeout(() => this.connect(), delay)
  }

  sendTyping(chatId: number) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type: 'typing', chat_id: chatId }))
    }
  }

  close() {
    this.closed = true
    if (this.timer) clearTimeout(this.timer)
    this.ws?.close()
    this.ws = null
  }
}

/** Best-effort: detect message_created field in protobuf payload (field 10). */
export function payloadLooksLikeMessageCreated(b64: string | undefined): boolean {
  if (!b64) return false
  try {
    const bin = atob(b64)
    // protobuf tags: field 10 wire type 2 => (10<<3)|2 = 82 = 0x52
    return bin.includes('\x52')
  } catch {
    return false
  }
}
