/** API timestamps are Unix milliseconds. Every visitor sees the site's chosen zone. */
export function formatSiteDate(timestamp: number, locale: string, timezone: string): string {
  const date = new Date(timestamp)
  if (!Number.isFinite(date.getTime())) return ''
  const formatted = new Intl.DateTimeFormat(locale, {
    timeZone: timezone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).format(date)
  return `${formatted} (${timezone})`
}
