import { useEffect, useRef, useState, useLayoutEffect } from 'react'
import { Select, classNames } from '@sentio/ui-core'
import LineIcon from '../charts/icons/LineIcon'
import BarIcon from '../charts/icons/BarIcon'
import AreaIcon from '../charts/icons/AreaIcon'
import { useVirtualizer } from '@tanstack/react-virtual'
import { LuChevronRight, LuMountainSnow } from 'react-icons/lu'
import { cloneDeep } from 'lodash'
import { Disclosure } from '@headlessui/react'
import type { ChartConfigLike, SeriesConfigLike } from '../types/chart'
import type { ChartTypeLike } from '../types/enums'

interface Props {
  config?: ChartConfigLike
  chartType?: ChartTypeLike
  setSeriesConfig: (seriesConfig: SeriesConfigLike) => void
  // Series names computed by the app (worker parses the SQL response); injected
  // so this panel stays free of the web worker.
  series: string[]
  // Mixed-chart feature flag, injected by the app.
  enabled?: boolean
}

// Define the available chart types for individual series
const seriesChartTypes = [
  {
    value: 'default',
    label: (
      <div className="flex items-center">
        <LuMountainSnow className="mr-2 h-4 w-4" />
        Default
      </div>
    )
  },
  {
    value: 'LINE',
    label: (
      <div className="flex items-center">
        <LineIcon className="mr-2 h-4 w-4" />
        Line
      </div>
    )
  },
  {
    value: 'BAR',
    label: (
      <div className="flex items-center">
        <BarIcon className="mr-2 h-4 w-4" />
        Bar
      </div>
    )
  },
  {
    value: 'AREA',
    label: (
      <div className="flex items-center">
        <AreaIcon className="mr-2 h-4 w-4" />
        Area
      </div>
    )
  }
]

