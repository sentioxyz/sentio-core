import type { DurationLike } from '@sentio/ui-core'
import type {
  ChartTypeLike,
  DataSourceTypeLike,
  NoteFontSizeLike,
  NoteAlignmentLike,
  NoteVerticalAlignmentLike,
  CalculationLike,
  DirectionLike,
  MarkerTypeLike,
  SortByLike,
  ValueFormatterLike,
  ValueStyleLike,
  PieTypeLike,
  LineStyleLike,
  ColumnTypeLike
} from './enums'
// Type-only import; the chart <-> dashboard type cycle is erased at compile time.
import type { GroupLike } from './dashboard'

/*
 * Chart-level shapes (chart / note / chart config / overlay graph).
 *
 * ChartConfigLike and its sub-interfaces are structural mirrors of the proto
 * `ChartConfig` message tree (all fields optional) — they back the chart render
 * components and the per-chart option panels. Deep payloads that the option
 * panels don't edit (e.g. an overridden TimeRange) are kept opaque (`unknown`).
 */

/** Sort key + direction (x-axis, bar-gauge). */
export interface SortLike {
  sortBy?: SortByLike
  orderDesc?: boolean
}

/** Y-axis configuration. */
export interface YAxisConfigLike {
  min?: string
  max?: string
  scale?: boolean
  stacked?: string
  column?: string
  name?: string
}

/** X-axis configuration. */
export interface XAxisConfigLike {
  type?: string
  min?: string
  max?: string
  scale?: boolean
  name?: string
  column?: string
  sort?: SortLike
  format?: string
}

/** Text/background color overrides for value + mapping rules. */
export interface ColorThemeLike {
  textColor?: string
  backgroundColor?: string
  themeType?: string
}

/** A value→text/color mapping rule. */
export interface MappingRuleLike {
  comparison?: string
  value?: number | 'NaN' | 'Infinity' | '-Infinity'
  text?: string
  colorTheme?: ColorThemeLike
}

/** Number/string/date value formatting. */
export interface ValueConfigLike {
  valueFormatter?: ValueFormatterLike
  showValueLabel?: boolean
  maxSignificantDigits?: number
  dateFormat?: string
  mappingRules?: MappingRuleLike[]
  style?: ValueStyleLike
  maxFractionDigits?: number
  precision?: number
  currencySymbol?: string
  tooltipTotal?: boolean
  prefix?: string
  suffix?: string
}

/** Line chart style. */
export interface LineConfigLike {
  style?: LineStyleLike
  smooth?: boolean
}

/** Pie/donut config. */
export interface PieConfigLike {
  pieType?: PieTypeLike
  showPercent?: boolean
  showValue?: boolean
  calculation?: CalculationLike
  absValue?: boolean
}

/** Bar-gauge config. */
export interface BarGaugeConfigLike {
  direction?: DirectionLike
  calculation?: CalculationLike
  sort?: SortLike
}

/** Query-value (single big number) config. */
export interface QueryValueConfigLike {
  colorTheme?: ColorThemeLike
  showBackgroundChart?: boolean
  calculation?: CalculationLike
  seriesCalculation?: CalculationLike
}

/** Per-column label config (event/log tables). */
export interface LabelConfigColumnLike {
  name?: string
  showLabel?: boolean
  showValue?: boolean
}

/** Series label config. */
export interface LabelConfigLike {
  columns?: LabelConfigColumnLike[]
  alias?: string
}

/** Scatter chart config. */
export interface ScatterConfigLike {
  symbolSize?: string
  color?: string
  minSize?: number
  maxSize?: number
}

/** A mark line/area on a chart. */
export interface MarkerLike {
  type?: MarkerTypeLike
  value?: number | 'NaN' | 'Infinity' | '-Infinity'
  color?: string
  label?: string
  valueX?: string
}

/** Series count limit. */
export interface DataConfigLike {
  seriesLimit?: number
}

/** Per-series override (currently just chart type). */
export interface SeriesConfigSeriesLike {
  type?: ChartTypeLike
}

