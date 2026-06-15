import { classNames } from '../../utils/classnames'
import dayjs from 'dayjs'
import weekday from 'dayjs/plugin/weekday'
import localeData from 'dayjs/plugin/localeData'
import utc from 'dayjs/plugin/utc'
import { useEffect, useMemo, useState } from 'react'
import { LuChevronLeft, LuChevronRight } from 'react-icons/lu'
import { Select } from '../../common/select/Select'

dayjs.extend(weekday)
dayjs.extend(localeData)
dayjs.extend(utc)

// Small numeric range helper.
const range = (start: number, stop: number, step = 1): number[] =>
  Array(Math.ceil((stop - start) / step))
    .fill(start)
    .map((x, y) => x + y * step)

interface Props {
  value: dayjs.Dayjs
  start?: dayjs.Dayjs
  end?: dayjs.Dayjs
  minDate?: dayjs.Dayjs
  maxDate?: dayjs.Dayjs
  onSelect: (value: dayjs.Dayjs) => void
  className?: string
  enableRangeLimit?: boolean
  useUTC?: boolean
}

function SelectYear({
  currentYear,
  onChange
}: {
  currentYear: number
  onChange: (year: number) => void
}) {
  const years = useMemo(
    () =>
      range(currentYear - 20, currentYear + 20).map((y) => ({
        value: y,
        label: y + ''
      })),
    [currentYear]
  )
  return (
    <Select options={years} value={currentYear} onChange={(v) => onChange(v)} />
  )
}

function SelectMonth({
  currentMonth,
  onChange
}: {
  currentMonth: number
  onChange: (month: number) => void
}) {
  const months = useMemo(
    () =>
      dayjs.monthsShort().map((m, idx) => ({
        value: idx,
        label: m
      })),
    []
  )
  return (
    <Select
      options={months}
      value={currentMonth}
      onChange={(v) => onChange(v)}
    />
  )
}

export default function Calendar({
  value,
  onSelect,
  className,
  start,
  end,
  maxDate,
  minDate,
  enableRangeLimit = false,
  useUTC = false
}: Props) {
  const getDate = (date: dayjs.Dayjs) => (useUTC ? date.utc() : date)

  const [firstDay, setFirstDay] = useState(getDate(value).date(1))

  useEffect(() => {
    setFirstDay(getDate(value).date(1))
  }, [value, useUTC])

  const days = useMemo(() => {
    const days: any[] = []
    let curr = getDate(firstDay).weekday(0).hour(0).minute(0).second(0)
    const lastDay = getDate(firstDay).endOf('month').weekday(6)
    while (curr.isBefore(lastDay)) {
      let isDisabled = false
      if (enableRangeLimit && start && end) {
        isDisabled =
          curr.isBefore(getDate(start), 'day') ||
          curr.isAfter(getDate(end), 'day')
      }
      if (maxDate && curr.isAfter(getDate(maxDate), 'day')) {
        isDisabled = true
      }
      if (minDate && curr.isBefore(getDate(minDate), 'day')) {
        isDisabled = true
      }
      days.push({
        date: curr,
        isCurrentMonth: curr.month() == getDate(firstDay).month(),
        isToday: getDate(dayjs()).isSame(curr, 'day'),
        isStart: start && curr.isSame(getDate(start), 'day'),
        isEnd: end && curr.isSame(getDate(end), 'day'),
        isSelected:
          start &&
          end &&
          curr.isAfter(getDate(start)) &&
          curr.isBefore(getDate(end)),
        isDisabled
      })
      curr = curr.add(1, 'day')
    }
    return days
  }, [firstDay, start, end, enableRangeLimit, maxDate, useUTC])

  const handleDateClick = (date: dayjs.Dayjs, isDisabled: boolean) => {
    if (!isDisabled) {
      onSelect(getDate(date))
    }
  }

  return (
    <div className={classNames(className, 'hidden max-w-md pt-1 md:block')}>
      <div className="text-text-foreground mb-3 flex items-center text-center">
        <button
          onClick={() => setFirstDay(getDate(firstDay).subtract(1, 'month'))}
          type="button"
          className="text-text-foreground-disabled hover:text-text-foreground-secondary mr-2 flex flex-none items-center justify-center p-1.5"
        >
          <span className="sr-only">Previous month</span>
          <LuChevronLeft className="h-5 w-5" aria-hidden="true" />
        </button>
        <div className="basis-1/2">
          <SelectMonth
            currentMonth={getDate(firstDay).month()}
            onChange={(month) => setFirstDay(getDate(firstDay).month(month))}
          />
        </div>
        <div className="ml-1 basis-1/2">
          <SelectYear
            currentYear={getDate(firstDay).year()}
            onChange={(year) => setFirstDay(getDate(firstDay).year(year))}
          />
        </div>
        <button
          onClick={() => setFirstDay(getDate(firstDay).add(1, 'month'))}
          type="button"
          className="text-text-foreground-disabled hover:text-text-foreground-secondary ml-2 flex flex-none items-center justify-center p-1.5"
        >
          <span className="sr-only">Next month</span>
          <LuChevronRight className="h-5 w-5" aria-hidden="true" />
        </button>
      </div>
      <div className="text-text-foreground-secondary grid grid-cols-7 text-center text-xs leading-6">
        {dayjs.weekdaysShort().map((day, index) => (
          <div key={index}>{day}</div>
        ))}
      </div>
      <div
        className="isolate mt-2 grid gap-y-1 text-xs"
        style={{ gridTemplateColumns: 'repeat(6, 1fr) auto' }}
      >
        {days.map(
          ({
            date,
            isCurrentMonth,
            isToday,
            isSelected,
            isStart,
            isEnd,
            isDisabled
          }) => (
            <div
              className={classNames(
                !isCurrentMonth && 'invisible',
                isStart &&
                  end &&
                  !isEnd &&
                  'bg-primary-100 dark:bg-primary-300 rounded-l-full',
                isSelected && !isEnd && 'bg-primary-100 dark:bg-primary-300'
              )}
              key={date.format('YYYY-MM-DD')}
            >
              <button
                onClick={() => handleDateClick(date, isDisabled)}
                type="button"
                disabled={isDisabled}
                className={classNames(
                  'dark:hover:bg-primary-400 relative hover:bg-gray-100 focus:z-10',
                  date.weekday() === 6 ? 'mr-1.5' : 'mr-2.5',
                  isEnd &&
                    start &&
                    !isStart &&
                    'bg-primary-100 dark:bg-primary-300 rounded-r-full',
                  isDisabled && 'cursor-not-allowed opacity-30'
                )}
              >
                <time
                  dateTime={getDate(date).format('YYYY-MM-DD')}
                  className={classNames(
                    'mx-auto flex h-7 w-7 items-center justify-center rounded-full',
                    isStart && 'bg-primary-600 rounded-full text-white',
                    isEnd && 'bg-primary-600 rounded-full text-white'
                  )}
                  aria-selected={isSelected}
                >
                  {getDate(date).date()}
                </time>
                {isToday && (
                  <div className="absolute bottom-0.5 left-3 h-1 w-1 rounded-full bg-cyan-500" />
                )}
              </button>
            </div>
          )
        )}
      </div>
    </div>
  )
}