export const SeriesControls = ({
  config,
  setSeriesConfig,
  series,
  enabled
}: Props) => {
  const parentRef = useRef<HTMLDivElement>(null)
  const [isDisclosureOpen, setIsDisclosureOpen] = useState(true)

  // Only use virtualization when there are more than 10 series
  const shouldVirtualize = series.length > 10

  // Setup virtualizer only when disclosure is open and should virtualize
  const virtualizer = useVirtualizer({
    count: shouldVirtualize && isDisclosureOpen ? series.length : 0,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 40,
    overscan: 5 // Render 5 extra items outside viewport for smooth scrolling
  })

  // Re-measure virtualizer when disclosure state changes
  useLayoutEffect(() => {
    if (isDisclosureOpen && shouldVirtualize && parentRef.current) {
      // Small delay to ensure DOM is updated
      const timeoutId = setTimeout(() => {
        virtualizer.measure()
      }, 10)
      return () => clearTimeout(timeoutId)
    }
  }, [isDisclosureOpen, shouldVirtualize, virtualizer])

  const handleSeriesTypeChange = (seriesName: string, selectedType: string) => {
    const currentSeriesConfig = config?.seriesConfig || { series: {} }

    if (selectedType === 'default') {
      // For 'default', we'll remove the series from config to use chart's default type
      const newSeriesConfig = cloneDeep(currentSeriesConfig)
      if (newSeriesConfig.series && newSeriesConfig.series[seriesName]) {
        delete newSeriesConfig.series[seriesName]
      }
      setSeriesConfig(newSeriesConfig)
      return
    }

    const newSeriesConfig: SeriesConfigLike = {
      ...currentSeriesConfig,
      series: {
        ...currentSeriesConfig.series,
        [seriesName]: { type: selectedType as ChartTypeLike }
      }
    }

    setSeriesConfig(newSeriesConfig)
  }

  const handleReset = () => {
    setSeriesConfig({ series: {} })
  }

  // Render single series item component
  const renderSeriesItem = (seriesName: string) => {
    const currentType =
      config?.seriesConfig?.series?.[seriesName]?.type || 'default'

    return (
      <div
        key={seriesName}
        className="text-icontent inline-flex h-8 w-full basis-0 px-2"
      >
        <div className="sm:text-icontent bg-sentio-gray-100 dark:bg-sentio-gray-200 border-main inline-flex shrink items-center rounded-l-md border border-r-0 px-2 font-medium sm:min-w-[160px]">
          <span className="truncate" title={seriesName}>
            {seriesName}
          </span>
        </div>
        <span className="sm:text-icontent bg-sentio-gray-100 dark:bg-sentio-gray-200 border-main inline-flex items-center whitespace-nowrap border border-r-0 px-2">
          Show as
        </span>
        <div className="w-40">
          <Select
            options={seriesChartTypes}
            value={currentType}
            onChange={(selectedType) =>
              handleSeriesTypeChange(seriesName, selectedType as string)
            }
            className="focus:border-primary-500 sm:text-icontent border-main h-full rounded-r-md border"
            buttonClassName="border-none! h-full!"
            placeholder="Select chart type"
            asLayer={true}
          />
        </div>
      </div>
    )
  }

  if (!enabled) {
    return null
  }

  const titleWithReset = (
    <div className="flex w-full items-center justify-between pr-2">
      <span>{`Series (${series.length})`}</span>
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation() // Prevent disclosure toggle
          handleReset()
        }}
        className="rounded-sm bg-gray-200 px-2 py-1 text-xs transition-colors hover:bg-gray-300"
        title="Reset all series to default"
      >
        Reset
      </button>
    </div>
  )

  return (
    <Disclosure defaultOpen={true}>
      {({ open }) => {
        // Synchronize disclosure state with `open` using useEffect
        // eslint-disable-next-line react-hooks/rules-of-hooks
        useEffect(() => {
          setIsDisclosureOpen(open)
        }, [open])

        return (
          <div className="bg-default-bg w-full rounded-sm">
            <Disclosure.Button
              className={classNames(
                open ? 'rounded-t' : 'rounded-sm',
                'focus-visible:ring-primary-500/75 text-ilabel font-ilabel text-text-foreground hover:bg-sentio-gray-100 dark:hover:bg-sentio-gray-400 focus:outline-hidden focus-visible:ring-3 flex w-full px-2 py-1.5 text-left'
              )}
            >
              <LuChevronRight
                className={classNames(
                  open ? 'rotate-90 transform' : '',
                  'mr-1 h-5 w-5 self-center transition-all'
                )}
              />
              {titleWithReset}
            </Disclosure.Button>
            <Disclosure.Panel className="p-2">
              {shouldVirtualize && open ? (
                // Virtualized rendering for large lists - only render when open
                <div
                  ref={parentRef}
                  className="text-icontent h-[200px] overflow-auto"
                  style={{
                    contain: 'strict'
                  }}
                >
                  <div
                    style={{
                      height: `${virtualizer?.getTotalSize() ?? 0}px`,
                      width: '100%',
                      position: 'relative'
                    }}
                  >
                    {virtualizer?.getVirtualItems().map((virtualItem) => {
                      const seriesName = series[virtualItem.index]
                      if (!seriesName) return null

                      return (
                        <div
                          key={virtualItem.key}
                          style={{
                            position: 'absolute',
                            top: 0,
                            left: 0,
                            width: '100%',
                            height: `${virtualItem.size}px`,
                            transform: `translateY(${virtualItem.start}px)`
                          }}
                        >
                          {renderSeriesItem(seriesName)}
                        </div>
                      )
                    })}
                  </div>
                </div>
              ) : (
                // Normal rendering for small lists
                <div className="text-icontent flex max-h-[200px] flex-col gap-2 overflow-y-auto">
                  {series.map((seriesName) => renderSeriesItem(seriesName))}
                </div>
              )}
            </Disclosure.Panel>
          </div>
        )
      }}
    </Disclosure>
  )
}
