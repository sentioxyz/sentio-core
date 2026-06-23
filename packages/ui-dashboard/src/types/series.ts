// Computed-series shapes consumed by the render charts. The app runs the
// worker compute (lib/metrics/series) and passes the result in; ui-dashboard
// only renders, so it mirrors the structural shape rather than importing it.

/** A single data point: [timestamp, value, ...extra values]. */
export type SeriesDataLike<T = Date> = [T, number | null, ...number[]]

/** One rendered series (echarts-ready). */
export interface SeriesLike<T = Date> {
  name: string
  data: SeriesDataLike<T>[]
  showSymbol: boolean
  type: 'line' | 'bar'
  areaStyle?: any
  lineStyle?: any
  stack?: string
  stackStrategy?: string
  emphasis?: any
  id: string
  xAxisIndex?: number
  symbolSize?: number | ((v: SeriesDataLike<T>) => number)
  itemStyle?: { color?: string }
  smooth?: boolean
}
