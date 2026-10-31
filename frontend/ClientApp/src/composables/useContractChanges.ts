import { onBeforeUnmount, onMounted } from 'vue'
import { getConfig } from '@/config'
import { authenticationService } from '@/services/authentication-service'
import { useAuthTokenStore } from '@/stores/auth-token-store'

/** One change from GET /contract/events (DCS-FR-CWE-24, DCS-FR-CSA-23). */
export interface ContractChange {
  did: string
  event_type: string
}

const RECONNECT_MS = 5_000
// The server sends a heartbeat every 30 seconds; silence beyond this is a dead connection.
const SILENCE_MS = 75_000

/**
 * Splits a server-sent event buffer into the changes it holds and the
 * unfinished rest. Events without a contract (CONNECTED, HEARTBEAT) are dropped.
 */
export function parseContractChanges(buffer: string): { changes: ContractChange[]; rest: string } {
  const blocks = buffer.split('\n\n')
  const rest = blocks.pop() ?? ''
  const changes: ContractChange[] = []
  for (const block of blocks) {
    const data = block
      .split('\n')
      .filter((line) => line.startsWith('data:'))
      .map((line) => line.slice(5).trim())
      .join('\n')
    try {
      const change = JSON.parse(data) as ContractChange
      if (change.did) changes.push(change)
    } catch {
      // not a JSON event
    }
  }
  return { changes, rest }
}

/**
 * Calls onChange for each change to a contract the user may read, while the
 * component is mounted. The stream reconnects after a drop and refreshes an
 * expired session. fetch is used because EventSource cannot send the
 * Authorization header.
 */
export function useContractChanges(onChange: (change: ContractChange) => void) {
  let stopped = false
  let controller: AbortController | null = null

  /**
   * Resolves true when a stream that delivered events ended, which the server
   * does after the token lifetime; the next stream then starts at once.
   */
  async function listen(): Promise<boolean> {
    controller = new AbortController()
    let silence = setTimeout(() => controller?.abort(), SILENCE_MS)
    try {
      const tokenStore = useAuthTokenStore()
      if (!tokenStore.isAuthSet) return false
      const response = await fetch(`${getConfig().API_BASE_URL}/contract/events`, {
        headers: { Authorization: tokenStore.getAuthenticationHeader, Accept: 'text/event-stream' },
        signal: controller.signal,
      })
      if (response.status === 401) {
        return (await authenticationService.refresh()) === true
      }
      if (!response.ok || !response.body) return false
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let received = false
      for (;;) {
        const { value, done } = await reader.read()
        if (done) return received
        received = true
        clearTimeout(silence)
        silence = setTimeout(() => controller?.abort(), SILENCE_MS)
        const parsed = parseContractChanges(buffer + decoder.decode(value, { stream: true }))
        buffer = parsed.rest
        parsed.changes.forEach(onChange)
      }
    } catch {
      // aborted or dropped: reconnect below
      return false
    } finally {
      clearTimeout(silence)
    }
  }

  async function run() {
    while (!stopped) {
      const ended = await listen()
      if (!stopped && !ended) await new Promise((resolve) => setTimeout(resolve, RECONNECT_MS))
    }
  }

  onMounted(() => void run())
  onBeforeUnmount(() => {
    stopped = true
    controller?.abort()
  })
}
