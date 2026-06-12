import type {
  ChartTypeLike,
  DataSourceTypeLike,
  NoteFontSizeLike,
  NoteAlignmentLike,
  NoteVerticalAlignmentLike
} from './enums'
// Type-only import; the chart <-> dashboard type cycle is erased at compile time.
import type { GroupLike } from './dashboard'

/*
 * Chart-level shapes (chart / note / chart config / overlay graph).
 *
 * NOTE: ChartConfigLike is intentionally a MINIMAL subset for now. The full set
 * of fields (xAxis, tableConfig, pieConfig, valueConfig, mapping rules, color
 * themes, etc.) is fleshed out alongside the chart-rendering components, adding
 * only the fields those components actually read.
 */

/** Y-axis configuration. */
export interface YAxisConfigLike {
  min?: string
  max?: string
  scale?: boolean
  stacked?: string
  column?: string
  name?: string
}

/** Minimal subset of a chart's config. Extended alongside chart components. */
export interface ChartConfigLike {
  yAxis?: YAxisConfigLike
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
