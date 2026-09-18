/** Formatting for the values the server sends as RFC 3339 strings. */

const instantFormat = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'medium',
})

/** The reader's own timezone. An unparseable value is shown as it arrived
 *  rather than as "Invalid Date". */
export function formatInstant(iso: string): string {
  const at = new Date(iso).getTime()
  return Number.isNaN(at) ? iso : instantFormat.format(at)
}

function count(value: number, unit: string): string {
  return `${value} ${unit}${value === 1 ? '' : 's'}`
}

/** How long a bin has left, in whole units, so the phrase always agrees with
 *  the exact timestamp beside it. */
export function untilExpiry(iso: string, now: number): string {
  const at = new Date(iso).getTime()
  if (Number.isNaN(at)) return ''
  const seconds = Math.floor((at - now) / 1000)
  if (seconds <= 0) return 'expired'
  if (seconds < 60) return `in ${count(seconds, 'second')}`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `in ${count(minutes, 'minute')}`
  const hours = Math.floor(minutes / 60)
  if (hours < 48) return `in ${count(hours, 'hour')}`
  return `in ${count(Math.floor(hours / 24), 'day')}`
}

const clockFormat = new Intl.DateTimeFormat(undefined, { timeStyle: 'medium' })

/** Time of day alone, for list rows where the date is the same all the way
 *  down and the second is what distinguishes one capture from the next. */
export function formatClock(iso: string): string {
  const at = new Date(iso).getTime()
  return Number.isNaN(at) ? iso : clockFormat.format(at)
}
