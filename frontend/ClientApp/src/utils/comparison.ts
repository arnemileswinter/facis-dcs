export function toComparableValue(value: unknown) {
  if (typeof value === 'number') return value
  if (value instanceof Date) return value.getTime()
  if (typeof value === 'string') {
    if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/.test(value)) {
      const dateTime = new Date(value).getTime()
      if (!Number.isNaN(dateTime)) return dateTime
    }
    return value
  }
  return undefined
}
