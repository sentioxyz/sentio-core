import { produce } from 'immer'
import { defaults } from 'lodash'
import {
  Checkbox,
  DisclosurePanel,
  NewButtonGroup as ButtonGroup
} from '@sentio/ui-core'
import type { CalculationLike, PieConfigLike, PieTypeLike } from '../../types'

interface Props {
  config?: PieConfigLike
  defaultOpen?: boolean
  onChange: (config: PieConfigLike) => void
}

export const defaultConfig: PieConfigLike = {
  pieType: 'Pie',
  calculation: 'LAST',
  showPercent: true,
  showValue: true,
  absValue: false
}

const CalculationItems = [
  { label: 'Last', value: 'LAST' },
  { label: 'First', value: 'FIRST' },
  { label: 'Total', value: 'TOTAL' },
  { label: 'Mean', value: 'MEAN' },
  { label: 'Max', value: 'MAX' },
  { label: 'Min', value: 'MIN' }
]

const PieTypeItems: { label: string; value: PieTypeLike }[] = [
  { label: 'Pie', value: 'Pie' },
  { label: 'Donut', value: 'Donut' }
]

export function PieChartControls({ config, defaultOpen, onChange }: Props) {
  config = defaults(config, defaultConfig)

  function onCalculationChange(cal: CalculationLike) {
    config &&
      onChange(produce(config, (draft) => void (draft.calculation = cal)))
  }

  function onPieTypeChange(pieType: PieTypeLike) {
    config &&
      onChange(produce(config, (draft) => void (draft.pieType = pieType)))
  }

  function toggle(
    field: 'showValue' | 'showPercent' | 'absValue',
    value: boolean
  ) {
    config &&
      onChange(
        produce(config, (draft) => {
          draft[field] = value
        })
      )
  }

  return (
    <DisclosurePanel
      title="Pie Chart Options"
      defaultOpen={defaultOpen}
      containerClassName="w-full bg-default-bg"
    >
      <div className="flex items-center gap-4">
        <div className="shadow-xs flex rounded-md">
          <ButtonGroup
            small
            buttons={PieTypeItems}
            value={config.pieType}
            onChange={onPieTypeChange}
          />
        </div>
        <div className="shadow-xs flex rounded-md">
          <span className="sm:text-ilabel border-main inline-flex items-center rounded-l-md  border bg-gray-50 px-3 ">
            Calculation
          </span>
          <select
            value={config.calculation}
            className="sm:text-ilabel text-text-foreground border-main inline-flex items-center rounded-r-md border  border-l-0   pl-4 pr-7"
            onChange={(e) =>
              onCalculationChange(e.target.value as CalculationLike)
            }
          >
            {CalculationItems.map((d) => {
              return (
                <option key={d.value} value={d.value}>
                  {d.label}
                </option>
              )
            })}
          </select>
        </div>
        <Checkbox
          checked={config?.showValue}
          onChange={(v) => toggle('showValue', v)}
          label="Show value"
          labelClassName="whitespace-nowrap"
        />
        <Checkbox
          checked={config?.showPercent}
          onChange={(v) => toggle('showPercent', v)}
          label="Show percent"
          labelClassName="whitespace-nowrap"
        />
        <Checkbox
          checked={config?.absValue}
          onChange={(v) => toggle('absValue', v)}
          label="Use absolute values"
          labelClassName="whitespace-nowrap"
        />
      </div>
    </DisclosurePanel>
  )
}
