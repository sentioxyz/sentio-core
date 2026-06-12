import { DateTimeValue, timeRangeToDisplayString } from '../../utils/time'
import { classNames } from '../../utils/classnames'

interface Props {
  startTime: DateTimeValue
  endTime: DateTimeValue
  className?: string
}

export function TimeRangeLabel({ startTime, endTime, className }: Props) {
  const timeRange = timeRangeToDisplayString(startTime, endTime)
  return (
    <div
      className={classNames(
        className,
        'text-text-foreground truncate whitespace-nowrap rounded-sm bg-gray-100 px-2 py-0.5 text-xs font-medium'
      )}
      title={timeRange}
    >
      {timeRange}
    </div>
  )
}
