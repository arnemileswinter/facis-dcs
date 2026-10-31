/**
 * The signing workflow a contract document defines (DCS-FR-SM-13): the
 * position of each signature field in the signing order (dcs:signingOrder,
 * lowest signs first) and the signing deadline (dcs:signingDeadline). The
 * backend enforces both at prepare and submit; this reads them for display.
 */
export interface SigningWorkflow {
  order: { field: string; position: number }[]
  deadline?: Date
}

function literal(value: unknown): unknown {
  return value && typeof value === 'object' && '@value' in value ? value['@value'] : value
}

export function signingWorkflow(contractData: unknown): SigningWorkflow {
  const doc = (contractData ?? {}) as Record<string, unknown>
  const fields = (doc['dcs:signatureFields'] ?? []) as Record<string, unknown>[]
  const order = fields
    .map((node) => ({
      field: typeof node['dcs:signatoryName'] === 'string' ? node['dcs:signatoryName'].trim() : '',
      position: Number(literal(node['dcs:signingOrder'])),
    }))
    .filter((entry) => entry.field !== '' && Number.isInteger(entry.position) && entry.position >= 1)
    .sort((a, b) => a.position - b.position)

  const raw = literal(doc['dcs:signingDeadline'])
  let deadline: Date | undefined
  if (typeof raw === 'string' && raw.trim() !== '') {
    // A date without a time includes the whole day in UTC, as on the backend.
    const text = /^\d{4}-\d{2}-\d{2}$/.test(raw.trim()) ? `${raw.trim()}T23:59:59.999Z` : raw.trim()
    const parsed = new Date(text)
    if (!Number.isNaN(parsed.getTime())) deadline = parsed
  }
  return { order, deadline }
}
