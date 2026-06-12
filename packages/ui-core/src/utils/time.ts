/*
 * Time utilities shared by timerange (and other) components.
 *
 * Pure date/time helpers with no dependency on any consumer data model, so
 * they are safe to use standalone. Consumers may re-export these alongside
 * their own model-coupled time helpers.
 */
import dayjs from 'dayjs'
import pluralize from 'pluralize'
import LocalizedFormat from 'dayjs/plugin/localizedFormat'
import duration from 'dayjs/plugin/duration'
import timezone from 'dayjs/plugin/timezone'
import isoWeek from 'dayjs/plugin/isoWeek'

dayjs.extend(duration)
dayjs.extend(LocalizedFormat)
dayjs.extend(timezone)
dayjs.extend(isoWeek)

export type TimeUnit =
  | 'seconds'
  | 'minutes'
  | 'hours'
  | 'days'
  | 'weeks'
  | 'months'
  | 'years'

export const TimeUnitShortNames: Record<string, TimeUnit> = {
  s: 'seconds',
  m: 'minutes',
  h: 'hours',
  d: 'days',
  w: 'weeks',
  M: 'months',
  y: 'years'
}

export function timeUnit2String(u: TimeUnit) {
  if (u == 'months') {
    return 'M'
  }
  return u[0]
}

export const FULL_DATE_TIME_PATTERN = 'MMM D, YYYY HH:mm:ss'

export interface RelativeTime {
  value: number
  unit?: TimeUnit
  sign: 0 | 1 | -1
  align?: TimeUnit
}

export const now: RelativeTime = { value: 0, sign: 0 }
export type DateTimeValue = dayjs.Dayjs | RelativeTime

export const ago = (value: number, unit: TimeUnit): RelativeTime => {
  return {
    value,
    unit,
    sign: -1
  }
}

export const previous = (value: number, unit: TimeUnit): RelativeTime => {
  return {
    value,
    unit,
    sign: -1,
    align: unit
  }
}

export const fromNow = (value: number, unit: TimeUnit): RelativeTime => {
  return {
    value,
    unit,
    sign: 1
  }
}

export const isNow = (value: DateTimeValue): boolean => {
  return (
    isRelativeTime(value) &&
    (value as RelativeTime).value == 0 &&
    (value as RelativeTime).align == null
  )
}

export const isRelativeTime = (value: DateTimeValue): boolean => {
  return !dayjs.isDayjs(value)
}

export function toDayjs(t: DateTimeValue, asStart = true): dayjs.Dayjs {
  if (dayjs.isDayjs(t)) {
    return t
  }
  t = t as RelativeTime
  const ret = dayjs().add(t.sign * t.value, t.unit || 'second')
  if (t.align) {
    if (asStart) {
      return ret.startOf(t.align)
    } else {
      return ret.endOf(t.align)
    }
  }
  return ret
}

export function fromNumber(n: number): DateTimeValue {
  return dayjs(new Date(n))
}

export function parseRelativeTime(s?: string): DateTimeValue | null {
  if (!s) {
    return null
  } else {
    const regex = /^(now)?(([-+])(\d+)([Mhwdmy]))?(\/([Mwdy]))?$/
    const m = s.match(regex)
    if (m) {
      const [, , , sign, value, units, , align] = m
      return {
        value: value ? parseInt(value) : 0,
        unit: units ? TimeUnitShortNames[units] : undefined,
        sign: sign === '-' ? -1 : 1,
        align: align ? TimeUnitShortNames[align] : undefined
      }
    }
    return null
  }
}

export function parseDateTime(s?: string): DateTimeValue | null {
  if (!s) {
    return null
  } else {
    const t = parseRelativeTime(s)
    if (t) {
      return t
    } else {
      const number = Number(s)
      if (isNaN(number)) {
        // try parse ISO date
        const d = dayjs(s)
        return d.isValid() ? d : null
      } else if (number < 10000000000) {
        // unix timestamp
        const d = dayjs.unix(number)
        return d.isValid() ? d : null
      } else {
        // date in milliseconds
        const d = dayjs(new Date(number))
        return d.isValid() ? d : null
      }
    }
  }
}

// default time range is 30 days
export const DEFAULT_FROM = ago(30, 'days')
export const DEFAULT_TO = now
export const DEFAULT_TZ = Intl.DateTimeFormat().resolvedOptions().timeZone

export function dateTimeToString(d: DateTimeValue | undefined | null): string {
  if (dayjs.isDayjs(d)) {
    return d.unix() + ''
  } else if (d == null) {
    return ''
  } else {
    const t = d as RelativeTime
    return relativeTimeToString(t)
  }
}

