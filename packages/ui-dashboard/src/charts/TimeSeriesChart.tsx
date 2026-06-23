import {
  CSSProperties,
  forwardRef,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState
} from 'react'
import dayjs from 'dayjs'
import type { ECharts } from 'echarts/core'
import { EChartsHandle, EChartsOption, ReactEChartsBase } from './EchartsBase'
import {
  dateRangeOfSeries,
  durationToSeconds,
  calculateStepByDate,
  formatTime,
  shiftTimezone
} from './series-utils'
import { alignTime, pickRangeByInterval, timeBefore } from './time-utils'
import {
  toDayjs,
  fromNumber,
  dateTimeToString,
  pickRangeByTimeRange,
  classNames,
  PopupMenuButton,
  type DateTimeValue,
  type DurationLike
} from '@sentio/ui-core'
import YaxisControls from './options/YaxisControls'
import { XAxisControls } from './options/XaxisControls'
import { sansFontFamily } from './theme/sentio-theme'
import { defaultConfig as DefaultValueConfig } from './options/ValueOptions'
import { sentioColors } from './theme/sentio-colors'
import { useDarkMode } from '../utils/use-dark-mode'
import { ChartTooltip } from './ChartTooltip'
import { ScatterChartTooltip } from './ScatterChartTooltip'
import ReactDOMServer from 'react-dom/server'
import { defaults, isArray, isEqual, isNumber, uniq } from 'lodash'
import {
  flip,
  FloatingOverlay,
  FloatingPortal,
  shift,
  useFloating
} from '@floating-ui/react'
import { useBoolean } from '@sentio/ui-core'
import { LuChevronDown } from 'react-icons/lu'
import type {
  ChartConfigLike,
  ChartTypeLike,
  YAxisConfigLike,
  XAxisConfigLike,
  SeriesLike,
  SeriesDataLike
} from '../types'

const NumberFormat = (o: Intl.NumberFormatOptions) =>
  new Intl.NumberFormat('en-US', o)

function roundByDPR(value: number) {
  const dpr = window.devicePixelRatio || 1
  return Math.round(value * dpr) / dpr
}

function onClickPreventDefault(e: React.MouseEvent) {
  e.preventDefault()
  e.stopPropagation()
}

const closestNumber = (arr: number[] | undefined, needle: number) => {
  return arr?.reduce((a, b) =>
    Math.abs(b - needle) < Math.abs(a - needle) ? b : a
  )
}

interface Mark {
  value?: number
  label?: string
  above?: boolean
  below?: boolean
  color?: string
  from?: Date
  to?: Date
}

export type MarkLine = Mark
export type MarkArea = Omit<Mark, 'above' | 'below' | 'value'>

/** Minimal structural mirror of the proto SegmentationQuery message. */
export interface SegmentationQueryLike {
  id?: string
  selectorExpr?: unknown
  resource?: { name?: string }
}

/** Overlay outputs injected by the app (computed by its overlay-series hook). */
export interface TimeSeriesOverlay {
  seriesIdToYAxisName: Map<string, string>
  overlaySeriesIdToFormatter: Map<string, (v: number) => string>
  applyOverlaySeries: (option: EChartsOption) => EChartsOption
}

const initialConfig: ChartConfigLike = {
  xAxis: {
    type: 'category',
    min: '',
    max: '',
    scale: true,
    name: '',
    column: ''
  },
  yAxis: {
    min: '',
    max: '',
    scale: true
  }
}

// Helper function to create result object with optional name
const createYAxisResult = (
  min: any,
  max: any,
  scale?: boolean,
  name?: string
) => {
  const result: any = { min, max, scale }
  if (name) {
    result.name = name
  }
  return result
}

// Helper function to calculate margin for axis values
const calculateMargin = (value: number, isMin: boolean) => {
  if (isMin) {
    return value >= 0 ? Math.max(0, value * 0.9) : value * 1.1
  } else {
    return value >= 0 ? value * 1.1 : value * 0.9
  }
}

const yAxisToChartOption = (
  option?: YAxisConfigLike,
  markLines: MarkLine[] = [],
  values?: SeriesLike<Date>[]
) => {
  const { min, max, scale, name } = option || {}

  // Parse option min/max values early
  const hasValidMin = min !== undefined && min !== ''
  const hasValidMax = max !== undefined && max !== ''
  const optionMin = hasValidMin ? parseFloat(min as string) || min : undefined
  const optionMax = hasValidMax ? parseFloat(max as string) || max : undefined

  // If both min and max are set in option, use them directly
  if (hasValidMin && hasValidMax) {
    return createYAxisResult(optionMin, optionMax, scale, name)
  }

  // Only calculate when markLines has valid non-zero values
  const lineValues = markLines
    .map((m) => m.value)
    .filter(
      (v) => v !== undefined && v !== 0 && (v as any as string) !== '0'
    ) as number[]

  // If no valid markLines values and no option values, return undefined for min and max
  if (lineValues.length === 0 && !hasValidMin && !hasValidMax) {
    return createYAxisResult(undefined, undefined, scale, name)
  }

  // Collect all values for calculation
  const allValues = [...lineValues]
  if (values) {
    const valueSet = new Set<number>()
    values.forEach((series) => {
      series.data.forEach((point) => {
        const value = point[1]
        if (isNumber(value)) {
          valueSet.add(value)
        }
      })
    })
    valueSet.forEach((value) => {
      allValues.push(value)
    })
  }

  // Calculate final min/max values
  let finalMin = optionMin
  let finalMax = optionMax

  if (allValues.length > 0) {
    // Calculate min if not set in option
    if (!hasValidMin) {
      const minValue = allValues.reduce(
        (min, current) => Math.min(min, current),
        Infinity
      )
      finalMin = calculateMargin(minValue, true)
    }

    // Calculate max if not set in option
    if (!hasValidMax) {
      const maxValue = allValues.reduce(
        (max, current) => Math.max(max, current),
        -Infinity
      )
      finalMax = calculateMargin(maxValue, false)
    }
  }

  return createYAxisResult(finalMin, finalMax, scale, name)
}

