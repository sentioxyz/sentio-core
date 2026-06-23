/*
 * Proto-coupled time helpers ported from the app's `lib/time.ts`, reimplemented
 * against `DurationLike` (from @sentio/ui-core) instead of the generated proto
 * `Duration` message. Only the helpers referenced by the render charts are
 * ported here; the proto-free core (toDayjs, fromNumber, pickRangeByTimeRange,
 * dateTimeToString, isRelativeTime, now, TimeUnitShortNames, shiftTimezone …)
 * already lives in @sentio/ui-core.
 */
import dayjs from 'dayjs'
import {
  toDayjs,
  isRelativeTime,
  now,
  TimeUnitShortNames,
  type DateTimeValue,
  type RelativeTime,
  type DurationLike
} from '@sentio/ui-core'
import { durationToSeconds } from './series-utils'

/** Window [currentTime, currentTime + interval] picked by a fixed interval. */
export function pickRangeByInterval(
  currentTime: DateTimeValue,
  interval: DurationLike
): {
  startTime: DateTimeValue
  endTime: DateTimeValue
} {
  const start = toDayjs(currentTime)
  const end = start.add(durationToSeconds(interval), 'second')

  return {
    startTime: start,
    endTime: end
  }
}

/** Snap a time to the start/end of the bucket of size `step` (timezone-aware). */
export function alignTime(
  time: DateTimeValue,
  step: DurationLike,
  tz?: string,
  align: 'start' | 'end' = 'start'
): DateTimeValue {
  try {
    const d = toDayjs(time)
    const seconds = durationToSeconds(step)
    const tzOffset = (tz ? d.tz(tz).utcOffset() : 0) * 60
    const offset = (d.unix() + tzOffset) % seconds
    if (offset === 0) {
      return time
    }

    return align == 'start'
      ? d.subtract(offset, 'second')
      : d.add(seconds - offset, 'second')
  } catch (e) {
    // dayjs.tz may throw exception for some unknown reason
    console.error(e)
    return time
  }
}

/** Shift a time backwards by `duration`, preserving relative-time semantics. */
export function timeBefore(
  time: DateTimeValue,
  duration: DurationLike,
  asStart: boolean = true
): DateTimeValue {
  const durationValue = Number(duration.value)
  if (isRelativeTime(time)) {
    const rt = time as RelativeTime
    if (rt.unit == null || time == now) {
      return {
        sign: -1,
        unit: TimeUnitShortNames[duration.unit!],
        value: durationValue
      }
    }
    if (rt.align) {
      const t = toDayjs(rt, asStart)
      return t.subtract(durationValue, TimeUnitShortNames[duration.unit!])
    }

    const t = toDayjs(time)
    const nt = t.subtract(durationValue, TimeUnitShortNames[duration.unit!])
    const unit = 'days'
    const value = nt.diff(dayjs(), unit)
    return { sign: value < 0 ? -1 : 1, unit, value: Math.abs(value) }
  } else {
    const t = toDayjs(time)
    return t.subtract(durationValue, TimeUnitShortNames[duration.unit!])
  }
}
