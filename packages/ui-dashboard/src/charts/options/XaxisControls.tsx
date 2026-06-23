import type { ReactNode } from 'react'
import {
  Button,
  DisclosurePanel,
  PopoverTooltip,
  Select
} from '@sentio/ui-core'
import { AddonLabel } from './controls-ui'
import { LuInfo } from 'react-icons/lu'
import type { SortByLike, XAxisConfigLike } from '../../types'

const TypeSelect = Select<string>
const SortSelect = Select<string>
const OrderSelect = Select<boolean | undefined>

const sortByItems = [
  { label: 'Name', value: 'ByName' },
  { label: 'Value', value: 'ByValue' }
]
const orderItems = [
  { label: 'Ascendant', value: false },
  { label: 'Descendant', value: true }
]

interface Props {
  xAxis?: XAxisConfigLike
  setXAxis: (val?: XAxisConfigLike) => void
  defaultOpen?: boolean
  /** Optional app-supplied column picker (sql charts). When present the Column row renders. */
  columnSelect?: ReactNode
  /** Whether the selected X column is a non-TIME column (app resolves from the data schema). */
  columnIsNonTime?: boolean
  panelTitle?: string
  supportName?: boolean
  supportSort?: boolean
  supportSetType?: boolean
}

export const XAxisControls = ({
  xAxis,
  setXAxis,
  defaultOpen,
  columnSelect,
  columnIsNonTime,
  panelTitle = 'X-Axis Controls',
  supportName = true,
  supportSort,
  supportSetType
}: Props) => {
  const onChangeInput =
    (field: string) => (event: React.ChangeEvent<HTMLInputElement>) => {
      const { value } = event.target
      setXAxis({ ...xAxis, [field]: value || undefined })
    }
  const onClickResetXAxis = () => {
    setXAxis(undefined)
  }

  const isXAixsNoneTime = xAxis && xAxis.column && columnIsNonTime

  return (
    <DisclosurePanel
      title={panelTitle}
      defaultOpen={defaultOpen}
      containerClassName="w-full bg-default-bg"
    >
      <div className="text-icontent flex flex-wrap gap-[10px]">
        {supportName && (
          <label className="inline-flex h-8">
            <AddonLabel className="rounded-l-md border border-r-0 px-2">
              Name
            </AddonLabel>
            <input
              type="text"
              className="sm:text-icontent border-main hover:border-primary-600 focus:ring-3 focus:ring-primary-600/30 focus:border-primary-600 w-40 rounded-r-md"
              value={xAxis?.name}
              placeholder="Axis Name"
              onChange={onChangeInput('name')}
            />
          </label>
        )}
        {supportSetType && (
          <span className="inline-flex h-8">
            <span className="sm:text-icontent border-main inline-flex items-center rounded-l-md border border-r-0 bg-gray-50 px-2">
              X-Axis Type{' '}
              <PopoverTooltip
                strategy="fixed"
                hideArrow
                placementOption="bottom"
                text={
                  <div className="text-text-foreground max-w-[300px] p-2">
                    <span className="font-medium">Discrete axis</span> displays{' '}
                    <span className="text-primary-600">
                      discrete values evenly
                    </span>
                    , while <span className="font-medium">Continuous axis</span>{' '}
                    shows{' '}
                    <span className="text-primary-600">
                      continuous time series
                    </span>{' '}
                    and{' '}
                    <span className="text-primary-600">
                      auto-fills missing time points
                    </span>
                  </div>
                }
              >
                <LuInfo className="text-text-foreground-secondary h-4 w-4" />
              </PopoverTooltip>
            </span>
            <TypeSelect
              className="h-8 w-40"
              buttonClassName="h-full border-main rounded-l-none items-center inline-flex items-center"
              value={xAxis?.type || 'time'}
              onChange={(val) => {
                setXAxis({ ...xAxis, type: val })
              }}
              options={[
                { label: 'Continuous', value: 'time' },
                { label: 'Discrete', value: 'category' }
              ]}
            />
          </span>
        )}
        {columnSelect && (
          <span className="inline-flex h-8">
            <AddonLabel className="rounded-l-md border px-2">Column</AddonLabel>
            {columnSelect}
          </span>
        )}
        {supportSort && isXAixsNoneTime && (
          <span className="inline-flex h-8">
            <AddonLabel className="rounded-l-md border border-r-0 px-2">
              Sort By
            </AddonLabel>
            <SortSelect
              className="h-8 w-20 leading-8"
              buttonClassName="h-full border-main rounded-none inline-flex items-center"
              options={sortByItems}
              value={xAxis?.sort?.sortBy as string}
              onChange={(value: string) => {
                setXAxis({
                  ...xAxis,
                  sort: { ...xAxis?.sort, sortBy: value as SortByLike }
                })
              }}
              placeholder="Sort By"
            />
            <OrderSelect
              className="h-8 w-40 leading-8"
              buttonClassName="h-full border-l-0 border-main rounded-l-none inline-flex items-center"
              options={orderItems}
              value={xAxis?.sort?.orderDesc}
              onChange={(value) => {
                setXAxis({
                  ...xAxis,
                  sort: { ...xAxis?.sort, orderDesc: value }
                })
              }}
              placeholder="Sort Order"
            />
          </span>
        )}
        <Button
          type="button"
          role="link"
          onClick={onClickResetXAxis}
          className="h-8"
        >
          Reset
        </Button>
      </div>
    </DisclosurePanel>
  )
}
