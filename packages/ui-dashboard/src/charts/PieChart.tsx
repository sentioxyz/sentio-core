import { CSSProperties, forwardRef, useEffect, useState } from 'react'
import type { ECharts } from 'echarts/core'
import { useResizeDetector } from 'react-resize-detector'
import { EChartsHandle, EChartsOption, ReactEChartsBase } from './EchartsBase'
import { useDarkMode } from '../utils/use-dark-mode'
import { isMobile as detectMobile } from '../utils/is-mobile'
import type { ChartConfigLike } from '../types'

const theresholdWidth = 480

// Min gap kept between the tooltip and each window edge.
const TOOLTIP_MIN_VIEWPORT_LEFT = 8
const TOOLTIP_MIN_VIEWPORT_RIGHT = 8

/** Minimal shape consumed from a computed series — only name + first data point. */
export interface PieSeriesInput {
  name: string
  data: any[]
}

export interface PieChartProps {
  /** Already-computed series (app runs the worker compute and passes the result). */
  series: PieSeriesInput[]
  /** Formats a slice value for tooltip/legend display. */
  valueFormatter: (value: number) => string
  config?: ChartConfigLike
  title?: string
  minHeight?: number
  loading?: boolean
  style?: CSSProperties
  onInitChart?: (chart: ECharts) => void
}

