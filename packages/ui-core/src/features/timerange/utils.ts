import dayjs from 'dayjs'
import duration from 'dayjs/plugin/duration'
import relativeTime from 'dayjs/plugin/relativeTime'
import utc from 'dayjs/plugin/utc'
import timezone from 'dayjs/plugin/timezone'
import pluralize from 'pluralize'
import {
  DEFAULT_TZ,
  DateTimeValue,
  FULL_DATE_TIME_PATTERN,
  RelativeTime,
  relativeTimeToString
} from '../../utils/time'

dayjs.extend(duration)
dayjs.extend(relativeTime)
dayjs.extend(utc)
dayjs.extend(timezone)

export const formatTimeRange = (
  start?: DateTimeValue | null,
  end?: DateTimeValue | null,
  tz?: string
) => {
  if (!start || !end) {
    return ''
  }
  const s = formatTime(start, tz)
  const e = formatTime(end, tz)
  if (s == e) {
    return s
  }
  return `${s} - ${e}`
}

export const applyTz = (date: dayjs.Dayjs, tz?: string) =>
  // https://github.com/iamkun/dayjs/issues/1606
  tz == 'UTC' ? date.utc() : date.tz(tz || DEFAULT_TZ)

const formatTime = (value?: DateTimeValue, tz?: string) => {
  if (!value) {
    return ''
  }
  if (dayjs.isDayjs(value)) {
    return applyTz(value, tz).format(FULL_DATE_TIME_PATTERN)
  } else {
    const v = value as RelativeTime
    if (v.align) {
      if (v.value == 0 || v.unit == null) {
        return `this ${pluralize.singular(v.align)}`
      }
      if (v.sign == -1) {
        return `previous ${pluralize(v.align, v.value, v.value != 1)}`
      }
      return relativeTimeToString(value)
    }
    if (v.value == 0 || v.unit == null) {
      return 'now'
    } else {
      return (
        v.value +
        ' ' +
        (v.value > 1 ? v.unit : v.unit.slice(0, -1)) +
        (value?.sign == -1 ? ' ago' : ' from now')
      )
    }
  }
}

export const formatTimeZone = (timeZone: string) => {
  const now = new Date()
  const dateString = Intl.DateTimeFormat([], {
    timeZone,
    timeZoneName: 'longOffset'
  }).format(now)
  return dateString.split(' ')[1].replace('GMT', 'UTC')
}