/** Per-series config keyed by series id. */
export interface SeriesConfigLike {
  series?: { [key: string]: SeriesConfigSeriesLike }
}

/** Table column sort entry. */
export interface ColumnSortLike {
  column?: string
  orderDesc?: boolean
}

/** Table chart config. */
export interface TableConfigLike {
  calculation?: CalculationLike
  showColumns?: { [key: string]: boolean }
  sortColumns?: ColumnSortLike[]
  columnOrders?: string[]
  columnWidths?: { [key: string]: number }
  showPlainData?: boolean
  calculations?: { [key: string]: CalculationLike }
  valueConfigs?: { [key: string]: ValueConfigLike }
  rowLimit?: number
}

/**
 * Minimal shape of the query result TableControls inspects to enumerate
 * columns — either a SQL result (column types) or metrics results (series
 * with labels). Mirrors the fields read from the proto MetricsQueryResponse /
 * SyncExecuteSQLResponse, so a consumer can pass either response directly.
 */
export interface TableDataLike {
  /** SQL path: column name → column type. */
  result?: { columnTypes?: { [name: string]: ColumnTypeLike } }
  /** Metrics path: one entry per query, each with its matrix samples. */
  results?: Array<{
    alias?: string
    matrix?: { samples?: Array<{ metric?: { labels?: { [k: string]: string }; displayName?: string } }> }
  }>
}

/** Compare-to-previous-period offset. */
export interface CompareTimeLike {
  ago?: DurationLike
}

/** Per-chart time-range override. `timeRange` payload kept opaque. */
export interface TimeRangeOverrideLike {
  enabled?: boolean
  timeRange?: unknown
  compareTime?: CompareTimeLike
}

/** A chart's full config — structural mirror of proto `ChartConfig`. */
export interface ChartConfigLike {
  yAxis?: YAxisConfigLike
  xAxis?: XAxisConfigLike
  lineConfig?: LineConfigLike
  valueConfig?: ValueConfigLike
  pieConfig?: PieConfigLike
  barGauge?: BarGaugeConfigLike
  queryValueConfig?: QueryValueConfigLike
  tableConfig?: TableConfigLike
  labelConfig?: LabelConfigLike
  scatterConfig?: ScatterConfigLike
  seriesConfig?: SeriesConfigLike
  dataConfig?: DataConfigLike
  timeRangeOverride?: TimeRangeOverrideLike
  markers?: MarkerLike[]
}

/** A note (text) panel's content and styling. */
export interface NoteLike {
  content?: string
  fontSize?: NoteFontSizeLike
  textAlign?: NoteAlignmentLike
  verticalAlign?: NoteVerticalAlignmentLike
  backgroundColor?: string
  textColor?: string
}

/**
 * An overlay graph drawn on top of a chart. Query payloads
 * (insightsQueries/formulas/sql*) are left opaque (`unknown`) for now — pure
 * render components treat them as opaque; data fetching lives in the consumer.
 */
export interface OverlayGraphLike {
  name?: string
  chartType?: ChartTypeLike
  yAxis?: YAxisConfigLike
  sqlQuery?: string
  sqlQueryId?: string
  insightsQueries?: unknown[]
  formulas?: unknown[]
}

/**
 * A chart. Datasource-specific query payloads are kept opaque (`unknown`) at
 * this stage; they are consumed by the application's data layer, not by the
 * pure render components.
 */
export interface ChartLike {
  type?: ChartTypeLike
  datasourceType?: DataSourceTypeLike
  config?: ChartConfigLike
  note?: NoteLike
  overlayGraphs?: OverlayGraphLike[]
  enableExperimentalFeatures?: boolean
  sqlQuery?: string
  sqlQueryId?: string
  /** Set when type === 'GROUP'. */
  group?: GroupLike
  queries?: unknown[]
  formulas?: unknown[]
  insightsQueries?: unknown[]
  segmentationQueries?: unknown[]
  retentionQuery?: unknown
  eventLogsConfig?: unknown
}
