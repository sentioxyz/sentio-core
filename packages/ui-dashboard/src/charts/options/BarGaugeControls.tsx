import { produce } from 'immer'
import { defaults } from 'lodash'
import { DisclosurePanel } from '@sentio/ui-core'
import type {
  BarGaugeConfigLike,
  CalculationLike,
  DirectionLike,
  SortByLike
} from '../../types'

interface Props {
  config?: BarGaugeConfigLike
  defaultOpen?: boolean
  onChange: (config: BarGaugeConfigLike) => void
}

export const defaultConfig: BarGaugeConfigLike = {
  direction: 'HORIZONTAL',
  calculation: 'LAST',
  sort: {
    sortBy: 'ByName',
    orderDesc: true
  }
}

const directionItems = [
  { label: 'Horizontal', value: 'HORIZONTAL' },
  { label: 'Vertical', value: 'VERTICAL' }
]

const CalculationItems = [
  { label: 'Last', value: 'LAST' },
  { label: 'First', value: 'FIRST' },
  { label: 'Total', value: 'TOTAL' },
  { label: 'Mean', value: 'MEAN' },
  { label: 'Max', value: 'MAX' },
  { label: 'Min', value: 'MIN' }
]

const sortByItems = [
  { label: 'Name', value: 'ByName' },
  { label: 'Value', value: 'ByValue' }
]

const orderItems = [
  { label: 'Ascendant', value: 'false' },
  { label: 'Descendant', value: 'true' }
]

export function BarGaugeControls({ config, defaultOpen, onChange }: Props) {
  config = defaults(config, defaultConfig)

  function onCalculationChange(cal: CalculationLike) {
    config &&
      onChange(produce(config, (draft) => void (draft.calculation = cal)))
  }

  function onDirectionChange(dir: DirectionLike) {
    config && onChange(produce(config, (draft) => void (draft.direction = dir)))
  }

  function onOrderChange(orderDesc: boolean) {
    config &&
      onChange(
        produce(config, (draft) => {
          draft.sort = draft.sort || {}
          draft.sort.orderDesc = orderDesc
        })
      )
  }

  function onSortByChange(sortBy: SortByLike) {
    config &&
      onChange(
        produce(config, (draft) => {
          draft.sort = draft.sort || {}
          draft.sort.sortBy = sortBy
        })
      )
  }

  return (
    <DisclosurePanel
      title="Bar Gauge Options"
      defaultOpen={defaultOpen}
      containerClassName="w-full bg-default-bg"
    >
      <div className="flex items-center gap-4">
        <div className="shadow-xs  flex rounded-md">
          <span className="sm:text-ilabel border-main inline-flex items-center rounded-l-md  border bg-gray-50 px-3 ">
            Direction
          </span>
          <select
            value={config.direction}
            className="sm:text-ilabel border-main text-text-foreground inline-flex items-center rounded-r-md border  border-l-0   pl-4 pr-7"
            onChange={(e) => onDirectionChange(e.target.value as DirectionLike)}
          >
            {directionItems.map((d) => {
              return (
                <option key={d.value} value={d.value}>
                  {d.label}
                </option>
              )
            })}
          </select>
        </div>

        <div className="shadow-xs flex rounded-md">
          <span className="sm:text-ilabel border-main inline-flex items-center rounded-l-md  border bg-gray-50 px-3 ">
            Calculation
          </span>
          <select
            value={config.calculation}
            className="sm:text-ilabel border-main text-text-foreground inline-flex items-center rounded-r-md border  border-l-0   pl-4 pr-7"
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

        <div className="shadow-xs flex rounded-md">
          <span className="sm:text-ilabel border-main inline-flex items-center rounded-l-md  border bg-gray-50 px-3 ">
            Sort by
          </span>
          <select
            value={config?.sort?.sortBy}
            className="sm:text-ilabel border-main text-text-foreground inline-flex items-center border    border-l-0   pl-4 pr-7"
            onChange={(e) => onSortByChange(e.target.value as SortByLike)}
          >
            {sortByItems.map((d) => {
              return (
                <option key={d.value} value={d.value}>
                  {d.label}
                </option>
              )
            })}
          </select>
          <select
            value={config?.sort?.orderDesc + ''}
            className="sm:text-ilabel border-main text-text-foreground inline-flex items-center rounded-r-md border  border-l-0   pl-4 pr-7"
            onChange={(e) => onOrderChange(e.target.value === 'true')}
          >
            {orderItems.map((d) => {
              return (
                <option key={d.label} value={d.value + ''}>
                  {d.label}
                </option>
              )
            })}
          </select>
        </div>
      </div>
    </DisclosurePanel>
  )
}
