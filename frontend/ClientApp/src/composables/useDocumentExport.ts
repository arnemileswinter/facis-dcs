import { ref } from 'vue'
import { useErrorStore } from '@/stores/error-store'

/** How long a finished export's object URL is kept alive after the click. */
const REVOKE_DELAY_MS = 60_000

/**
 * Shared download-with-feedback for the Export PDF / Export bundle buttons:
 * a failed export surfaces the server's error as a toast instead of dying
 * as an unhandled rejection ("nothing happens"). Axios blob responses carry
 * blob error bodies, so the JSON error message is decoded before display.
 */
export function useDocumentExport() {
  const errorStore = useErrorStore()
  const exporting = ref(false)

  async function download(fetchBlob: () => Promise<Blob>, filename: string): Promise<void> {
    exporting.value = true
    try {
      const blob = await fetchBlob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      // The anchor must be in the document and the object URL must outlive the
      // click: the browser starts the download asynchronously, so revoking the
      // URL on the same tick can abort a multi-hundred-KB export before it ever
      // begins — the export silently produces no file at all.
      a.style.display = 'none'
      document.body.appendChild(a)
      a.click()
      a.remove()
      setTimeout(() => URL.revokeObjectURL(url), REVOKE_DELAY_MS)
    } catch (err: unknown) {
      errorStore.add(`Export failed: ${await exportErrorMessage(err)}`)
    } finally {
      exporting.value = false
    }
  }

  return { download, exporting }
}

async function exportErrorMessage(err: unknown): Promise<string> {
  const response = (err as { response?: { data?: unknown; status?: number } }).response
  const data = response?.data
  if (data instanceof Blob) {
    try {
      const body = JSON.parse(await data.text()) as { message?: string }
      if (body.message) return body.message
    } catch {
      // non-JSON error body — fall through to the generic message
    }
  }
  if (typeof data === 'object' && data !== null && 'message' in data) {
    return String(data.message)
  }
  if (response?.status) return `server responded with HTTP ${response.status}`
  return err instanceof Error ? err.message : 'unknown error'
}