export const relativeTimeToString = (t?: RelativeTime) => {
  if (!t) {
    return ''
  }
  let str = 'now'
  if (t.sign !== 0 && t.value !== 0 && t.unit) {
    str += `${t.sign * t.value}${timeUnit2String(t.unit)}`
  }
  if (t.align) {
    return str + '/' + timeUnit2String(t.align)
  }
  return str
}

export function isBefore(
  a: DateTimeValue | null | undefined,
  b: DateTimeValue | null | undefined
): boolean {
  if (a == null || b == null) {
    return false
  }
  return toDayjs(a).isBefore(toDayjs(b, false))
}

export function timeRangeToDisplayString(
  from: DateTimeValue,
  to: DateTimeValue
): string {
  let result: string
  if (isNow(to)) {
    if (isRelativeTime(from)) {
      const t = from as RelativeTime
      result = `Past ${pluralize(t.unit!, t.value, true)}`
    } else {
      result = `Since ${timeToDisplayString(from)}`
    }
  } else {
    result = `${timeToDisplayString(from)} - ${timeToDisplayString(to)}`
  }
  return result
}

export function timeToDisplayString(t: DateTimeValue): string {
  if (isRelativeTime(t)) {
    t = t as RelativeTime
    if (t.sign === 0 || t.value === 0) {
      return 'now'
    }
    if (t.sign < 0) {
      return `${pluralize(t.unit!, t.value, true)} ago`
    } else {
      return `${pluralize(t.unit!, t.value, true)} after`
    }
  } else {
    return toDayjs(t).format('LLL')
  }
}

export function displayDate(d?: Date | string): string {
  if (typeof d == 'string') {
    return dayjs(parseInt(d)).format('LLL')
  } else {
    return dayjs(d).format('LLL')
  }
}

export function pickRangeByTimeRange(
  currentTime: DateTimeValue,
  startTime?: DateTimeValue,
  endTime?: DateTimeValue
): {
  startTime: DateTimeValue
  endTime: DateTimeValue
} {
  const start = toDayjs(startTime || currentTime)
  const end = toDayjs(endTime || currentTime)
  const current = toDayjs(currentTime)

  const offset = end.diff(start, 'minute')

  if (offset >= 60 * 24 * 365) {
    // 1year: 1week
    return {
      startTime: current,
      endTime: current.add(1, 'week')
    }
  } else if (offset >= 60 * 24 * 30) {
    // 1month: 1day
    return {
      startTime: current,
      endTime: current.add(1, 'day')
    }
  } else if (offset >= 60 * 24 * 7) {
    // 1week: 12hour
    return {
      startTime: current,
      endTime: current.add(12, 'hour')
    }
  } else if (offset >= 60 * 24 * 2) {
    // 2day: 2hour
    return {
      startTime: current,
      endTime: current.add(2, 'hour')
    }
  } else if (offset >= 60 * 24) {
    // 1day: 1hour
    return {
      startTime: current,
      endTime: current.add(1, 'hour')
    }
  } else if (offset >= 60 * 4) {
    // 4hour: 5min
    return {
      startTime: current,
      endTime: current.add(5, 'minute')
    }
  } else if (offset >= 60) {
    // 1hour: 1min
    return {
      startTime: current,
      endTime: current.add(1, 'minute')
    }
  } else if (offset >= 30) {
    // 30min: 30s
    return {
      startTime: current,
      endTime: current.add(30, 'second')
    }
  }

  return {
    startTime: current,
    endTime: current.add(5, 'second')
  }
}

export function computeDuration(
  startTime: DateTimeValue,
  endTime: DateTimeValue
) {
  return dayjs.duration(toDayjs(endTime).diff(toDayjs(startTime)))
}

const tzOffsetCache = new Map<string, number>()
function cachedTzOffset(tz: string, origin: dayjs.Dayjs) {
  const key = tz + origin.format('YYYY-MM-DD')
  if (tzOffsetCache.has(key)) {
    return tzOffsetCache.get(key)!
  }
  try {
    const tzDate = origin.tz(tz, true)
    const diff = origin.diff(tzDate, 'minute')
    tzOffsetCache.set(key, diff)
    return diff
  } catch (e) {
    // tz is not correct timezone string
    console.error(e)
    return 0
  }
}

export function shiftTimezone(time: Date | dayjs.Dayjs, tz?: string) {
  const origin = dayjs.isDayjs(time) ? time : dayjs(time)
  const diff = cachedTzOffset(tz || DEFAULT_TZ, origin)
  return origin.add(diff, 'minute').toDate()
}
