import { Fragment, ReactNode, useMemo } from 'react'
import dayjs from 'dayjs'
import utc from 'dayjs/plugin/utc'
import timezone from 'dayjs/plugin/timezone'
import { isNumber } from 'lodash'
import { LuCircleUserRound, LuList } from 'react-icons/lu'
import { CopyButton, classNames, type DurationLike } from '@sentio/ui-core'

dayjs.extend(utc)
dayjs.extend(timezone)

interface Props {
  data: any
  compareTimeDuration?: DurationLike
  numberFormatter: (value: number) => string
  highlightSeriesId?: string
  title?: ReactNode
  onViewLogs?: (seriesId: string, seriesIndex: number) => void
  viewLogDisabled?: (seriesId: string, seriesIndex: number) => boolean
  onViewUsers?: (seriesId: string, seriesIndex: number) => void
  viewUsersDisabled?: (seriesId: string, seriesIndex: number) => boolean
  isFixed?: boolean
  sizeTitle?: string
}

export function ScatterChartTooltip({
  data,
  numberFormatter,
  highlightSeriesId,
  title,
  onViewLogs,
  viewLogDisabled,
  onViewUsers,
  viewUsersDisabled,
  isFixed,
  sizeTitle = 'Size'
}: Props) {
  const { point, seriesName, seriesId, marker } = useMemo(() => {
    // For scatter charts, data is typically a single point
    const param = Array.isArray(data) ? data[0] : data

    return {
      point: param,
      seriesName: param?.seriesName || '',
      seriesId: param?.seriesId || '',
      marker: param?.marker || ''
    }
  }, [data])

  if (!point || !point.value) {
    return (
      <div className="w-full px-2">
        <div className="text-text-foreground-secondary pl-2 text-sm">
          No data available
        </div>
      </div>
    )
  }

  const { value } = point
  const [xValue, yValue, sizeValue] = value

  const highlighted = seriesId === highlightSeriesId
  const showViewLogs = onViewLogs && !viewLogDisabled?.(seriesId, 0)
  const showViewUsers = onViewUsers && !viewUsersDisabled?.(seriesId, 0)

  const formatValue = (val: any) => {
    if (val instanceof Date) {
      return dayjs(val).format('YYYY-MM-DD HH:mm:ss')
    } else if (isNumber(val)) {
      return numberFormatter(val)
    } else {
      return String(val)
    }
  }

  return (
    <div
      className={classNames('grid w-full px-2')}
      style={{ gridTemplateColumns: '1fr auto' }}
    >
      {/* Title */}
      <div
        className={classNames(
          'mb-2 pl-2',
          'text-text-foreground-secondary col-span-2 text-left'
        )}
      >
        {title ?? dayjs(xValue).format('YYYY-MM-DD HH:mm:ss')}
      </div>

      {/* Main Series Row */}
      <div
        className={classNames(
          'sentio-tooltip-item series-name text-text-foreground inline-flex items-center overflow-hidden',
          'group',
          highlighted ? 'highlighted' : ''
        )}
        style={{ minWidth: '4rem' }}
      >
        <span dangerouslySetInnerHTML={{ __html: marker || '' }}></span>
        <span className="truncate">{seriesName}</span>
        {showViewLogs && isFixed && (
          <button
            className="text-text-foreground/60 hover:text-text-foreground invisible ml-1 text-xs underline group-hover:visible"
            onClick={(e) => {
              e.preventDefault()
              e.stopPropagation()
              onViewLogs(seriesId, 0)
            }}
            title="View logs"
          >
            <LuList className="h-4 w-4" />
          </button>
        )}
        {showViewUsers && isFixed && (
          <button
            className="text-text-foreground/60 hover:text-text-foreground invisible ml-1 text-xs underline group-hover:visible"
            onClick={(e) => {
              e.preventDefault()
              e.stopPropagation()
              onViewUsers(seriesId, 0)
            }}
            title="View users"
          >
            <LuCircleUserRound className="h-4 w-4" />
          </button>
        )}
        {isFixed && (
          <CopyButton
            size={16}
            text={seriesName}
            className="invisible ml-1 group-hover:visible"
          />
        )}
      </div>

      {/* Y Value */}
      <div
        className={classNames(
          'sentio-tooltip-item min-w-16 flex items-center truncate pl-1 text-right font-semibold',
          highlighted ? 'highlighted' : ''
        )}
      >
        <span>{formatValue(yValue)}</span>
      </div>

      {/* Additional Dimensions */}
      {sizeValue !== undefined && sizeValue !== null && (
        <Fragment>
          <div className="border-border-color col-span-2 my-2 w-full border-t"></div>
          <div
            className="sentio-tooltip-item series-name text-text-foreground-secondary truncate"
            style={{ minWidth: '4rem' }}
          >
            {sizeTitle}
          </div>
          <div className="sentio-tooltip-item min-w-16 text-text-foreground-secondary truncate pl-1 text-right font-semibold">
            {formatValue(sizeValue)}
          </div>
        </Fragment>
      )}
    </div>
  )
}
