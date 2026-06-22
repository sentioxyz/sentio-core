import { Fragment, ReactNode, useMemo } from 'react'
import dayjs from 'dayjs'
import utc from 'dayjs/plugin/utc'
import timezone from 'dayjs/plugin/timezone'
import { isNumber, some, sortBy } from 'lodash'
import BigDecimal from '@sentio/bigdecimal'
import { LuCircleUserRound, LuList } from 'react-icons/lu'
import { CopyButton, classNames, type DurationLike } from '@sentio/ui-core'
import { durationToLongString } from './duration'

dayjs.extend(utc)
dayjs.extend(timezone)

interface Props {
  data: any
  compareTimeDuration?: DurationLike
  numberFormatter: (value: number, seriesId?: string) => string
  highlightSeriesId?: string
  title?: ReactNode
  showTotal?: boolean
  onViewLogs?: (seriesId: string, seriesIndex: number) => void
  viewLogDisabled?: (seriesId: string, seriesIndex: number) => boolean
  onViewUsers?: (seriesId: string, seriesIndex: number) => void
  viewUsersDisabled?: (seriesId: string, seriesIndex: number) => boolean
  isFixed?: boolean
}

function isValidValue(value: any, includeZero: boolean) {
  if (includeZero) {
    return Number.isFinite(value)
  } else {
    return Number.isFinite(value) && value !== 0
  }
}

