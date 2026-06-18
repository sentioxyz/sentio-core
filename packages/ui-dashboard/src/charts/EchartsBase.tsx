import React, {
  useEffect,
  CSSProperties,
  useCallback,
  useState,
  useImperativeHandle,
  forwardRef,
  useRef,
  useMemo
} from 'react'
import { CanvasRenderer, SVGRenderer } from 'echarts/renderers'
import { init, use } from 'echarts/core'
import {
  LineChart,
  BarChart,
  PieChart,
  ScatterChart,
  SankeyChart
} from 'echarts/charts'
import {
  LegendComponent,
  GridComponent,
  TooltipComponent,
  ToolboxComponent,
  TitleComponent,
  DataZoomComponent,
  BrushComponent,
  MarkLineComponent,
  MarkAreaComponent,
  GraphicComponent,
  VisualMapComponent
} from 'echarts/components'
import type { ECharts, ComposeOption, SetOptionOpts } from 'echarts/core'
import type {
  BarSeriesOption,
  LineSeriesOption,
  SankeySeriesOption
} from 'echarts/charts'
import type {
  TitleComponentOption,
  GridComponentOption
} from 'echarts/components'
import { useResizeDetector, type OnResizeCallback } from 'react-resize-detector'
import { LegendComponentOption, LegendOption } from 'echarts/types/dist/shared'
import { BarLoading } from '@sentio/ui-core'
import { ChartLegend } from './ChartLegend'
import { isMobile } from '../utils/is-mobile'
import { registerSentioTheme } from './theme/register'
import { useDarkMode } from '../utils/use-dark-mode'
import { sansFontFamily } from './theme/sentio-theme'

// Register the required components
use([
  LegendComponent,
  PieChart,
  LineChart,
  ScatterChart,
  MarkLineComponent,
  MarkAreaComponent,
  BarChart,
  SankeyChart,
  GridComponent,
  TooltipComponent,
  BrushComponent,
  TitleComponent,
  ToolboxComponent, // A group of utility tools, which includes export, data view, dynamic type switching, data area zooming, and reset.
  DataZoomComponent, // Used in Line Graph Charts
  CanvasRenderer, // If you only need to use the canvas rendering mode, the bundle will not include the SVGRenderer module, which is not needed.
  GraphicComponent,
  SVGRenderer,
  VisualMapComponent
])

// Register the 'sentio' / 'sentio-dark' themes (idempotent). A function call,
// not a bare side-effect import, so it survives tree-shaking.
registerSentioTheme()

// Combine an Option type with only required components and charts via ComposeOption
export type EChartsOption = ComposeOption<
  | BarSeriesOption
  | LineSeriesOption
  | TitleComponentOption
  | GridComponentOption
  | SankeySeriesOption
  // | ScatterSeriesOption
>

export interface ReactEChartsProps {
  group?: string
  option: EChartsOption
  style?: CSSProperties
  settings?: SetOptionOpts
  loading?: boolean
  theme?: 'light' | 'dark' | 'sentio'
  minHeight?: number
  returnedSeries?: number
  totalSeries?: number
  onSelect?: (start: number, end: number) => void
  onZoom?: (start: number, end: number) => void
  noLegend?: boolean
  onClick?: (params: any, extraParams?: any) => void
  onInitChart?: (chart: ECharts) => void
  onSeriesEvent?: (
    event: 'click' | 'mouseover' | 'mouseout',
    params: any
  ) => void
}

export interface EChartsHandle {
  getEChart: () => ECharts | undefined
  highlightSeries: (highlighted?: SeriesFinder) => void
  getSeriesColor: (s: SeriesFinder) => string | undefined
  getFrame: () => HTMLDivElement | null
  toggleLegend: (legend: string, selected?: boolean) => void
  resize: (size: { width?: number; height?: number }) => void

  getSeries(seriesId: string): any
}

type SeriesFinder = {
  seriesId?: string
  seriesIndex?: number
  seriesName?: string
}
const ReactEChartsBaseComponent: React.ForwardRefRenderFunction<
  EChartsHandle,
  ReactEChartsProps