const getEventName = (name: string) => {
  let eventName: undefined | string = name.split('-')[0].trim()
  if (eventName.toLowerCase() === 'all events') {
    eventName = undefined
  }

  return eventName
}

// In dark mode, area fills at ECharts' default opacity (0.7) blend together and
// obscure each other when multiple areas are stacked or overlapping, making them
// hard to tell apart against the dark background. Lower the fill opacity so the
// areas (and the series behind them) stay distinguishable. Only kick in when
// there is more than one series; a single area keeps the default 0.7.
const DARK_AREA_FILL_OPACITY = 0.35

const applyDarkAreaOpacity = (series: any[]): any[] => {
  if (series.length <= 1) {
    return series
  }
  return series.map((s) => {
    if (!s?.areaStyle) {
      return s
    }
    const existing =
      typeof s.areaStyle.opacity === 'number' ? s.areaStyle.opacity : 0.7
    return {
      ...s,
      areaStyle: {
        ...s.areaStyle,
        opacity: Math.min(existing, DARK_AREA_FILL_OPACITY)
      }
    }
  })
}

/**
 * ECharts does not support time series with gaps, make sure all the series have same time series
 * @param series source series
 * @returns
 */
export const fixTimeSeries = (series: SeriesLike<Date>[]): void => {
  try {
    if (!series?.length) {
      return
    }

    const isValidDateSeries = series.every((s) =>
      s.data?.every((point) => {
        const date = point?.[0]
        return date instanceof Date && !isNaN(date.getTime())
      })
    )

    if (
      !isValidDateSeries ||
      series.length < 2 ||
      uniq(series.map((s) => s.data?.length)).length === 1
    ) {
      return
    }

    const timeIndex: Date[] = []
    const dataIndexes = new Array(series.length).fill(0)

    while (
      dataIndexes.some((d, index) => d < (series[index]?.data?.length || 0))
    ) {
      const times = dataIndexes.map((d, index) => {
        const date = series[index]?.data?.[d]?.[0]
        return date instanceof Date && !isNaN(date.getTime())
          ? date.getTime()
          : Infinity
      })
      const minTime = Math.min(...times)

      if (!isFinite(minTime)) {
        break
      }

      const newDate = new Date(minTime)
      if (isNaN(newDate.getTime())) {
        break
      }

      timeIndex.push(newDate)
      dataIndexes.forEach((d, index) => {
        const date = series[index]?.data?.[d]?.[0]
        if (
          date instanceof Date &&
          !isNaN(date.getTime()) &&
          date.getTime() === minTime
        ) {
          dataIndexes[index]++
        }
      })
    }

    series.forEach((s) => {
      if (!s?.data) return

      const data = s.data
      const fixedData: SeriesDataLike<Date>[] = []
      let index = 0

      timeIndex.forEach((t) => {
        if (
          data[index]?.[0] instanceof Date &&
          !isNaN(data[index][0].getTime()) &&
          isEqual(data[index][0], t)
        ) {
          fixedData.push(data[index])
          index++
        } else {
          fixedData.push([t, null])
        }
      })

      s.data = fixedData
    })
  } catch (error) {
    console.warn('Error in fixTimeSeries:', error)
  }
}

/**
 * Context built by the presentational chart for a hovered/clicked series, handed
 * out to the app so it can build the log/cohort URL + navigation. The app combines
 * `value` (the data point, holding the timestamp) with its own time range.
 */
export interface ViewActionContext {
  value: any
  labels: Record<string, string>
  name: string
  id?: string
  isEventSeries: boolean
  eventName?: string
  query?: { selectorExpr?: unknown }
}

export interface TimeSeriesChartProps {
  // ── Display / behaviour fields (Like-typed mirror of the app's ChartProps) ──
  group?: string
  title?: string
  startTime?: DateTimeValue
  endTime?: DateTimeValue
  tz?: string
  minHeight?: number
  onSelectTimeRange?: (start: DateTimeValue, end: DateTimeValue) => void
  loading?: boolean
  controls?: boolean
  config?: ChartConfigLike
  onChangeConfig?: (config: ChartConfigLike) => void
  style?: CSSProperties
  chartType?: ChartTypeLike
  allowClick?: boolean
  showSymbol?: boolean
  sourceType?: string
  getEventNameById?: (id?: string) => string
  noLegend?: boolean
  onInitChart?: (chart: ECharts) => void
  markAreas?: MarkArea[]
  markLines?: MarkLine[]
  nonXAxis?: boolean

