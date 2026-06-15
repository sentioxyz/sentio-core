import dayjs from 'dayjs'
import { LuCalendar, LuChevronDown } from 'react-icons/lu'
import { PopoverButton } from '../../common/popper/PopoverButton'
import Calendar from './Calendar'
import { classNames } from '../../utils/classnames'

interface Props {
  value?: dayjs.Dayjs
  onChange?: (value: dayjs.Dayjs) => void
  start?: dayjs.Dayjs
  end?: dayjs.Dayjs
  minDate?: dayjs.Dayjs
  maxDate?: dayjs.Dayjs
  className?: string
  placeholder?: string
  disabled?: boolean
  format?: string
}

export const DatePicker = ({
  value,
  onChange,
  start,
  end,
  minDate,
  maxDate,
  className,
  placeholder = 'Select date',
  disabled = false,
  format = 'YYYY-MM-DD'
}: Props) => {
  const displayValue = value ? value.utc().format(format) : ''
  const enableRangeLimit = !!(start && end)

  return (
    <PopoverButton
      content={({ close }) => {
        const handleDateSelect = (date: dayjs.Dayjs) => {
          onChange?.(date)
          close()
        }
        return (
          <div className="p-4">
            <Calendar
              value={value || dayjs().utc()}
              onSelect={handleDateSelect}
              start={start}
              end={end}
              minDate={minDate}
              maxDate={maxDate}
              enableRangeLimit={enableRangeLimit}
              className="w-full"
              useUTC
            />
          </div>
        )
      }}
      className={classNames(
        'text-icontent shadow-xs border-main flex w-full items-center justify-between rounded-md border px-3 py-2',
        'bg-default-bg dark:border-light',
        'focus:ring-primary-500 focus:border-primary-500 focus:outline-hidden focus:ring-1',
        'hover:border-dark dark:hover:border-dark',
        disabled && 'cursor-not-allowed opacity-50',
        !disabled && 'cursor-pointer',
        className
      )}
      contentClassName="bg-default-bg border border-light rounded-lg shadow-lg"
      portal={true}
      placement="bottom-start"
    >
      <div className="flex min-w-0 flex-1 items-center space-x-2">
        <LuCalendar className="text-text-foreground-disabled h-4 w-4 shrink-0" />
        <span
          className={classNames(
            'truncate',
            displayValue
              ? 'text-text-foreground'
              : 'text-text-foreground-secondary'
          )}
        >
          {displayValue || placeholder}
        </span>
      </div>
      {!disabled && (
        <LuChevronDown className="text-text-foreground-disabled h-4 w-4 shrink-0" />
      )}
    </PopoverButton>
  )
}
