import dayjs from 'dayjs'
import duration from 'dayjs/plugin/duration'
import utc from 'dayjs/plugin/utc'
import timezone from 'dayjs/plugin/timezone'
import quarterOfYear from 'dayjs/plugin/quarterOfYear'
import localizedFormat from 'dayjs/plugin/localizedFormat'
import type { DurationLike } from '@sentio/ui-core'
import type { SeriesLike } from '../types'

dayjs.extend(duration)
dayjs.extend(utc)
dayjs.extend(timezone)
dayjs.extend(quarterOfYear)
dayjs.extend(localizedFormat)

const TimeUnitShortNames: Record<string, string> = {
  s: 'seconds',
  m: 'minutes',
  h: 'hours',
  d: 'days',
  w: 'weeks',
  M: 'months',
  y: 'years'
}

/** [min, max] timestamp across all series (data is sorted by timestamp). */
export function dateRangeOfSeries(series: SeriesLike<Date>[]): [Date, Date] {
  let min = new Date()
  let max = new Date(0)
  for (const s of series) {
    if (s.data.length > 0) {
      const dmin = s.data[0][0] || min
      const dmax = s.data[s.data.length - 1][0] || max
      if (dmin < min) min = dmin
      if (dmax > max) max = dmax
    }
  }
  return [min, max]
}

export function durationToSeconds(d?: DurationLike): number {
  if (!d) return 0
  return dayjs
    .duration(Number(d.value) || 0, d.unit as dayjs.UnitTypeShort)
    .asSeconds()
}

export function parseDuration(s: string): DurationLike {
  const m = s.match(/(\d+)([a-z]+)/i)
  if (m) {
    const [, value, unit] = m
    return { value: parseInt(value), unit: TimeUnitShortNames[unit] }
  }
  return { value: 0, unit: 'second' }
}

// Round a millisecond span to the nearest sensible bucket interval.
export function roundInterval(diff: number): string {
  switch (true) {
    case diff <= dayjs.duration(15, 'minute').asMilliseconds():
      return '10s'
    case diff <= dayjs.duration(1, 'day').asMilliseconds():
      return '1m'
    case diff <= dayjs.duration(30, 'day').asMilliseconds():
      return '1h'
    case diff <= dayjs.duration(2, 'years').asMilliseconds():
      return '1d'
    case diff <= dayjs.duration(10, 'years').asMilliseconds():
    default:
      return '7d'
  }
}

export function calculateStepByDate(start: Date, end: Date): DurationLike {
  return parseDuration(roundInterval(end.getTime() - start.getTime()))
}

/** Shift a Date so plain formatting renders the wall-clock time in `tz`. */
export function shiftTimezone(time: Date | dayjs.Dayjs, tz?: string): Date {
  const origin = dayjs.isDayjs(time) ? time : dayjs(time)
  if (!tz) return origin.toDate()
  const offset = origin.tz(tz).utcOffset() - origin.utcOffset()
  return origin.add(offset, 'minute').toDate()
}

export const formatTime = (
  time: Date,
  tz?: string,
  interval?: DurationLike
): string => {
  const d = dayjs(shiftTimezone(time, tz))
  switch (interval?.unit) {
    case 's':
      return d.format('HH:mm:ss')
    case 'm':
      return d.format('DD HH:mm')
    case 'h':
      return d.format('MM-DD HH:mm')
    case 'd':
    case 'w':
      return d.format('YYYY-MM-DD')
    case 'M':
      return interval.value === 3
        ? `${d.format('YYYY')}-Q${d.quarter()}`
        : d.format('YYYY-MMM')
    default:
      return d.format('lll')
  }
}
