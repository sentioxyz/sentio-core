// Chart foundation (phase 3a): ECharts base wrapper, legend, refresh context,
// theme, and the chart-type icons. Render charts and option panels are added in
// later phases.

// ECharts base + legend
export {
  ReactEChartsBase,
  type EChartsOption,
  type ReactEChartsProps,
  type EChartsHandle
} from './EchartsBase'
export { ChartLegend } from './ChartLegend'

// Refresh affordance
export { RefreshContext, RefreshButton } from './RefreshContext'

// Chart-type selector
export { ChartTypeButtonGroup } from './ChartTypeButtonGroup'

// Tooltips
export { ChartTooltip } from './ChartTooltip'
export { ScatterChartTooltip } from './ScatterChartTooltip'

// Render charts (presentational — app injects computed series + formatter)
export { PieChart, type PieChartProps, type PieSeriesInput } from './PieChart'

// Option panels
export * from './options'

// Theme
export { sentioColors } from './theme/sentio-colors'
export { sentioTheme, sentioThemeDark } from './theme/sentio-theme'

// Chart-type icons
export { default as LineIcon } from './icons/LineIcon'
export { default as AreaIcon } from './icons/AreaIcon'
export { default as BarIcon } from './icons/BarIcon'
export { default as BarGuageIcon } from './icons/BarGuageIcon'
export { default as PieIcon } from './icons/PieIcon'
export { default as QueryValueIcon } from './icons/QueryValueIcon'
export { default as ScatterIcon } from './icons/ScatterIcon'
export { default as TableIcon } from './icons/TableIcon'
