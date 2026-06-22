import type { DurationLike } from '@sentio/ui-core'

const longUnits: Record<string, string> = {
  s: 'seconds',
  m: 'minutes',
  h: 'hours',
  d: 'days',
  w: 'weeks',
  M: 'months',
  y: 'years'
}

// "previous hour" / "previous days" — count drives singular/plural, count itself
// not shown (matches app lib/time.durationToLongString).
export function durationToLongString(d: DurationLike): string {
  const u = longUnits[d.unit ?? ''] ?? ''
  return Number(d.value) === 1 ? u.replace(/s$/, '') : u
}