  // ── Injected (replacing app/runtime couplings) ──
  /** Already-computed primary series (app runs the worker compute). */
  series: SeriesLike<Date>[]
  /** Already-computed compare-period series (already styled by the app, or styled here). */
  compareSeries?: SeriesLike<Date>[]
  /** Legend entries for the primary series. */
  legend?: string[]
  /** Formats a value for axis/tooltip display (replaces lib/metrics/formatter). */
  numberFormatter: (value: number, seriesId?: string) => string
  /** Overlay outputs computed by the app (replaces the overlay-series hook). */
  overlay?: TimeSeriesOverlay
  /** Template-variable keys (replaces the app's template-variables atom). */
  templateVariableKeys?: string[]
  /**
   * Per-series metric labels, indexed by series index (the worker compute's
   * `seriesToMetricLabels`; the original read `labelsRef.current[idx]`).
   */
  seriesToMetricLabels?: {
    name: string
    labels: Record<string, string>
    id?: string
  }[]
  /** Map of series id → event name (the original `eventNameMapRef.current`). */
  eventNameMap?: Map<string, string>
  /** Map of series id → events query (the original `eventsQueryMapRef.current`). */
  eventsQueryMap?: Map<string, { selectorExpr?: unknown }>
  /**
   * Navigate to logs for a clicked series. The presentational builds the context
   * (from the hovered tooltip value + injected labels/event maps) and hands it out;
   * the app maps it to a URL + navigation using its own time range.
   */
  onViewLogs?: (ctx: ViewActionContext) => void
  /** Navigate to users/accounts for a clicked series (the app builds the cohort query). */
  onViewUsers?: (ctx: ViewActionContext) => void
  /**
   * Whether the "View Logs" action should be disabled. The component computes the
   * context (may be null) and passes it; the app decides disabled.
   */
  viewLogDisabled?: (ctx: ViewActionContext | null) => boolean
  /** Whether the "View Users" action should be disabled. */
  viewUsersDisabled?: (ctx: ViewActionContext | null) => boolean
  /** Counts of returned/total samples for the data-truncation banner. */
  returnedSeries?: number
  totalSeries?: number
}

