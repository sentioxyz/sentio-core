import { defaults } from 'lodash'
import type { ReactNode } from 'react'
import { Button, Checkbox, DisclosurePanel } from '@sentio/ui-core'
import { AddonLabel } from './controls-ui'
import type { YAxisConfigLike } from '../../types'

interface Props {
  yAxis?: YAxisConfigLike
  setYAxis: (yAxis: YAxisConfigLike) => void
  defaultOpen?: boolean
  /** Optional app-supplied column picker (sql charts). When present the Column row renders. */
  columnSelect?: ReactNode
  supportSetName?: boolean
  supportSetMinMax?: boolean
  supportStackSeries?: boolean
  supportAlwaysShowZero?: boolean
  supportReset?: boolean
  panelTitle?: string
}

const initialConfig = {
  yAxis: {
    min: '',
    max: '',
    scale: true,
    stack: ''
  }
}

export default function YaxisControls({
  yAxis,
  setYAxis,
  defaultOpen,
  columnSelect,
  supportSetName = true,
  supportSetMinMax = true,
  supportStackSeries = true,
  supportAlwaysShowZero = true,
  supportReset = true,
  panelTitle = 'Y-Axis Controls'
}: Props) {
  yAxis = defaults(yAxis || {}, initialConfig.yAxis)
  const onChangeInput =
    (field: string) => (event: React.ChangeEvent<HTMLInputElement>) => {
      const { value } = event.target
      setYAxis({
        ...yAxis,
        [field]: value || undefined,
        scale: field == 'min' && value > '0' ? true : yAxis?.scale
      })
    }
  const onToggleZero = (checked: boolean) => {
    setYAxis({ ...yAxis, scale: !checked, min: checked ? '' : yAxis?.min })
  }
  const onClickResetYAxis = () => {
    setYAxis(initialConfig.yAxis)
  }

  const onToggleStack = (checked: boolean) => {
    setYAxis({ ...yAxis, stacked: checked ? 'samesign' : '' })
  }

  const minMaxLabelCls =
    'inline-flex items-center border border-r-0 sm:text-icontent border-main  bg-gray-50 px-2 rounded-l-md'
  const minMaxInputCls =
    'border focus:border-primary-500 rounded-r-md sm:text-icontent border-main hover:border-primary-600 focus:ring-3 focus:ring-primary-600/30'

  return (
    <DisclosurePanel
      title={panelTitle}
      defaultOpen={defaultOpen}
      containerClassName="w-full bg-default-bg"
    >
      <div className="text-icontent flex flex-wrap gap-[10px]">
        {supportSetName && (
          <label className="inline-flex h-8">
            <AddonLabel className="rounded-l-md border border-r-0 px-2">
              Name
            </AddonLabel>
            <input
              type="text"
              className="focus:border-primary-500 sm:text-icontent border-main hover:border-primary-600 focus:ring-3 focus:ring-primary-600/30 w-40 rounded-r-md border"
              value={yAxis?.name}
              placeholder="Axis Name"
              onChange={onChangeInput('name')}
            />
          </label>
        )}
        {columnSelect && (
          <span className="inline-flex h-8">
            <AddonLabel className="rounded-l-md border border-r-0 px-2">
              Column
            </AddonLabel>
            {columnSelect}
          </span>
        )}
        {supportSetMinMax && (
          <>
            <label className="inline-flex h-8">
              <span className={minMaxLabelCls}>Min</span>
              <input
                type="text"
                className={minMaxInputCls}
                style={{ width: '10em' }}
                value={yAxis.min}
                placeholder="Auto"
                onChange={onChangeInput('min')}
              />
            </label>
            <label className="inline-flex h-8">
              <span className={minMaxLabelCls}>Max</span>
              <input
                type="text"
                className={minMaxInputCls}
                style={{ width: '10em' }}
                value={yAxis.max}
                placeholder="Auto"
                onChange={onChangeInput('max')}
              />
            </label>
          </>
        )}

        {supportStackSeries && (
          <Checkbox
            checked={!!yAxis?.stacked}
            onChange={onToggleStack}
            label="Stack series"
          />
        )}

        {supportAlwaysShowZero && (
          <Checkbox
            checked={!yAxis.scale}
            onChange={onToggleZero}
            label="Always show zero"
          />
        )}

        {supportReset && (
          <Button type="button" role="link" onClick={onClickResetYAxis}>
            Reset
          </Button>
        )}
      </div>
    </DisclosurePanel>
  )
}
