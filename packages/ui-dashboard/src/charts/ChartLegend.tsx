import React, { useCallback, useEffect, useRef, useState } from 'react'

import { Tooltip } from '../common/Tooltip'
import { EChartsHandle } from './EchartsBase'

const COLOR_UNSELECTED = '#dddddd'

interface Props {
  legend: string[]
  legendSelected: Record<string, boolean>
  returnedSeries?: number
  totalSeries?: number
  onRendered: (v: boolean) => void
  chartHandle?: EChartsHandle
}

export const ChartLegend = ({
  legend,
  legendSelected,
  returnedSeries,
  totalSeries,
  onRendered,
  chartHandle
}: Props) => {
  const rootRef = useRef<HTMLDivElement>(null)
  const [tooltipText, setTooltipText] = useState('')
  const [tooltipReferenceElement, setTooltipReferenceElement] =
    useState<HTMLDivElement>()

  useEffect(() => {
    if (!rootRef.current) {
      return
    }
    const offsetHeight = rootRef?.current?.parentElement?.offsetHeight || 0
    chartHandle?.resize({
      height: offsetHeight - rootRef.current.offsetHeight
    })
    onRendered(true)
  }, [chartHandle, onRendered])

  const onToggleLegend = useCallback(
    (event: React.MouseEvent, name: string, _seriesIndex: number) => {
      if (event.altKey) {
        legend.forEach((n) => {
          chartHandle?.toggleLegend(n, n === name)
        })
        return
      }
      chartHandle?.toggleLegend(name)
    },
    [chartHandle, legendSelected]
  )

  const highlightSeries = useCallback(
    (index: number) => {
      chartHandle?.highlightSeries({ seriesIndex: index })
    },
    [chartHandle]
  )

  const unhighlightSeries = useCallback(() => {
    chartHandle?.highlightSeries(undefined)
  }, [chartHandle])

  const onToggleAll = useCallback(
    (
      legend: string[],
      legendSelected: Record<string, boolean>,
      chartHandle?: EChartsHandle
    ) => {
      const allSelected = legend.every((name) => legendSelected[name])
      legend.forEach((name) => {
        chartHandle?.toggleLegend(name, !allSelected)
      })
    },
    [legend, legendSelected, chartHandle]
  )

  const list = legend.map((name, index) => {
    const selected = legendSelected[name] || legendSelected[name] === undefined
    return (
      <div
        className="flex cursor-pointer items-center gap-0.5 whitespace-nowrap"
        key={name + index}
        data-tip={name}
        onClick={(event) => onToggleLegend(event, name, index)}
        onDoubleClick={(event) => {
          onToggleAll(legend, legendSelected, chartHandle)
        }}
        onMouseEnter={(e) => {
          if (legendSelected[name] !== false) {
            // Only highlight when the current legend is active.
            highlightSeries(index)
          }
          setTooltipReferenceElement(e.currentTarget)
          setTooltipText(name)
        }}
        onMouseLeave={() => {
          unhighlightSeries()
          setTooltipReferenceElement(undefined)
          setTooltipText('')
        }}
      >
        <span
          className="rounded-xs h-2.5 w-2.5"
          style={{
            backgroundColor: selected
              ? chartHandle?.getSeriesColor({ seriesName: name })
              : COLOR_UNSELECTED
          }}
        />
        <span
          className="truncate text-xs"
          style={{
            maxWidth: '12em',
            color: selected ? undefined : COLOR_UNSELECTED
          }}
        >
          {name}
        </span>
      </div>
    )
  })

  return (
    <div
      ref={rootRef}
      className="text-text-foreground-secondary flex max-h-10 flex-wrap gap-x-3 gap-y-1 overflow-y-auto px-2 text-[13px] leading-[18px]"
    >
      {list}
      {returnedSeries && totalSeries && returnedSeries < totalSeries ? (
        <div className="font-semibold" style={{ color: '#6B7280' }}>
          showing {returnedSeries} of {totalSeries} series
        </div>
      ) : null}
      <Tooltip referenceElement={tooltipReferenceElement} text={tooltipText} />
    </div>
  )
}