const TimeSeriesChart = forwardRef<EChartsHandle, TimeSeriesChartProps>(
  (props: TimeSeriesChartProps, ref) => {
    const yAxisRef = useRef<any>(null)
    const {
      group,
      title,
      startTime,
      endTime,
      tz,
      minHeight,
      onSelectTimeRange,
      loading,
      controls,
      config,
      markAreas,
      markLines,
      onChangeConfig,
      style,
      chartType,
      allowClick,
      sourceType,
      getEventNameById,
      noLegend: _noLegend = false,
      onInitChart,
      nonXAxis,
      // injected
      series: seriesProp,
      compareSeries: compareSeriesProp,
      legend: legendProp,
      numberFormatter,
      overlay,
      templateVariableKeys: _templateVariableKeys,
      seriesToMetricLabels,
      eventNameMap,
      eventsQueryMap,
      onViewLogs,
      onViewUsers,
      viewLogDisabled: viewLogDisabledProp,
      viewUsersDisabled: viewUsersDisabledProp,
      returnedSeries,
      totalSeries
    } = props

    void _templateVariableKeys

    const seriesToMetricLabelsRef = useMemo(
      () => seriesToMetricLabels ?? [],
      [seriesToMetricLabels]
    )
    const eventNameMapRef = useMemo(
      () => eventNameMap ?? new Map<string, string>(),
      [eventNameMap]
    )
    const eventsQueryMapRef = useMemo(
      () => eventsQueryMap ?? new Map<string, { selectorExpr?: unknown }>(),
      [eventsQueryMap]
    )

    const [yAxis, setYAxis] = useState(config?.yAxis || initialConfig.yAxis)
    const [xAxis, setXAxis] = useState(config?.xAxis || initialConfig.xAxis)
    const [maximumSignificantDigits, setMaximumSignificantDigits] = useState(3)
    const isDarkMode = useDarkMode()

    const minRef = useRef<any>(0)
    const maxRef = useRef<any>(0)
    const getSymbolSize = useCallback(
      (value: SeriesDataLike<Date>) => {
        const val = value[2] || 0
        return normalize(
          val,
          minRef.current,
          maxRef.current,
          config?.scatterConfig?.minSize,
          config?.scatterConfig?.maxSize
        )
      },
      [config?.scatterConfig?.minSize, config?.scatterConfig?.maxSize]
    )

    useEffect(() => {
      setYAxis(config?.yAxis || initialConfig.yAxis)
    }, [config])

    const legend = useMemo(() => legendProp || [], [legendProp])

    // Post-process the (already computed) primary series. This used to run on the
    // worker-compute result; now it runs on the injected `series` prop.
    const series = useMemo<SeriesLike<Date>[]>(() => {
      const result = (seriesProp || []).map((s) => ({ ...s }))
      computeMarkLines(result, markLines || [])
      computeMarksAreas(result, markAreas || [])
      if (config?.yAxis?.stacked) {
        fixTimeSeries(result)
        result.forEach((s) => {
          if (config?.seriesConfig && config.seriesConfig.series?.[s.name]) {
            return
          }
          s.stack = 'Total'
          s.stackStrategy = config?.yAxis?.stacked
        })
      }
      if (config?.lineConfig) {
        result.forEach((s) => {
          s.lineStyle = {
            ...s.lineStyle,
            type: config?.lineConfig?.style?.toLowerCase() || 'solid'
          }
          if (config?.lineConfig?.smooth) {
            s.smooth = true
          }
        })
      }
      if (chartType === 'SCATTER') {
        result.forEach((s) => {
          const min = s.data?.reduce((min, p) => {
            const val = p[2] || 0
            return Math.min(min, val)
          }, Infinity)
          const max = s.data?.reduce((max, p) => {
            const val = p[2] || 0
            return Math.max(max, val)
          }, -Infinity)
          minRef.current = min
          maxRef.current = max
          s.symbolSize = getSymbolSize
        })
      }
      if (allowClick) {
        result.forEach((serie) => {
          serie.emphasis =
            serie.type === 'bar'
              ? {
                  itemStyle: {
                    shadowColor: 'rgba(0, 0, 0, 0.3)',
                    shadowBlur: 10
                  }
                }
              : {
                  scale: 1.5
                }
        })
      }
      return result
    }, [
      seriesProp,
      markAreas,
      markLines,
      chartType,
      config?.yAxis,
      config?.lineConfig,
      config?.seriesConfig,
      allowClick,
      getSymbolSize
    ])

    // Reset the max-significant-digits whenever the data changes (the worker
    // effect did this when it set series; we key it on the series prop instead).
    useEffect(() => {
      setMaximumSignificantDigits(3)
    }, [seriesProp])

    // Post-process the compare series (dashed/grey, secondary x-axis).
    const compareSeries = useMemo<SeriesLike<Date>[]>(() => {
      if (!compareSeriesProp?.length) {
        return []
      }
      const result = compareSeriesProp.map((s) => ({ ...s }))
      for (const s of result) {
        s.lineStyle = {
          ...s.lineStyle,
          type: 'dashed'
        }
        s.itemStyle = {
          ...s.itemStyle,
          color: 'rgba(120, 120, 120, 0.3)'
        }
        s.xAxisIndex = 1
        if (chartType === 'SCATTER') {
          s.symbolSize = getSymbolSize
        }
      }
      return result
    }, [compareSeriesProp, chartType, getSymbolSize])

    const NF = NumberFormat({ notation: 'compact', maximumSignificantDigits })
    const NF_LARGE = NumberFormat({ notation: 'scientific' })

    const seriesIdToYAxisName =
      overlay?.seriesIdToYAxisName ?? new Map<string, string>()
    const overlaySeriesIdToFormatter =
      overlay?.overlaySeriesIdToFormatter ??
      new Map<string, (v: number) => string>()
    const applyOverlaySeries =
      overlay?.applyOverlaySeries ?? ((options: EChartsOption) => options)

    const setYAxisWrap = (yAxis: YAxisConfigLike) => {
      setYAxis(yAxis)
      onChangeConfig?.({ ...config, yAxis })
    }

    let yAxisLabel: number | undefined

    const [start, end] = useMemo(() => {
      const [dataStart, dataEnd] = dateRangeOfSeries(series)
      const selectStart = startTime && toDayjs(startTime, true).toDate()
      const selectEnd = endTime && toDayjs(endTime, false).toDate()
      const selStart = selectStart ? shiftTimezone(selectStart, tz) : undefined
      const selEnd = selectEnd ? shiftTimezone(selectEnd, tz) : undefined
      const start =
        dataStart && selStart
          ? dataStart.getTime() < selStart.getTime()
            ? dataStart
            : selStart
          : selStart
      const end =
        dataEnd && selEnd
          ? dataEnd.getTime() > selEnd.getTime()
            ? dataEnd
            : selEnd
          : selEnd
      return [start, end]
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [series, startTime, endTime])

    const xLabelFormatter = useMemo(() => {
      return config?.xAxis?.type === 'category'
        ? (value: Date) => {
            const date = shiftTimezone(value, tz)
            let interval = (config?.timeRangeOverride?.timeRange as any)
              ?.interval as DurationLike | undefined
            if (!interval && start && end) {
              interval = calculateStepByDate(start, end)
            }
            return formatTime(date, tz, interval)
          }
        : undefined
    }, [
      config?.xAxis?.type,
      config?.timeRangeOverride?.timeRange,
      tz,
      start,
      end
    ])

    const tooltipParmsRef = useRef<any>({})

    // Build the per-series context the app needs to construct the log/cohort URL
    // and navigation. Sources the hovered/clicked data value from the internal
    // tooltipParmsRef, and labels / event name / query from the injected props
    // (the original read these from labelsRef / eventNameMapRef / eventsQueryMapRef).
    const getSeriesContext = useCallback(
      (seriesId: string, seriesIndex: number): ViewActionContext | null => {
        const params = tooltipParmsRef.current?.data
        if (!params || !seriesToMetricLabelsRef) {
          return null
        }

        let value: any
        let idx: number

        // Find the correct series data
        if (Array.isArray(params)) {
          const param = params.find(
            (p: any) => p.seriesId === seriesId || p.seriesIndex === seriesIndex
          )
          if (!param) return null
          value = param.value
          idx = param.seriesIndex
        } else {
          value = params.value
          idx = seriesIndex
        }

        if (!value) return null

        const {
          labels = {},
          name = '',
          id
        } = seriesToMetricLabelsRef[idx] || {}
        const isEventSeries =
          eventNameMapRef.has(id as string) || sourceType === 'ANALYTICS'

        let eventName: string | undefined
        if (sourceType === 'ANALYTICS') {
          try {
            const _eventName = getEventNameById?.(id)
            eventName =
              _eventName == undefined ? getEventName(name) : _eventName
          } catch {
            //do nothing
          }
        } else if (eventNameMapRef.has(id as string)) {
          eventName = eventNameMapRef.get(id as string) || undefined
        }

        const query = eventsQueryMapRef.get(id as string)

        return {
          value,
          labels,
          name,
          id,
          isEventSeries,
          eventName,
          query
        }
      },
      [
        seriesToMetricLabelsRef,
        eventNameMapRef,
        eventsQueryMapRef,
        sourceType,
        getEventNameById
      ]
    )

    // Internal handlers passed to ChartTooltip (which calls them with
    // (seriesId, seriesIndex)). They build the context and delegate to the
    // outward props; the app computes time ranges / URLs / navigation itself.
    const handleViewLogs = useCallback(
      (seriesId: string, seriesIndex: number) => {
        const ctx = getSeriesContext(seriesId, seriesIndex)
        if (!ctx) return
        onViewLogs?.(ctx)
      },
      [getSeriesContext, onViewLogs]
    )

    const handleViewUsers = useCallback(
      (seriesId: string, seriesIndex: number) => {
        const ctx = getSeriesContext(seriesId, seriesIndex)
        if (!ctx) return
        onViewUsers?.(ctx)
      },
      [getSeriesContext, onViewUsers]
    )

    const getViewLogDisabled = useCallback(
      (seriesId: string, seriesIndex: number) => {
        const ctx = getSeriesContext(seriesId, seriesIndex)
        return viewLogDisabledProp?.(ctx) ?? false
      },
      [getSeriesContext, viewLogDisabledProp]
    )

    const getViewUsersDisabled = useCallback(
      (seriesId: string, seriesIndex: number) => {
        const ctx = getSeriesContext(seriesId, seriesIndex)
        return viewUsersDisabledProp?.(ctx) ?? false
      },
      [getSeriesContext, viewUsersDisabledProp]
    )

    const tooltipFormatter = useCallback(
      (params: any /*ticket, callback*/) => {
        // Normalize params to always be an array for consistent handling
        const paramsArray = Array.isArray(params) ? params : [params]

        if (allowClick) {
          yAxisRef.current = paramsArray.map((param: any) => ({
            value: param.value,
            seriesIndex: param.seriesIndex,
            seriesName: param.seriesName
          }))
        }
        let title: string | undefined = undefined
        if (nonXAxis) {
          const value = paramsArray?.[0]?.data?.[0]
          title = `${config?.xAxis?.column ? `${config?.xAxis?.column}:` : ''} ${value}`
        } else if (xLabelFormatter) {
          const value = paramsArray?.[0]?.data?.[0]
          title = xLabelFormatter(value)
        }

        // Append Y-axis name suffix to overlay series in tooltip for disambiguation
        const annotatedParams = paramsArray.map((param: any) => {
          const yAxisName = seriesIdToYAxisName.get(param.seriesId)
          if (yAxisName) {
            return { ...param, seriesName: `${param.seriesName}(${yAxisName})` }
          }
          return param
        })

        const parmas = {
          data: chartType === 'SCATTER' ? params : annotatedParams,
          title,
          highlightSeriesId: (global as any).highlightSeriesId,
          compareTimeDuration: config?.timeRangeOverride?.compareTime?.ago as
            | DurationLike
            | undefined,
          numberFormatter: (value: number, seriesId?: string) => {
            const seriesFormatter = seriesId
              ? overlaySeriesIdToFormatter.get(seriesId)
              : undefined
            if (seriesFormatter) {
              return seriesFormatter(value)
            }
            return numberFormatter(value) as string
          },
          showTotal: config?.valueConfig?.tooltipTotal,
          // Gate on the outward props existing (same as the original gating on the
          // handlers existing): no handler → button doesn't render.
          onViewLogs: onViewLogs ? handleViewLogs : undefined,
          viewLogDisabled: onViewLogs ? getViewLogDisabled : undefined,
          onViewUsers: onViewUsers ? handleViewUsers : undefined,
          viewUsersDisabled: onViewUsers ? getViewUsersDisabled : undefined,
          sizeTitle: config?.scatterConfig?.symbolSize
        }
        tooltipParmsRef.current = parmas

        // Use specialized component for scatter charts
        if (chartType === 'SCATTER') {
          return ReactDOMServer.renderToString(
            <ScatterChartTooltip
              {...parmas}
              onViewLogs={undefined}
              viewLogDisabled={undefined}
              onViewUsers={undefined}
              viewUsersDisabled={undefined}
              sizeTitle={config?.scatterConfig?.symbolSize}
            />
          )
        } else {
          return ReactDOMServer.renderToString(
            <ChartTooltip
              {...parmas}
              onViewLogs={undefined}
              viewLogDisabled={undefined}
              onViewUsers={undefined}
              viewUsersDisabled={undefined}
            />
          )
        }
      },
      [
        allowClick,
        numberFormatter,
        overlaySeriesIdToFormatter,
        config,
        nonXAxis,
        chartType,
        xLabelFormatter,
        onViewLogs,
        handleViewLogs,
        getViewLogDisabled,
        onViewUsers,
        handleViewUsers,
        getViewUsersDisabled,
        seriesIdToYAxisName
      ]
    )

    // the brush event will be fired from all grouped charts. So we need to check if the event is from this chart
    const [active, setActive] = useState(false)
    const onSelect = useCallback(
      (start: number, end: number) => {
        if (onSelectTimeRange && active) {
          let startTime: DateTimeValue = fromNumber(start)
          let endTime: DateTimeValue = fromNumber(end)
          const interval = (config?.timeRangeOverride?.timeRange as any)
            ?.interval as DurationLike | undefined
          if (interval) {
            startTime = alignTime(startTime, interval, tz, 'start')
            endTime = alignTime(endTime, interval, tz, 'end')
          }
          onSelectTimeRange(startTime, endTime)
        }
      },
      // eslint-disable-next-line react-hooks/exhaustive-deps
      [onSelectTimeRange, active]
    )

    const allowBrush = useMemo(() => {
      if (nonXAxis) {
        return false
      }
      if (onSelectTimeRange && startTime && endTime) {
        const interval = (config?.timeRangeOverride?.timeRange as any)
          ?.interval as DurationLike | undefined
        const diff = toDayjs(endTime).diff(toDayjs(startTime), 'seconds')
        return interval ? diff > durationToSeconds(interval) : true
      }
      return false
    }, [
      config?.timeRangeOverride?.timeRange,
      startTime,
      endTime,
      onSelectTimeRange,
      nonXAxis
    ])

    const xAxisData = useMemo(() => {
      if (nonXAxis) {
        return [
          {
            type: 'category',
            axisLabel: {
              hideOverlap: true,
              fontSize: 11
            },
            name: config?.xAxis?.name
          }
        ]
      }

      const ret = [
        {
          type: config?.xAxis?.type || 'time',
          min: config?.xAxis?.type === 'category' ? undefined : start,
          max: config?.xAxis?.type === 'category' ? undefined : end,
          axisLabel: {
            hideOverlap: true,
            fontSize: 11,
            formatter: xLabelFormatter
          },
          axisTick: {
            show: false
          }
        } as any
      ]
      if (config?.xAxis?.name) {
        ret[0].name = config?.xAxis?.name
      }
      if (config?.timeRangeOverride?.compareTime?.ago) {
        const d = config?.timeRangeOverride?.compareTime?.ago as DurationLike
        const compareStart = toDayjs(timeBefore(dayjs(start), d, true)).toDate()
        const compareEnd = toDayjs(timeBefore(dayjs(end), d, false)).toDate()
        ret.push({
          show: false,
          type: config?.xAxis?.type || 'time',
          min: config?.xAxis?.type === 'category' ? undefined : compareStart,
          max: config?.xAxis?.type === 'category' ? undefined : compareEnd,
          axisLabel: {
            hideOverlap: true,
            fontSize: 11,
            formatter: xLabelFormatter
          }
        } as any)
      }
      return ret as any
    }, [
      config?.timeRangeOverride?.compareTime,
      start,
      end,
      nonXAxis,
      config?.xAxis,
      xLabelFormatter
    ])

    const options: EChartsOption = useMemo(() => {
      const primarySeries =
        xAxisData.length == 2 ? [...series, ...compareSeries] : series

      // Build primary Y-axis
      const primaryYAxis: any = {
        type: 'value',
        axisLabel: {
          formatter: function (value: number) {
            if (value == yAxisLabel && maximumSignificantDigits < 21) {
              setMaximumSignificantDigits(maximumSignificantDigits + 1)
            }
            yAxisLabel = value
            if (value > 1e21) {
              return NF_LARGE.format(value)
            }
            return NF.format(value)
          },
          margin:
            chartType === 'SCATTER'
              ? config?.scatterConfig?.maxSize
                ? config.scatterConfig.maxSize
                : 30
              : 4
        },
        ...yAxisToChartOption(
          yAxis,
          markLines,
          primarySeries as SeriesLike<Date>[]
        )
      }

      const baseOptions: EChartsOption = {
        title: {
          text: title
        },
        grid: {
          top: title ? 48 : 16,
          right: chartType === 'SCATTER' ? 16 : 8,
          bottom: 8,
          left: chartType === 'SCATTER' ? 20 : 12,
          containLabel: true
        },
        xAxis: xAxisData,
        dataZoom: {
          type: 'inside',
          zoomLock: true
        },
        legend: {
          data: legend,
          top: -10000,
          left: -10000
        },
        brush: allowBrush
          ? {
              xAxisIndex: 0
            }
          : undefined,
        toolbox: {
          show: false
        },
        yAxis: primaryYAxis,
        animation: false,
        series: primarySeries as any,
        tooltip: {
          trigger: chartType === 'SCATTER' ? 'item' : 'axis',
          confine: true,
          textStyle: {
            fontSize: 14,
            fontFamily: sansFontFamily
          },
          extraCssText:
            'max-width: 75%; max-height: 50vh; overflow-y: auto; padding: 10px 0; background-color: rgba(var(--text-background)); border-color: rgba(var(--border-color));',
          formatter: tooltipFormatter
        },
        visualMap:
          chartType === 'SCATTER' &&
          config?.scatterConfig?.color &&
          (sentioColors.dark as any)[config.scatterConfig.color]
            ? {
                dimension: 2,
                min: minRef.current,
                max: maxRef.current,
                show: false,
                inRange: {
                  color: isDarkMode
                    ? (sentioColors.dark as any)[config.scatterConfig.color]
                    : (sentioColors.light as any)[config.scatterConfig.color]
                }
              }
            : undefined
      }

      const finalOptions = applyOverlaySeries(baseOptions)
      if (isDarkMode && Array.isArray(finalOptions.series)) {
        finalOptions.series = applyDarkAreaOpacity(
          finalOptions.series as any[]
        ) as any
      }
      return finalOptions
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [
      title,
      xAxisData,
      legend,
      allowBrush,
      yAxis,
      series,
      compareSeries,
      applyOverlaySeries,
      tooltipFormatter,
      maximumSignificantDigits,
      xAxis,
      chartType,
      isDarkMode
    ])

    const {
      value: isOpen,
      setFalse: setClose,
      setTrue: setOpen
    } = useBoolean(false)
    const clickEventParamsRef = useRef<any>(null)
    const [selectedSeriesIndex, setSelectedSeriesIndex] = useState(0)
    const chartPositionRef = useRef<HTMLDivElement>(null)

    const { x, y, strategy, refs } = useFloating({
      open: isOpen,
      placement: 'right',
      middleware: [flip(), shift()]
    })
    const onClick = useCallback(
      (params: any, rawParams?: any) => {
        if (!params) {
          setClose()
          return
        }
        const baseBoundingRect =
          chartPositionRef.current?.getBoundingClientRect()
        if (params.event) {
          const { event } = params
          clickEventParamsRef.current = params
          const targetBoundRect = event.target.getBoundingRect()
          refs.setPositionReference({
            getBoundingClientRect: () => {
              return {
                x: event.offsetX + (baseBoundingRect?.x ?? 0),
                y: event.offsetY + (baseBoundingRect?.y ?? 0),
                width: targetBoundRect.width,
                height: targetBoundRect.height,
                top: event.offsetY + (baseBoundingRect?.top ?? 0),
                right: event.offsetX + (baseBoundingRect?.right ?? 0),
                bottom: event.offsetY + (baseBoundingRect?.bottom ?? 0),
                left: event.offsetX + (baseBoundingRect?.left ?? 0)
              } as DOMRect
            }
          })
        } else if (isArray(params)) {
          const targetValue = closestNumber(
            yAxisRef.current?.map((item: any) => item.value[1]),
            params[1]
          )
          clickEventParamsRef.current = yAxisRef.current?.find(
            (item: any) => item.value[1] === targetValue
          )
          refs.setPositionReference({
            getBoundingClientRect: () => {
              return {
                x: rawParams.offsetX + (baseBoundingRect?.x ?? 0),
                y: rawParams.offsetY + (baseBoundingRect?.y ?? 0),
                width: 10,
                height: 10,
                top: rawParams.offsetY + (baseBoundingRect?.top ?? 0),
                right: rawParams.offsetX + (baseBoundingRect?.right ?? 0),
                bottom: rawParams.offsetY + (baseBoundingRect?.bottom ?? 0),
                left: rawParams.offsetX + (baseBoundingRect?.left ?? 0)
              } as DOMRect
            }
          })
        }
        setOpen()
      },
      [refs, setClose, setOpen]
    )

    const onSeriesEvent = useCallback((event: string, params: any) => {
      switch (event) {
        case 'click':
          setSelectedSeriesIndex(params.seriesIndex)
          break
        case 'mouseover':
          ;(global as any).highlightSeriesId = params.seriesId
          document
            .querySelectorAll(`.series_${params.seriesId}`)
            .forEach((node) => {
              node.classList.add('highlighted')
            })
          break
        case 'mouseout':
          const previousSeriesId = (global as any).highlightSeriesId
          if (previousSeriesId) {
            document
              .querySelectorAll(`.series_${previousSeriesId}`)
              .forEach((node) => {
                node.classList.remove('highlighted')
              })
          }
          ;(global as any).highlightSeriesId = ''
      }
    }, [])

    const onSelectSeries = useCallback(
      (seriesIndex: number) => {
        clickEventParamsRef.current = {
          seriesIndex: seriesIndex,
          seriesName: series[seriesIndex].name
        }
        if (yAxisRef.current[seriesIndex]?.value) {
          clickEventParamsRef.current.value =
            yAxisRef.current[seriesIndex].value
        }
        setSelectedSeriesIndex(seriesIndex)
      },
      [series, setSelectedSeriesIndex]
    )

    const switchSeries = useMemo(() => {
      const items = [
        series.map((serie, index) => ({
          label: serie.name || '',
          key: `${index}`
        }))
      ]
      return (
        <PopupMenuButton
          onSelect={(d) => onSelectSeries(parseInt(d, 10))}
          items={items}
          buttonIcon={(open: boolean) => (
            <span
              className={classNames(
                'text-primary-100',
                'hover:bg-primary-200/80 inline-block cursor-pointer rounded-md px-1 py-0.5',
                open ? 'bg-primary-200/80' : ''
              )}
            >
              <LuChevronDown className="h-3.5 w-3.5" />
            </span>
          )}
          portal={false}
          placement="bottom-end"
          itemsClassName="max-h-[50vh] overflow-y-auto"
        />
      )
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [series])

    // Note: `switchSeries` and `selectedSeriesIndex` participate in the fixed
    // (click) tooltip series-switching affordance; referenced to keep parity with
    // the source even though they are wired via tooltipParmsRef.
    void switchSeries
    void selectedSeriesIndex
    // `getEventName`, `sourceType`, and `getEventNameById` are now genuinely used
    // by getSeriesContext.

    const noLegend = useMemo(() => {
      // hide legend for single series scatter chart
      if (chartType === 'SCATTER' && series.length === 1) {
        return true
      }
      return _noLegend
    }, [_noLegend, chartType, series?.length])

    console.log('options', options)

    return (
      <div
        className="h-full w-full"
        onMouseOver={() => {
          setActive(true)
        }}
        onMouseOut={() => {
          setActive(false)
        }}
      >
        <ReactEChartsBase
          ref={ref}
          loading={loading}
          group={group}
          option={options}
          minHeight={minHeight}
          returnedSeries={returnedSeries}
          totalSeries={totalSeries}
          onSelect={onSelect}
          onClick={allowClick ? onClick : undefined}
          style={style}
          onSeriesEvent={onSeriesEvent}
          noLegend={noLegend}
          onInitChart={onInitChart}
        />
        {controls && nonXAxis && (
          <XAxisControls xAxis={xAxis} setXAxis={setXAxis} defaultOpen />
        )}
        {controls && <YaxisControls yAxis={yAxis} setYAxis={setYAxisWrap} />}
        <div className="absolute left-0 top-0" ref={chartPositionRef}></div>
        {isOpen && allowClick && (
          <FloatingPortal>
            <FloatingOverlay className="z-50 bg-gray-100/20" onClick={setClose}>
              <div
                ref={refs.setFloating}
                className="border-border-color bg-default-bg min-w-64 absolute w-fit divide-y rounded-md border pb-2 shadow-sm"
                style={{
                  position: strategy,
                  top: 0,
                  left: 0,
                  transform: `translate(${roundByDPR(x || 0)}px,${roundByDPR(y || 0)}px)`
                }}
                onClick={onClickPreventDefault}
              >
                <div className="text-text-foreground max-h-80 max-w-[50vw] overflow-y-auto py-2 text-sm">
                  {chartType === 'SCATTER' ? (
                    <ScatterChartTooltip {...tooltipParmsRef.current} isFixed />
                  ) : (
                    <ChartTooltip {...tooltipParmsRef.current} isFixed />
                  )}
                </div>
              </div>
            </FloatingOverlay>
          </FloatingPortal>
        )}
      </div>
    )
  }
)

TimeSeriesChart.displayName = 'TimeSeriesChart'

export { TimeSeriesChart }
export default TimeSeriesChart

function computeMarkLines(series: any[], markLines: MarkLine[]) {
  if (markLines && markLines.length > 0 && series.length > 0) {
    series[0] = {
      ...series[0],
      markLine: {
        symbol: [],
        data: markLines.map((area) => {
          if (area.from || area.to) {
            return [
              {
                name: 'threshold',
                xAxis: area.from,
                yAxis: area.value ?? 'min',
                symbol: 'react',
                symbolSize: [20, 1],
                symbolOffset: [area.below ? -10 : 10, 0],
                label: {
                  formatter: `${area.label}`,
                  position: area.below
                    ? 'insideMiddleBottom'
                    : 'insideMiddleTop',
                  color: area.color || '#ff0000'
                },
                lineStyle: {
                  color: area.color || '#ff0000'
                }
              },
              {
                symbol: 'rect',
                symbolSize: [20, 1],
                symbolOffset: [area.below ? 10 : -10, 0],
                xAxis: area.to ? area.to : undefined,
                yAxis: area.value ?? 'max'
              }
            ]
          } else {
            return {
              name: 'threshold',
              yAxis: area.value,
              symbol: 'react',
              symbolSize: [20, 1],
              symbolOffset: [area.below ? -10 : 10, 0],
              label: {
                formatter: `${area.label}`,
                position: area.below ? 'insideStartBottom' : 'insideStartTop',
                color: area.color || '#ff0000'
              },
              lineStyle: {
                color: area.color || '#ff0000'
              }
            }
          }
        })
      }
    }
  }
}

function computeMarksAreas(series: any[], markAreas: MarkArea[]) {
  if (markAreas && markAreas.length > 0 && series.length > 0) {
    series[0].markArea = {
      itemStyle: {
        color: markAreas[0].color || 'rgba(255, 173, 177, 0.4)'
      }
    }
    series[0].markArea.data = markAreas.map((markArea) => {
      return [
        {
          xAxis: markArea.from ? markArea.from : undefined
        },
        {
          xAxis: markArea.to ? markArea.to : undefined
        }
      ]
    })
  }
}

const MIN_SIZE = 5
const MAX_SIZE = 30
const LEVELS = 10

function normalize(
  value: number,
  min: number,
  max: number,
  minSize = MIN_SIZE,
  maxSize = MAX_SIZE
): number {
  if (max === min) return MIN_SIZE
  if (value <= min) return minSize
  if (value >= max) return maxSize

  const range = max - min
  const levelRange = range / LEVELS

  const sizeIncrement = (maxSize - minSize) / (LEVELS - 1)

  const level = Math.floor((value - min) / levelRange)
  const clampedLevel = Math.min(level, LEVELS - 1)

  return minSize + clampedLevel * sizeIncrement
}
