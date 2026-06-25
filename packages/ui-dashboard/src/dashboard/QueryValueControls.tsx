import { produce } from 'immer'
import { defaults } from 'lodash'
import { Checkbox, DisclosurePanel } from '@sentio/ui-core'
import type { ReactNode } from 'react'
import type { QueryValueConfigLike, ColorThemeLike } from '../types/chart'
import type { CalculationLike } from '../types/enums'

interface Props {
  config?: QueryValueConfigLike
  defaultOpen?: boolean
  onChange: (config: QueryValueConfigLike) => void
  // ColorSelect is bound to the app's query-value-theme (pulled into the chart
  // web worker), so it stays app-side and is injected as a slot.
  renderColorSelect: (
    value: ColorThemeLike | undefined,
    onChange: (picked: { value?: ColorThemeLike }) => void
  ) => ReactNode
}

export const defaultConfig: QueryValueConfigLike = {
  calculation: 'LAST',
  colorTheme: {
    themeType: 'Gray'
  },
  showBackgroundChart: false
}

const CalculationItems = [
  { label: 'Last', value: 'LAST' },
  { label: 'First', value: 'FIRST' },
  { label: 'Total', value: 'TOTAL' },
  { label: 'Mean', value: 'MEAN' },
  { label: 'Max', value: 'MAX' },
  { label: 'Min', value: 'MIN' }
]

export function QueryValueControls({
  config,
  defaultOpen,
  onChange,
  renderColorSelect
}: Props) {
  config = defaults(config, defaultConfig)

  function onCalculationChange(cal: CalculationLike) {
    config &&
      onChange(produce(config, (draft) => void (draft.calculation = cal)))
  }

  function onSeriesCalculationChange(cal: CalculationLike) {
    config &&
      onChange(produce(config, (draft) => void (draft.seriesCalculation = cal)))
  }

  function onSelectColor(c: { value?: ColorThemeLike }) {
    config &&
      onChange(
        produce(config, (draft) => {
          draft.colorTheme = c.value
        })
      )
  }

  function toggleShowBackgroundChart() {
    config &&
      onChange(
        produce(config, (draft) => {
          draft.showBackgroundChart = !draft.showBackgroundChart
        })
      )
  }

  return (
    <DisclosurePanel
      title="Query Value Options"
      defaultOpen={defaultOpen}
      containerClassName="w-full bg-default-bg"
    >
      <div className="flex flex-wrap items-center gap-4">
        <div className="shadow-xs flex h-8 rounded-md">
          <span className="sm:text-ilabel border-main text-text-foreground inline-flex items-center whitespace-nowrap rounded-l-md border bg-gray-50 px-3">
            For each series, calculate the
          </span>
          <select
            value={config?.calculation}
            className="sm:text-ilabel border-main text-text-foreground inline-flex items-center border border-x-0 py-0.5 pl-4 pr-7"
            onChange={(e) =>
              onCalculationChange(e.target.value as CalculationLike)
            }
          >
            {CalculationItems.map((d) => (
              <option key={d.value} value={d.value}>
                {d.label}
              </option>
            ))}
          </select>
          <span className="sm:text-ilabel border-main text-text-foreground inline-flex items-center whitespace-nowrap border bg-gray-50 px-3">
            value, then show the
          </span>
          <select
            value={config?.seriesCalculation}
            className="sm:text-ilabel border-main text-text-foreground inline-flex items-center border border-x-0 py-0.5 pl-4 pr-7"
            onChange={(e) =>
              onSeriesCalculationChange(e.target.value as CalculationLike)
            }
          >
            {CalculationItems.map((d) => (
              <option key={d.value} value={d.value}>
                {d.label}
              </option>
            ))}
          </select>
          <span className="sm:text-ilabel border-main text-text-foreground inline-flex items-center whitespace-nowrap rounded-r-md border bg-gray-50 px-3">
            value of multiple series
          </span>
        </div>

        <div className="focus-within:ring-primary-500 shadow-xs border-main flex h-8 divide-x divide-gray-300 rounded-md border focus-within:border-transparent focus-within:ring-2">
          <span className="sm:text-ilabel text-text-foreground inline-flex items-center whitespace-nowrap rounded-l-md bg-gray-50 px-3">
            Color Theme
          </span>
          {renderColorSelect(config?.colorTheme, onSelectColor)}
        </div>
        <Checkbox
          checked={config?.showBackgroundChart}
          onChange={toggleShowBackgroundChart}
          label="Show Background Chart"
        />
      </div>
    </DisclosurePanel>
  )
}
