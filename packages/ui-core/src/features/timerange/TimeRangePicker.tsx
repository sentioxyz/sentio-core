import {
  dateTimeToString,
  DateTimeValue,
  isBefore,
  parseRelativeTime
} from '../../utils/time'
import { useEffect, useMemo, useState } from 'react'
import { NewButton as Button } from '../../common/NewButton'
import { classNames } from '../../utils/classnames'
import { Placement } from '@floating-ui/react'
import { PopoverButton } from '../../common/popper/PopoverButton'
import Calendar from './Calendar'
import { PresetPicker } from './PresetPicker'
import dayjs from 'dayjs'
import duration from 'dayjs/plugin/duration'
import relativeTime from 'dayjs/plugin/relativeTime'
import { LuCalendar, LuChevronDown } from 'react-icons/lu'
import { TimeZonePicker } from './TimeZonePicker'
import { TimeInput } from './TimeInput'
import { DATE_FORMAT, DateInput } from './DateInput'
import { formatTimeRange, applyTz } from './utils'
import { AutoRefreshButton } from './AutoRefreshButton'
import { Menu } from '@headlessui/react'
import {
  DefaultTimeConfirmDialog,
  type DefaultTimerangeValue
} from './DefaultTimeConfirmDialog'

dayjs.extend(duration)
dayjs.extend(relativeTime)

interface Props {
  startTime?: DateTimeValue
  endTime?: DateTimeValue
  tz?: string
  onChange: (start?: DateTimeValue, end?: DateTimeValue, tz?: string) => void
  onRefresh?: () => void
  skipApply?: boolean
  allowProjectEdit?: boolean
  placement?: Placement
  /** Auto-refresh interval in ms (0 = off); controlled and persisted by the consumer. */
  autoRefresh?: number
  onAutoRefreshChange?: (value: number) => void
  /** Current project default time range + a save callback, for "Apply as project settings". */
  defaultTimerange?: DefaultTimerangeValue
  onSaveDefault?: (defaultTimerange: DefaultTimerangeValue) => Promise<void>
}

const formatTimeInput = (value?: DateTimeValue, tz?: string) => {
  return dayjs.isDayjs(value) ? applyTz(value, tz).format('HH:mm') : ''
}