export function ChartTooltip({
  data,
  numberFormatter,
  compareTimeDuration,
  highlightSeriesId,
  title,
  showTotal,
  onViewLogs,
  viewLogDisabled,
  onViewUsers,
  viewUsersDisabled,
  isFixed
}: Props) {
  const {
    series,
    hasCompare,
    hasCurrent,
    currentTime,
    compareTime,
    markers,
    compareMarkers,
    total,
    compareTotal
  } = useMemo(() => {
    const params = sortBy(data, (p) => -p.value[1])
    const hasCompare = some(params, (param) =>
      param.seriesId.endsWith('_compare')
    )
    const seriesData: Record<string, any> = {}
    const markers: Record<string, string> = {}
    const compareMarkers: Record<string, string> = {}
    let currentTime: dayjs.Dayjs | undefined
    let compareTime: dayjs.Dayjs | undefined
    let total = new BigDecimal(0)
    let compareTotal = new BigDecimal(0)

    for (const p of params) {
      const { marker, seriesName, value, seriesId } = p
      if (seriesId.endsWith('_compare')) {
        const id = seriesId.replace('_compare', '')
        compareMarkers[id] = marker
        if (compareTime === undefined) {
          compareTime = dayjs(value[0])
        }
        if (isValidValue(value[1], hasCompare)) {
          seriesData[id] = {
            seriesId: id,
            ...seriesData[id],
            compareValue: value[1],
            compareTime: value[0],
            seriesName
          }
          compareTotal = compareTotal.plus(value[1])
        }
      } else {
        markers[seriesId] = marker
        if (currentTime === undefined) {
          currentTime = dayjs(value[0])
        }
        if (isValidValue(value[1], hasCompare)) {
          seriesData[seriesId] = {
            seriesId,
            ...seriesData[seriesId],
            time: value[0],
            value: value[1],
            seriesName
          }
          total = total.plus(value[1])
        }
      }
    }
    const series = sortBy(Object.values(seriesData), (s) => -s.value)
    const hasCurrent = series[0]?.value !== undefined
    if (compareTimeDuration && compareTime && !currentTime) {
      currentTime = compareTime.add(
        Number(compareTimeDuration.value!),
        compareTimeDuration.unit as any
      )
    }
    return {
      series,
      hasCompare,
      currentTime,
      compareTime,
      hasCurrent,
      markers,
      compareMarkers,
      total,
      compareTotal
    }
  }, [data])

  const renderRow = (p: any, idx: number) => {
    const { seriesName, compareValue, value, seriesId } = p
    const highlighted = seriesId === highlightSeriesId
    const marker = markers[seriesId]
    // const diff = undefined
    const diff =
      hasCompare && hasCurrent && compareValue != null && value != null
        ? new BigDecimal(value).minus(compareValue).div(compareValue).toNumber()
        : undefined

    const showViewLogs = onViewLogs && !viewLogDisabled?.(seriesId, idx)
    const showViewUsers = onViewUsers && !viewUsersDisabled?.(seriesId, idx)

    return (
      <Fragment key={idx}>
        <div
          key={idx}
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
                onViewLogs(seriesId, idx)
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
                onViewUsers(seriesId, idx)
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

        <div
          key={`${idx}-value`}
          className={classNames(
            'sentio-tooltip-item min-w-16 flex items-center truncate pl-1 text-right font-semibold',
            highlighted ? 'highlighted' : ''
          )}
        >
          <span>{hasCurrent ? numberFormatter(value, seriesId) : '-'}</span>
          {diff !== undefined && Number.isFinite(diff) && (
            <span
              className={classNames(
                'ml-1 text-xs',
                diff > 0 ? 'text-green-500' : 'text-red'
              )}
            >
              {diff > 0 ? '+' : ''}
              {(diff * 100).toFixed(2)}%
            </span>
          )}
        </div>
      </Fragment>
    )
  }

  const renderCompareRow = (p: any, idx: number) => {
    const { seriesName, compareValue, seriesId } = p
    const highlighted = seriesId === highlightSeriesId
    const compareMarker = compareMarkers[seriesId]
    return (
      <Fragment key={idx}>
        <div
          key={idx}
          className={classNames(
            'sentio-tooltip-item sentio-tooltip-compare-item series-name text-text-foreground-secondary truncate',
            highlighted ? 'highlighted' : ''
          )}
          style={{ minWidth: '4rem' }}
        >
          <span
            dangerouslySetInnerHTML={{ __html: compareMarker || '' }}
          ></span>
          {seriesName}
        </div>

        <div
          key={`${idx}-value`}
          className={classNames(
            'sentio-tooltip-item min-w-16 text-text-foreground-secondary truncate pl-1 text-right font-semibold',
            highlighted ? 'highlighted' : ''
          )}
        >
          {isNumber(compareValue)
            ? numberFormatter(compareValue, seriesId)
            : '-'}
        </div>
      </Fragment>
    )
  }

  const renderTotalRow = () => {
    if (!showTotal || series.length < 2) return null

    const diff =
      hasCompare && hasCurrent && total && compareTotal
        ? total.minus(compareTotal).div(compareTotal).toNumber()
        : undefined

    return (
      <div className="border-border-color col-span-2 mt-1 flex items-center justify-between border-t pt-1">
        <div className="sentio-tooltip-item series-name text-text-foreground truncate font-semibold">
          Total
        </div>
        <div className="sentio-tooltip-item min-w-16 flex items-center truncate pl-1 text-right font-semibold">
          <span>{hasCurrent ? numberFormatter(total.toNumber()) : '-'}</span>
          {diff !== undefined && Number.isFinite(diff) && (
            <span
              className={classNames(
                'ml-1 text-xs',
                diff > 0 ? 'text-green-500' : 'text-red'
              )}
            >
              {diff > 0 ? '+' : ''}
              {(diff * 100).toFixed(2)}%
            </span>
          )}
        </div>
      </div>
    )
  }

  const renderCompareTotalRow = () => {
    if (!showTotal || series.length < 2 || !hasCompare) return null

    return (
      <div className="border-border-color col-span-2 mt-1 flex items-center justify-between border-t pt-1">
        <div className="sentio-tooltip-item sentio-tooltip-compare-item series-name text-text-foreground-secondary truncate font-semibold">
          Total
        </div>
        <div className="sentio-tooltip-item min-w-16 text-text-foreground-secondary truncate pl-1 text-right font-semibold">
          {isNumber(compareTotal)
            ? numberFormatter(compareTotal.toNumber())
            : '-'}
        </div>
      </div>
    )
  }

  return (
    <div
      className={classNames('grid w-full px-2')}
      style={{ gridTemplateColumns: '1fr auto' }}
    >
      <div
        className={classNames(
          'pl-2',
          'text-text-foreground-secondary col-span-2 text-left'
        )}
      >
        {title ?? currentTime?.format('YYYY-MM-DD HH:mm:ss')}
      </div>
      {!series || series.length === 0 ? (
        <div className="text-text-foreground-secondary pl-2 text-sm">
          No data available
        </div>
      ) : (
        <>
          {series.map((s, idx) => renderRow(s, idx))}
          {renderTotalRow()}
          {hasCompare && compareTimeDuration && (
            <>
              <div
                className={classNames(
                  'mt-2 pl-2',
                  'text-text-foreground-secondary col-span-2 text-left'
                )}
              >
                {compareTime?.format('YYYY-MM-DD HH:mm:ss')} (previous{' '}
                {durationToLongString(compareTimeDuration)})
              </div>
              {series.map((s, idx) => renderCompareRow(s, idx))}
              {renderCompareTotalRow()}
            </>
          )}
        </>
      )}
    </div>
  )
}