> = (
  {
    group,
    option,
    style,
    settings,
    loading,
    theme: _theme,
    // minHeight,
    returnedSeries,
    totalSeries,
    onSelect,
    noLegend,
    onZoom,
    onClick,
    onSeriesEvent,
    onInitChart
  }: ReactEChartsProps,
  forwardedRef
) => {
  const isDarkMode = useDarkMode()
  const theme = _theme || (isDarkMode ? 'sentio-dark' : 'sentio')
  const [legendSelected, setLegendSelected] = useState<Record<string, boolean>>(
    {}
  )
  const [chart, setChart] = useState<ECharts>()
  const echartInstanceRef = useRef<ECharts | undefined>()
  const [legendRendered, setLegendRendered] = useState(false)
  const chartRender = 'canvas'
  const frameRef = useRef<HTMLDivElement>(null)

  const chartHandle = useMemo(() => {
    return {
      getEChart: () => echartInstanceRef.current,
      highlightSeries(highlighted?: SeriesFinder) {
        const chart = echartInstanceRef.current
        if (chart) {
          const { series: s } = chart.getOption()
          const series = s as any[]
          if (highlighted) {
            for (let i = 0; i < series.length; i++) {
              const s = series[i]
              if (
                s.id == highlighted.seriesId ||
                highlighted.seriesIndex == i
              ) {
                s.lineStyle = s.lineStyle || {}
                s.lineStyle.opacity = 1
              } else {
                s.lineStyle = s.lineStyle || {}
                s.lineStyle.opacity = 0.2
              }
            }
          } else {
            series.forEach((s) => {
              s.lineStyle = s.lineStyle || {}
              s.lineStyle.opacity = 1
            })
          }
          chart.setOption({ series })
        }
      },
      getSeriesColor(s: SeriesFinder) {
        const chart = echartInstanceRef.current
        if (chart) {
          try {
            // Resolve a valid series index before calling getVisual to avoid
            // the "There is no specified series model" dev warning (ECharts warns
            // before throwing when the series can't be found).
            const { series: optionSeries } = chart.getOption()
            const seriesList = (optionSeries || []) as any[]

            let resolvedIndex = s.seriesIndex
            if ((resolvedIndex == null || resolvedIndex < 0) && s.seriesId) {
              resolvedIndex = seriesList.findIndex(
                (serie) => serie.id === s.seriesId
              )
            }
            if ((resolvedIndex == null || resolvedIndex < 0) && s.seriesName) {
              resolvedIndex = seriesList.findIndex(
                (serie) => serie.name === s.seriesName
              )
            }

            // Only call getVisual when we know the series exists.
            if (
              resolvedIndex != null &&
              resolvedIndex >= 0 &&
              resolvedIndex < seriesList.length
            ) {
              return chart.getVisual(
                { seriesIndex: resolvedIndex },
                'color'
              ) as string | undefined
            }
            return undefined
          } catch (e) {
            // ignore error
          }
        }
      },
      getFrame() {
        return frameRef.current
      },
      toggleLegend(name: string, selected?: boolean) {
        const chart = echartInstanceRef.current
        if (selected == null) {
          chart?.dispatchAction({
            type: 'legendToggleSelect',
            name
          })
        } else {
          chart?.dispatchAction({
            type: selected ? 'legendSelect' : 'legendUnSelect',
            name
          })
        }
      },
      getSeries(seriesId: string) {
        const chart = echartInstanceRef.current
        if (chart) {
          const { series: s } = chart.getOption()
          const series = s as any[]
          return series?.find((s) => s.id == seriesId)
        }
      },
      resize: (size) => {
        const chart = echartInstanceRef.current
        chart?.resize(size)
      }
    } as EChartsHandle
  }, [])

  useImperativeHandle(forwardedRef, () => {
    return chartHandle
  }, [chartHandle])

  const onResize: OnResizeCallback = useCallback(({ width, height }) => {
    const chart = echartInstanceRef.current
    chart?.resize({
      width: width ?? undefined,
      height: height ?? undefined
    })
  }, [])
  const {
    // width,
    // height,
    ref: chartRef
  } = useResizeDetector({
    onResize,
    refreshMode: 'throttle',
    refreshRate: 100
  })

  useEffect(() => {
    // Initialize chart
    let instance: ECharts
    const containerNode = frameRef.current?.querySelector('.echart-container')
    if (containerNode) {
      instance = init(containerNode as HTMLDivElement, theme, {
        renderer: chartRender,
        locale: 'EN'
      })
      echartInstanceRef.current = instance
      setChart(instance)
    }

    // Return cleanup function
    return () => {
      echartInstanceRef.current = undefined
      instance?.dispose()
    }
  }, [theme, chartRender])

  useEffect(() => {
    if (!chart || chart.isDisposed()) {
      return
    }
    chart.on('legendselected', (event: any) => {
      setLegendSelected(event.selected)
    })
    chart.on('legendunselected', (event: any) => {
      setLegendSelected(event.selected)
    })
    chart.on('legendselectchanged', (event: any) => {
      setLegendSelected(event.selected)
    })

    chart.on('brushEnd', (params: any) => {
      const areas = params.areas[0]
      if (areas) {
        const start = areas.coordRange[0]
        const end = areas.coordRange[1]
        onSelect && onSelect(start, end)
      }
    })
    if (onZoom) {
      chart.on('dataZoom', (params: any) => {
        onZoom(params.start, params.end)
      })
    }

    return () => {
      if (chart.isDisposed()) return
      chart.off('legendselectchanged')
      chart.off('brushEnd')
      chart.off('dataZoom')
    }
  }, [chart, onSelect, onZoom])

  useEffect(() => {
    if (!chart || chart.isDisposed() || !onClick) {
      return
    }
    chart.getZr()?.on('click', (params: any) => {
      const pointInPixel = [params.offsetX, params.offsetY]
      const pointInGrid = chart.convertFromPixel('grid', pointInPixel)
      onClick(pointInGrid, params)
    })
    if (onSeriesEvent) {
      chart.on('click', 'series', (params: any) => {
        onSeriesEvent?.('click', params)
      })
      chart.on('mouseover', 'series', (params: any) => {
        onSeriesEvent?.('mouseover', params)
      })
      chart.on('mouseout', 'series', (params: any) => {
        onSeriesEvent?.('mouseout', params)
      })
    }

    return () => {
      if (chart.isDisposed()) return
      chart.getZr()?.off('click')
      if (onSeriesEvent) {
        chart.off('click')
        chart.off('mouseout')
        chart.off('mouseover')
      }
    }
  }, [chart, onClick, onSeriesEvent, onInitChart])

  // Support X/Y Axis title
  const processedOption = useMemo(() => {
    if (!option) return option

    const processedOpt = { ...option }
    const graphicElements: any[] = []
    let hasYAxisName = false
    let hasXAxisName = false

    // Get text color based on theme
    const textColor = isDarkMode ? '#A6A6A6' : '#6E7079'

    // Common function to create axis name graphic element
    const createAxisNameElement = (
      name: string,
      isYAxis: boolean,
      axisIndex = 0
    ) => {
      const baseStyle = {
        text: name,
        fontSize: 11,
        fontFamily: sansFontFamily,
        fontWeight: 600,
        fill: textColor,
        textAlign: 'center' as const,
        textVerticalAlign: 'middle' as const
      }

      if (isYAxis) {
        return {
          type: 'text',
          left: axisIndex === 0 ? 8 : 'right',
          top: 'middle',
          rotation: Math.PI / 2,
          style: baseStyle,
          z: 100,
          silent: true
        }
      } else {
        return {
          type: 'text',
          left: 'center',
          bottom: axisIndex === 0 ? 8 : 'top',
          style: baseStyle,
          z: 100,
          silent: true
        }
      }
    }

    // Generic function to process axis names
    const processAxisName = (
      axisConfig: any,
      isYAxis: boolean,
      axisIndex = 0
    ) => {
      if (axisConfig && typeof axisConfig === 'object' && axisConfig.name) {
        if (isYAxis) {
          hasYAxisName = true
        } else {
          hasXAxisName = true
        }

        const { name, ...restAxis } = axisConfig
        const graphicElement = createAxisNameElement(name, isYAxis, axisIndex)
        graphicElements.push(graphicElement)
        return restAxis
      }
      return axisConfig
    }

    // Process both yAxis and xAxis using the generic function
    const processAxisArray = (axisOption: any, isYAxis: boolean) => {
      if (!axisOption) return axisOption

      if (Array.isArray(axisOption)) {
        return axisOption.map((axis, index) =>
          processAxisName(axis, isYAxis, index)
        )
      } else {
        return processAxisName(axisOption, isYAxis, 0)
      }
    }

    // Process axes
    processedOpt.yAxis = processAxisArray(option.yAxis, true)
    processedOpt.xAxis = processAxisArray(option.xAxis, false)

    // Adjust grid spacing when axis names are present
    if (hasYAxisName || hasXAxisName) {
      const originalGrid = processedOpt.grid || {}

      const adjustGridSpacing = (gridItem: any) => ({
        ...gridItem,
        left: hasYAxisName
          ? typeof gridItem.left === 'number'
            ? gridItem.left + 20
            : 32
          : gridItem.left,
        bottom: hasXAxisName
          ? typeof gridItem.bottom === 'number'
            ? gridItem.bottom + 20
            : 28
          : gridItem.bottom
      })

      processedOpt.grid = Array.isArray(originalGrid)
        ? originalGrid.map(adjustGridSpacing)
        : adjustGridSpacing(originalGrid)
    }

    // Add graphic elements to the processed option
    if (graphicElements.length > 0) {
      const existingGraphic = processedOpt.graphic
      if (existingGraphic) {
        processedOpt.graphic = Array.isArray(existingGraphic)
          ? [...existingGraphic, ...graphicElements]
          : [existingGraphic, ...graphicElements]
      } else {
        processedOpt.graphic = graphicElements
      }
    }

    return processedOpt
  }, [option, isDarkMode])

  useEffect(() => {
    if (!chart || chart.isDisposed()) {
      return
    }
    try {
      chart.setOption(
        {
          ...processedOption,
          legend: {
            ...(processedOption.legend as LegendOption),
            // Persist legend selected state between re-render.
            ...(legendSelected ? { selected: legendSelected } : {})
          }
        },
        { ...settings, notMerge: true }
      )
    } catch (e) {
      console.error('echarts set option failed', e, processedOption)
    }
    onInitChart?.(chart)

    if (!isMobile()) {
      // Don't allow brush on mobile
      chart.dispatchAction({
        type: 'brush',
        command: 'clear',
        areas: []
      })
      chart.dispatchAction({
        type: 'takeGlobalCursor',
        key: 'brush',
        brushOption: {
          brushType: 'lineX',
          brushMode: 'single'
        }
      })
    }
  }, [
    chart,
    processedOption,
    settings,
    theme,
    onSelect,
    legendSelected,
    onInitChart
  ])

  useEffect(() => {
    if (loading) {
      setLegendRendered(false)
    }
  }, [loading])

  useEffect(() => {
    if (chart && !chart.isDisposed()) {
      chart.group = group || ''
    }
  }, [chart, group])

  const onMouseDown = useCallback(
    (event: React.MouseEvent) => {
      // Cancel brush selection if the pressed button is not the main button.
      if (event.button !== 0 && chartRef.current) {
        chartRef.current
          .querySelector(chartRender)
          ?.dispatchEvent(
            new MouseEvent('mouseup', event as unknown as MouseEventInit)
          )
      }
    },
    [chartRef, chartRender]
  )

  const legends = noLegend ? null : (
    <>
      {chart && !loading && (
        <ChartLegend
          legend={
            ((option?.legend as LegendComponentOption)?.data as string[]) || []
          }
          chartHandle={chartHandle}
          legendSelected={legendSelected}
          returnedSeries={returnedSeries}
          totalSeries={totalSeries}
          onRendered={setLegendRendered}
        />
      )}
    </>
  )

  return (
    <div
      className="relative mb-1 grid h-full"
      style={{ gridTemplateRows: '1fr max-content', height: '270px', ...style }}
      onMouseDown={onMouseDown}
      ref={frameRef}
    >
      <div
        ref={chartRef}
        className="echart-container min-h-0 w-full min-w-0"
      ></div>
      {legends}
      {loading && (
        <BarLoading
          className="bg-default-bg absolute w-full"
          hint="Loading"
          width={100}
        />
      )}
    </div>
  )
}

export const ReactEChartsBase = forwardRef(ReactEChartsBaseComponent)
