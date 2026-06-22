import { CSSProperties, forwardRef, useEffect, useMemo, useState } from 'react'
import type { ECharts } from 'echarts/core'
import type { YAXisComponentOption } from 'echarts'
import { EChartsHandle, EChartsOption, ReactEChartsBase } from './EchartsBase'
import type { ChartConfigLike } from '../types'
import type { PieSeriesInput } from './PieChart'

const compareOption: Intl.CollatorOptions = { numeric: true }

export interface BarGaugeChartProps {
  /** Already-computed series (app runs the worker compute and passes the result). */
  series: PieSeriesInput[]
  legend?: string[]
  /** Formats a bar value for the label / value-axis. */
  valueFormatter: (value: number) => string
  config?: ChartConfigLike
  title?: string
  minHeight?: number
  loading?: boolean
  style?: CSSProperties
  onInitChart?: (chart: ECharts) => void
}

export const BarGaugeChart = forwardRef<EChartsHandle, BarGaugeChartProps>(
  (props: BarGaugeChartProps, ref) => {
    const {
      series: input,
      legend,
      valueFormatter,
      config,
      title,
      minHeight,
      loading,
      style,
      onInitChart
    } = props
    const [series, setSeries] = useState<any[]>([])
    const [xAxis, setXAxis] = useState<any>()
    const [yAxis, setYAxis] = useState<any>()
    const isVertical = config?.barGauge?.direction === 'VERTICAL'

    useEffect(() => {
      const tmpData = input.map((s) => {
        const d = s.data && s.data[0]
        return { name: s.name, value: d && d[1]! }
      })
      const sort = config?.barGauge?.sort
      switch (sort?.sortBy) {
        case 'ByName':
          tmpData.sort((a, b) =>
            sort.orderDesc
              ? b.name!.localeCompare(a.name!, undefined, compareOption)
              : a.name!.localeCompare(b.name!, undefined, compareOption)
          )
          break
        case 'ByValue':
          tmpData.sort((a, b) =>
            sort.orderDesc ? b.value - a.value : a.value - b.value
          )
          break
      }

      const series: { type: 'bar'; data: number[]; label: any }[] = [
        {
          type: 'bar',
          data: tmpData.map((d) => d.value),
          label: {
            show: config?.valueConfig
              ? config.valueConfig.showValueLabel
              : false,
            position:
              config?.barGauge?.direction == 'VERTICAL' ? 'top' : 'right',
            formatter: ({ value }: any) => valueFormatter(value)
          }
        }
      ]

      const seriesAxis = {
        type: 'category',
        data: tmpData.map((s) => s.name),
        axisLabel:
          config?.barGauge?.direction == 'VERTICAL'
            ? { interval: 0, rotate: 30 }
            : {}
      } as YAXisComponentOption

      if (config?.xAxis?.name) {
        seriesAxis.name = config?.xAxis?.name
        seriesAxis.nameLocation = 'middle'
        seriesAxis.nameGap = isVertical ? 45 : 60
      }

      const valueAxis = {
        type: 'value',
        axisLabel:
          // show dates on value-axis label is weird
          config?.valueConfig?.valueFormatter == 'DateFormatter'
            ? undefined
            : { formatter: (v: number) => valueFormatter(v) }
      }

      let xAxis, yAxis
      switch (config?.barGauge?.direction) {
        case 'VERTICAL':
          xAxis = seriesAxis
          yAxis = valueAxis
          break
        case 'HORIZONTAL':
        default:
          xAxis = valueAxis
          yAxis = seriesAxis
      }
      setSeries(series)
      setXAxis(xAxis)
      setYAxis(yAxis)
    }, [
      input,
      config?.barGauge?.calculation,
      config?.barGauge?.sort,
      config?.valueConfig?.showValueLabel,
      config?.xAxis?.name,
      isVertical,
      valueFormatter
    ])

    const dataZoom = useMemo(() => {
      if (config?.barGauge?.direction == 'HORIZONTAL') {
        return [
          {
            show: series[0]?.data.length > 15,
            type: 'slider',
            yAxisIndex: 0,
            zoomLock: true,
            width: 8,
            right: 10,
            top: 5,
            bottom: 30,
            minValueSpan: 5,
            maxValueSpan: 15,
            orient: 'vertical',
            handleSize: 0,
            showDetail: false,
            brushSelect: false,
            showDataShadow: false
          },
          {
            type: 'inside',
            id: 'insideY',
            yAxisIndex: 0,
            zoomOnMouseWheel: false,
            moveOnMouseMove: true,
            moveOnMouseWheel: true
          }
        ]
      } else {
        return [
          {
            show: series[0]?.data.length > 25,
            type: 'slider',
            xAxisIndex: 0,
            zoomLock: true,
            height: 8,
            bottom: 5,
            maxValueSpan: 25,
            minValueSpan: 10,
            handleSize: '0',
            showDetail: false,
            orient: 'horizontal',
            brushSelect: false,
            showDataShadow: false
          },
          {
            type: 'inside',
            id: 'insideX',
            xAxisIndex: 0,
            zoomOnMouseWheel: false,
            moveOnMouseMove: true,
            moveOnMouseWheel: true
          }
        ]
      }
    }, [config, series])

    const options: EChartsOption = {
      title: { text: title },
      grid: {
        top: title ? 48 : 16,
        right: 40,
        bottom: isVertical && config?.xAxis?.name ? 40 : 16,
        left: !isVertical && config?.xAxis?.name ? 40 : 16,
        containLabel: true
      },
      xAxis,
      legend: { data: legend, top: -10000, left: -10000 },
      toolbox: { show: false },
      yAxis,
      dataZoom,
      animation: false,
      series,
      tooltip: {
        trigger: 'axis',
        confine: true,
        extraCssText: 'max-width: 50%; max-height: 50vh; overflow-y: auto;'
      }
    }

    return (
      <div className="h-full w-full">
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
    )
  }
)

BarGaugeChart.displayName = 'BarGaugeChart'
