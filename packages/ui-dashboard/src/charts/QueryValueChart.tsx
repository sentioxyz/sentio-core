import { CSSProperties, forwardRef, useMemo } from 'react'
import type { ECharts } from 'echarts/core'
import { useResizeDetector } from 'react-resize-detector'
import { EChartsHandle, EChartsOption, ReactEChartsBase } from './EchartsBase'

export interface QueryValueChartProps {
  /** Worker-resolved render inputs (app runs the QueryValue worker compute). */
  series: any[]
  valueText?: string
  textColor?: string
  backgroundColor?: string
  minHeight?: number
  loading?: boolean
  style?: CSSProperties
  onInitChart?: (chart: ECharts) => void
}

export const QueryValueChart = forwardRef<EChartsHandle, QueryValueChartProps>(
  (props: QueryValueChartProps, ref) => {
    const {
      series,
      valueText,
      textColor,
      backgroundColor,
      minHeight,
      loading,
      style,
      onInitChart
    } = props
    const { width, height, ref: ref2 } = useResizeDetector()

    const fontSize = useMemo(() => {
      return Math.min(
        (width || 0) / String(valueText).length,
        (height || 0) / 1.5
      )
    }, [width, height, valueText])

    const options: EChartsOption = {
      backgroundColor,
      grid: { top: 0, right: 0, bottom: 0, left: 0 },
      toolbox: { show: false },
      animation: false,
      series: series as any,
      xAxis: { type: 'time', show: false },
      yAxis: { type: 'value', show: false },
      legend: { show: false },
      graphic: {
        type: 'text',
        z: 100,
        left: 'center',
        top: 'middle',
        style: {
          text: valueText,
          fontSize,
          stroke: textColor,
          fill: textColor
        }
      }
    }

    return (
      <div className="h-full w-full">
        <div className="h-full w-full" ref={ref2}>
          <ReactEChartsBase
            ref={ref}
            loading={loading}
            option={options}
            minHeight={minHeight}
            style={style}
            noLegend
            onInitChart={onInitChart}
          />
        </div>
      </div>
    )
  }
)

QueryValueChart.displayName = 'QueryValueChart'