export default function TimeRangePicker({
  startTime,
  endTime,
  tz,
  onChange,
  onRefresh,
  skipApply = false,
  allowProjectEdit,
  placement,
  autoRefresh = 0,
  onAutoRefreshChange,
  defaultTimerange,
  onSaveDefault
}: Props) {
  const [start, setStart] = useState(startTime)
  const [end, setEnd] = useState(endTime)
  const [timeZone, setTimeZone] = useState(tz)
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false)

  const onTimeChange = (
    start?: DateTimeValue,
    end?: DateTimeValue,
    tz?: string
  ) => {
    setStart(start)
    setEnd(end)
    setTimeZone(tz)
  }

  useEffect(() => {
    onTimeChange(startTime, endTime, tz)
  }, [startTime, endTime, tz])

  const onSelectStartDate = (date: dayjs.Dayjs) => {
    if (dayjs.isDayjs(start)) {
      date = date.hour(start.hour()).minute(start.minute())
    }
    setStart(date)
  }

  const onSelectEndDate = (date: dayjs.Dayjs) => {
    if (dayjs.isDayjs(end)) {
      date = date.hour(end.hour()).minute(end.minute())
    }
    setEnd(date)
  }

  const onChangeStartDate = (value: string) => {
    let date: DateTimeValue | null = dayjs(value, DATE_FORMAT, true)
    if (!date.isValid()) {
      date = parseRelativeTime(value)
    }
    if (dayjs.isDayjs(date) && dayjs.isDayjs(start)) {
      date = date.hour(start.hour()).minute(start.minute())
    }
    if (date) {
      setStart(date)
    } else {
      setStart(undefined)
    }
  }

  const onChangeEndDate = (value: string) => {
    let date: DateTimeValue | null = dayjs(value, DATE_FORMAT, true)
    if (!date.isValid()) {
      date = parseRelativeTime(value)
    }
    if (dayjs.isDayjs(date) && dayjs.isDayjs(end)) {
      date = date.hour(end.hour()).minute(end.minute())
    }
    if (date) {
      setEnd(date)
    } else {
      setEnd(undefined)
    }
  }

  const onChangeStartTime = (value: string) => {
    if (!dayjs.isDayjs(start)) {
      return
    }
    const [hour, minute] = value.split(':')
    const date = applyTz(start, timeZone).hour(+hour).minute(+minute)
    setStart(date)
  }

  const onChangeEndTime = (value: string) => {
    if (!dayjs.isDayjs(end)) {
      return
    }
    const [hour, minute] = value.split(':')
    const date = applyTz(end, timeZone).hour(+hour).minute(+minute)
    setEnd(date)
  }

  const onChangeTimeZone = (tz?: string) => {
    if (dayjs.isDayjs(start)) {
      const v = applyTz(start, tz)
      const date = v
        .year(start.year())
        .month(start.month())
        .date(start.date())
        .hour(start.hour())
      setStart(date)
    }
    if (dayjs.isDayjs(end)) {
      const v = applyTz(end, tz)
      const date = v
        .year(end.year())
        .month(end.month())
        .date(end.date())
        .hour(end.hour())
      setEnd(date)
    }
    setTimeZone(tz)
  }

  const isClean =
    dateTimeToString(start) == dateTimeToString(startTime) &&
    dateTimeToString(end) == dateTimeToString(endTime) &&
    tz == timeZone
  const isValid = (!start && !end) || isBefore(start, end)

  const timeZoneStatus = useMemo(() => {
    if (timeZone) {
      try {
        const now = new Date()
        const dateString = Intl.DateTimeFormat([], {
          timeZone,
          timeZoneName: 'longOffset'
        }).format(now)
        const offset = dateString.split(' ')[1].replace('GMT', 'UTC')
        return (
          <span className="text-text-foreground rounded-sm border bg-gray-50 px-1 py-0.5 text-[11px] leading-[14px]">
            {offset}
          </span>
        )
      } catch (e) {
        // parse timezone failed
        return undefined
      }
    }
    return undefined
  }, [timeZone])

  const applyButton = (close: () => void) => (
    <div className="flex">
      <Button
        role="primary"
        size="md"
        onClick={() => {
          onChange(start, end, timeZone)
          close()
        }}
        disabled={isClean || !isValid}
        className={classNames(
          'w-[calc(100vw-4rem)] sm:w-fit',
          allowProjectEdit && 'rounded-r-none'
        )}
      >
        Apply
      </Button>
      {allowProjectEdit && (
        <Menu as="div" className="relative -ml-px block h-[30px]">
          {({ open }) => (
            <>
              <Menu.Button
                className={classNames(
                  'relative inline-flex h-[30px] items-center rounded-r-md',
                  'px-1.5 py-2 focus:z-10',
                  'bg-primary-600 border-l border-[#054BBD]',
                  open
                    ? 'bg-primary-600 text-white'
                    : 'hover:bg-primary-600 text-text-foreground-disabled bg-gray-100 ring-gray-300 hover:text-white'
                )}
              >
                <span className="sr-only">Open options</span>
                <LuChevronDown className="h-3 w-3" aria-hidden="true" />
              </Menu.Button>
              <Menu.Items className="focus:outline-hidden bg-default-bg absolute right-0 z-10 -mr-1 mt-2 w-56 origin-top-right rounded-md shadow-lg ring-1 ring-black/5 dark:ring-gray-100">
                <div className="py-1">
                  <Menu.Item key={'refresh'}>
                    {({ active }) => (
                      <button
                        onClick={() => setConfirmDialogOpen(true)}
                        className={classNames(
                          active
                            ? 'text-text-foreground bg-gray-100'
                            : 'text-foreground',
                          'flex w-full justify-between px-4 py-2 text-sm'
                        )}
                      >
                        Apply as project settings
                      </button>
                    )}
                  </Menu.Item>
                </div>
              </Menu.Items>
            </>
          )}
        </Menu>
      )}
    </div>
  )

  return (
    <div className="flex w-full sm:w-fit sm:pr-0">
      <PopoverButton
        containerClassName="w-full sm:w-fit"
        placement={placement}
        content={({ close }) => (
          <>
            <div className="pl-37 grid grid-cols-1 justify-items-stretch sm:flex">
              <PresetPicker
                onSelect={(x, y) => {
                  onChange(x, y, timeZone)
                  close()
                }}
              />
              <div className="pb-px sm:p-4">
                <div className="hidden gap-3 sm:flex">
                  <Calendar
                    value={
                      dayjs.isDayjs(start)
                        ? start
                        : dayjs().subtract(1, 'month')
                    }
                    start={dayjs.isDayjs(start) ? start : undefined}
                    end={dayjs.isDayjs(end) ? end : undefined}
                    onSelect={onSelectStartDate}
                  />
                  <div className="border-r" />
                  <Calendar
                    value={dayjs.isDayjs(end) ? end : dayjs()}
                    start={dayjs.isDayjs(start) ? start : undefined}
                    end={dayjs.isDayjs(end) ? end : undefined}
                    onSelect={onSelectEndDate}
                  />
                </div>
                <div className="border-main rounded-md border sm:mt-4 sm:p-3">
                  <div className="text-ilabel grid grid-cols-1 justify-between gap-y-2 sm:flex sm:gap-y-0">
                    <label className="items-center space-y-2 sm:flex sm:space-y-0">
                      <span className="mr-2.5 font-medium">From</span>
                      <DateInput value={start} onChange={onChangeStartDate} />
                      <span className="hidden sm:inline">
                        <TimeInput
                          value={formatTimeInput(start, timeZone)}
                          disabled={!dayjs.isDayjs(start)}
                          onChange={onChangeStartTime}
                        />
                      </span>
                    </label>
                    <label className="items-center space-y-2 sm:flex sm:space-y-0">
                      <span className="mr-2.5 font-medium">To</span>
                      <DateInput value={end} onChange={onChangeEndDate} />
                      <span className="hidden sm:inline">
                        <TimeInput
                          value={formatTimeInput(end, timeZone)}
                          disabled={!dayjs.isDayjs(end)}
                          onChange={onChangeEndTime}
                        />
                      </span>
                    </label>
                  </div>
                  <div className="mt-2.5 grid grid-cols-1 justify-items-stretch gap-y-4 sm:flex sm:justify-between">
                    <TimeZonePicker
                      value={timeZone || ' '}
                      onChange={onChangeTimeZone}
                    />
                    <div className="grid grid-cols-1 justify-items-stretch gap-2 sm:flex">
                      <Button
                        role="secondary"
                        size="md"
                        disabled={isClean}
                        onClick={() => onTimeChange(startTime, endTime, tz)}
                        className="w-full sm:w-fit"
                      >
                        Reset
                      </Button>
                      {applyButton(close)}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </>
        )}
        contentClassName={classNames(
          'z-30 rounded-md',
          placement?.includes('top') ? '-mt-1' : 'mt-3',
          placement == 'bottom-end' && onRefresh && 'ml-[132px]'
        )}
        as="div"
      >
        <div
          className={classNames(
            'text-ilabel hover:bg-primary-600 hover:border-primary-600 border-main h-7.5 flex cursor-pointer items-center gap-1.5 border pl-2 pr-4 font-medium hover:text-white',
            onRefresh ? 'rounded-l-md border-r-0' : 'rounded-md',
            'flex-1 whitespace-nowrap',
            isValid
              ? isClean
                ? ''
                : 'text-primary-600'
              : !start && !end
                ? ''
                : 'bg-red-100'
          )}
          suppressHydrationWarning
        >
          <LuCalendar className="h-5 w-5 text-inherit" />
          {!start && !end && (
            <span className="text-xs">
              Not Set{!timeZone && ', Browser Time'}
            </span>
          )}
          {formatTimeRange(start, end, timeZone)}
          {timeZoneStatus}
        </div>
      </PopoverButton>

      {onRefresh && (
        <AutoRefreshButton
          autoRefresh={autoRefresh}
          onAutoRefreshChange={onAutoRefreshChange ?? (() => {})}
          onClick={() => {
            onTimeChange(startTime, endTime, tz)
            onRefresh()
          }}
        />
      )}
      {confirmDialogOpen && onSaveDefault && (
        <DefaultTimeConfirmDialog
          start={start}
          end={end}
          tz={timeZone}
          currentDefault={defaultTimerange}
          onSaveDefault={onSaveDefault}
          onConfirm={onChange}
          onClose={() => setConfirmDialogOpen(false)}
        />
      )}
    </div>
  )
}
