import { produce } from 'immer'
import { defaults } from 'lodash'
import { useCallback, type ReactNode } from 'react'
import { DisclosurePanel } from '@sentio/ui-core'
import type { ScatterConfigLike } from '../../types'

interface Props {
  config?: ScatterConfigLike
  defaultOpen?: boolean
  onChange: (config: ScatterConfigLike) => void
  /** App-supplied sql column picker (Size By Column). */
  columnSelect?: (props: {
    value?: string
    onChange: (col: string) => void
  }) => ReactNode
  /** App-supplied color picker (Size Color Mapping). */
  colorPicker?: (props: {
    value?: string
    onChange: (color?: string) => void
  }) => ReactNode
}

export const defaultConfig: ScatterConfigLike = {
  minSize: 5,
  maxSize: 30
}

export function ScatterControls({
  config,
  defaultOpen,
  onChange,
  columnSelect,
  colorPicker
}: Props) {
  config = defaults(config, defaultConfig)

  const onSymbolSizeColumnChange = useCallback(
    (column: string) => {
      config &&
        onChange(produce(config, (draft) => void (draft.symbolSize = column)))
    },
    [config, onChange]
  )

  const onSymbolColorChange = useCallback(
    (color?: string) => {
      config && onChange(produce(config, (draft) => void (draft.color = color)))
    },
    [config, onChange]
  )

  const onMinSizeChange = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const value = parseInt(event.target.value) || 5
      config &&
        onChange(produce(config, (draft) => void (draft.minSize = value)))
    },
    [config, onChange]
  )

  const onMaxSizeChange = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const value = parseInt(event.target.value) || 50
      config &&
        onChange(produce(config, (draft) => void (draft.maxSize = value)))
    },
    [config, onChange]
  )

  return (
    <DisclosurePanel
      title="Scatter Chart Options"
      defaultOpen={defaultOpen}
      containerClassName="w-full bg-default-bg"
    >
      <div className="flex items-center gap-4">
        {columnSelect && (
          <div className="inline-flex h-8">
            <span className="sm:text-icontent border-main inline-flex items-center rounded-l-md border bg-gray-50 px-2">
              Size By Column
            </span>
            {columnSelect({
              value: config.symbolSize,
              onChange: onSymbolSizeColumnChange
            })}
          </div>
        )}
        {colorPicker && (
          <div className="inline-flex h-8">
            <span className="sm:text-icontent border-main inline-flex items-center rounded-l-md border bg-gray-50 px-2">
              Size Color Mapping
            </span>
            {colorPicker({
              value: config.color,
              onChange: onSymbolColorChange
            })}
          </div>
        )}
        <div className="inline-flex h-8">
          <span className="sm:text-icontent border-main inline-flex items-center rounded-l-md border bg-gray-50 px-2">
            Min Size
          </span>
          <input
            name="minSize"
            type="number"
            className="focus:ring-primary-500 focus:border-primary-500 border-main h-8 w-24 rounded-r-md border border-l-0 px-2 text-sm focus:ring-1"
            value={config.minSize || 5}
            onChange={onMinSizeChange}
            min="1"
            max="60"
          />
        </div>
        <div className="inline-flex h-8">
          <span className="sm:text-icontent border-main inline-flex items-center rounded-l-md border bg-gray-50 px-2">
            Max Size
          </span>
          <input
            name="maxSize"
            type="number"
            className="focus:ring-primary-500 focus:border-primary-500 border-main h-8 w-24 rounded-r-md border border-l-0 px-2 text-sm focus:ring-1"
            value={config.maxSize || 30}
            onChange={onMaxSizeChange}
            min="1"
            max="60"
          />
        </div>
      </div>
    </DisclosurePanel>
  )
}
