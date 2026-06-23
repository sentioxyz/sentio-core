import { produce } from 'immer'
import { defaults } from 'lodash'
import { DisclosurePanel } from '@sentio/ui-core'
import { AddonLabel } from './controls-ui'
import type { ChartConfigLike, DataConfigLike } from '../../types'

interface Props {
  defaultOpen?: boolean
  onChange: (config: DataConfigLike) => void
  chartConfig?: ChartConfigLike
  /** Per-chart-type fallback when no explicit limit is set (app resolves from ChartTypeLimits). */
  defaultSeriesLimit?: number
}

export const defaultConfig: DataConfigLike = {
  seriesLimit: undefined
}

export function DataControls({
  defaultOpen,
  onChange,
  chartConfig,
  defaultSeriesLimit = 20
}: Props) {
  const config = defaults(chartConfig?.dataConfig, defaultConfig)

  // migrate tableConfig.rowLimit to dataConfig.seriesLimit
  const currentSeriesLimit =
    config?.seriesLimit || chartConfig?.tableConfig?.rowLimit

  function onSeriesLimitChange(e: React.ChangeEvent<HTMLInputElement>) {
    const value = parseInt(e.target.value)
    if (value > 1000) {
      return
    }
    config &&
      onChange(
        produce(config, (draft) => {
          draft.seriesLimit = value
        })
      )
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    // Prevent non-numeric character input (except for delete, backspace, arrow keys, etc.)
    if (
      !/[0-9]/.test(e.key) &&
      !['Backspace', 'Delete', 'ArrowLeft', 'ArrowRight', 'Tab'].includes(e.key)
    ) {
      e.preventDefault()
    }
  }

  return (
    <DisclosurePanel
      title="Data Options"
      defaultOpen={defaultOpen}
      containerClassName="w-full bg-default-bg"
    >
      <div className="flex h-8">
        <AddonLabel className="rounded-l-md border border-r-0 px-3">
          Max Series Limit
        </AddonLabel>
        <input
          type="number"
          max={1000}
          min={20}
          className="text-icontent border-main hover:border-primary-600 focus:ring-3 focus:ring-primary-600/30 focus:border-primary-600 mr-1 w-32 rounded-r-md"
          value={currentSeriesLimit ?? defaultSeriesLimit}
          onChange={onSeriesLimitChange}
          onKeyDown={onKeyDown}
        />
      </div>
    </DisclosurePanel>
  )
}