export const PieChart = forwardRef<EChartsHandle, PieChartProps>(
  (props: PieChartProps, ref) => {
    const {
      series,
      valueFormatter,
      config,
      title,
      minHeight,
      loading,
      style,
      onInitChart
    } = props
    const [options, setOptions] = useState<EChartsOption>({})
    const isDarkMode = useDarkMode()
    const isMobile = detectMobile()
    const { width, ref: resizeRef } = useResizeDetector({
      refreshMode: 'debounce',
      refreshRate: 500,
      handleHeight: false
    })

    // Place tooltip near cursor while keeping it inside the viewport
    // (with extra reserved padding on the left for the app sidebar).
    // Coordinates returned are in chart-container space — ECharts translates
    // them to document coords when `appendToBody` is set.
    const tooltipPosition = (
      point: [number, number],
      _params: unknown,
      _dom: HTMLElement | unknown,
      _rect: unknown,
      size: { contentSize: [number, number]; viewSize: [number, number] }
    ): [number, number] => {
      const chartRect = (
        resizeRef as React.RefObject<HTMLDivElement>
      ).current?.getBoundingClientRect()
      const chartLeft = chartRect?.left ?? 0
      const winW =
        typeof window !== 'undefined' ? window.innerWidth : size.viewSize[0]
      const [w, h] = size.contentSize

      const minXInChart = TOOLTIP_MIN_VIEWPORT_LEFT - chartLeft
      const maxXInChart = winW - TOOLTIP_MIN_VIEWPORT_RIGHT - chartLeft - w

      let x = point[0] + 12
      if (x > maxXInChart) {
        x = point[0] - w - 12
      }
      if (x < minXInChart) x = minXInChart
      if (x > maxXInChart) x = maxXInChart

      const y = Math.max(0, point[1] - h / 2)
      return [x, y]
    }

    useEffect(() => {
      const isHLegend = width && width < theresholdWidth

      // Tooltip wrapping/clamping — mobile only.
      // ECharts tooltips default to `white-space: nowrap`, so a single long
      // label (e.g. a full token name) produces a wide tooltip. With
      // `appendToBody: true` that tooltip lives on <body>; near the right edge
      // of the viewport it can push past `innerWidth` and trigger horizontal
      // scroll or unwanted zoom on mobile. Cap the width and force wrapping
      // ONLY on mobile — desktop keeps the default behavior.
      const winW = typeof window !== 'undefined' ? window.innerWidth : 1024
      const tooltipMaxWidth = Math.max(160, Math.min(280, winW - 48))
      const tooltipExtraCss = isMobile
        ? `max-width: ${tooltipMaxWidth}px; white-space: normal; word-break: break-word; overflow-wrap: anywhere;`
        : ''

      const d = [] as any[]
      series.forEach((s) => {
        if (s.data.length > 0 && s.data[0] && s.data[0][1] != null) {
          const rawValue = s.data[0][1]
          if (config?.pieConfig?.absValue) {
            d.push({ name: s.name, value: Math.abs(rawValue) })
          } else if (rawValue > 0) {
            d.push({ name: s.name, value: rawValue })
          }
        }
      })

      const total = d.reduce((acc, cur) => acc + cur.value, 0)
      d.sort((a, b) => {
        const percentA = (a.value / total) * 100
        const percentB = (b.value / total) * 100
        return percentB - percentA
      })

      const pieSeries = [
        {
          type: 'pie',
          radius: [config?.pieConfig?.pieType == 'Donut' ? '40%' : 0, '70%'],
          center: isHLegend ? ['50%', '50%'] : ['35%', '50%'],
          label: { show: false },
          labelLine: { length: 10, length2: 10, maxSurfaceAngle: 50 },
          data: d
        }
      ]
      const options: EChartsOption = {
        title: { text: title, left: 8 },
        legend: isHLegend
          ? {
              type: 'scroll',
              orient: 'horizontal',
              bottom: 12,
              left: 'center',
              animation: true,
              animationDurationUpdate: 300,
              pageIconSize: [10, 8],
              pageButtonItemGap: 2,
              pageButtonGap: 4,
              textStyle: {
                width: width ? width * 0.4 : 100,
                overflow: 'truncate'
              },
              tooltip: {
                show: true,
                appendToBody: true,
                extraCssText: tooltipExtraCss,
                position: tooltipPosition,
                formatter: function (params: any) {
                  const name = params.name
                  const item = d.find((i) => i.name === name)
                  let ret = name
                  if (config?.pieConfig?.showValue && item) {
                    ret += '<br/>' + valueFormatter(item.value)
                  }
                  if (config?.pieConfig?.showPercent && item) {
                    const percent = (
                      (item.value /
                        d.reduce((acc, cur) => acc + cur.value, 0)) *
                      100
                    ).toFixed(2)
                    ret += config.pieConfig.showValue
                      ? ` • ${percent}%`
                      : `\n${percent}%`
                  }
                  return ret
                }
              }
            }
          : {
              type: 'scroll',
              orient: 'vertical',
              right: 16,
              top: title ? 48 : 8,
              width: '35%',
              animation: true,
              animationDurationUpdate: 300,
              tooltip: {
                show: true,
                appendToBody: true,
                extraCssText: tooltipExtraCss,
                position: tooltipPosition,
                formatter: function (params: any) {
                  const name = params.name
                  const item = d.find((i) => i.name === name)
                  let ret = name
                  if (config?.pieConfig?.showValue && item) {
                    ret += '<br/>' + valueFormatter(item.value)
                  }
                  if (config?.pieConfig?.showPercent && item) {
                    const percent = (
                      (item.value /
                        d.reduce((acc, cur) => acc + cur.value, 0)) *
                      100
                    ).toFixed(2)
                    ret += config.pieConfig.showValue
                      ? ` • ${percent}%`
                      : `\n${percent}%`
                  }
                  return ret
                }
              },
              icon: 'roundRect',
              itemWidth: 12,
              itemHeight: 12,
              itemGap: 6,
              show: true,
              pageIconSize: [8, 10],
              pageButtonGap: 4,
              pageButtonItemGap: 2,
              pageIconColor: isDarkMode ? '#909399' : '#4E5969',
              pageIconInactiveColor: isDarkMode ? '#606266' : '#C9CDD4',
              textStyle: {
                width: width ? width * 0.3 : 'auto',
                overflow: 'truncate',
                lineHeight: 16,
                fontSize: 12,
                rich: { value: { padding: [4, 0, 0, 0] } }
              },
              formatter: (name: string) => {
                const item = d.find((i) => i.name === name)
                let ret = name
                if (config?.pieConfig?.showValue && item) {
                  ret += '\n' + valueFormatter(item.value)
                }
                if (config?.pieConfig?.showPercent && item) {
                  const percent = (
                    (item.value / d.reduce((acc, cur) => acc + cur.value, 0)) *
                    100
                  ).toFixed(2)
                  ret += config.pieConfig.showValue
                    ? ` • ${percent}%`
                    : `\n${percent}%`
                }
                return ret
              }
            },
        tooltip: {
          trigger: 'item',
          appendToBody: true,
          extraCssText: tooltipExtraCss,
          position: tooltipPosition,
          formatter: ({ name, data, percent }: any) => {
            let ret = `${name}`
            if (config?.pieConfig?.showValue) {
              ret += '<br/>' + valueFormatter(data.value)
            }
            if (config?.pieConfig?.showPercent) {
              ret += config.pieConfig.showValue
                ? ` (${percent}%)`
                : `\n${percent}%`
            }
            return ret
          }
        },
        toolbox: { show: false },
        animation: false,
        series: pieSeries as any
      }
      setOptions(options)
    }, [series, config, valueFormatter, isDarkMode, isMobile, width, title])

    return (
      <div className="h-full w-full" ref={resizeRef}>
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

PieChart.displayName = 'PieChart'
