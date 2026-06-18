import { defaults } from 'lodash'
import { Checkbox, DisclosurePanel } from '@sentio/ui-core'
import { produce } from 'immer'
import {
  ValueFormatters,
  ValueOptions,
  type ValueFormatter
} from './ValueOptions'
import type { ValueConfigLike } from '../../types'

interface Props {
  config?: ValueConfigLike
  defaultOpen?: boolean
  onChange: (config: ValueConfigLike) => void
  formatters?: ValueFormatter[]
  showPrefix?: boolean
  showSuffix?: boolean
}

export const defaultConfig: ValueConfigLike = {
  valueFormatter: 'NumberFormatter',
  showValueLabel: false,
  maxSignificantDigits: 3,
  dateFormat: 'LLL',
  mappingRules: [],
  style: 'None',
  maxFractionDigits: 2
}

export const ValueControls = ({
  config,
  defaultOpen,
  onChange,
  formatters = ValueFormatters,
  showPrefix,
  showSuffix
}: Props) => {
  config = defaults(config || {}, defaultConfig)
  function toggleShowValueLabel(checked: boolean) {
    config &&
      onChange(
        produce(config, (draft) => void (draft.showValueLabel = checked))
      )
  }
  function toggleTooltipTotal(checked: boolean) {
    config &&
      onChange(produce(config, (draft) => void (draft.tooltipTotal = checked)))
  }
  return (
    <DisclosurePanel
      title="Value Options"
      defaultOpen={defaultOpen}
      containerClassName="w-full bg-default-bg"
    >
      <ValueOptions
        onChange={onChange}
        config={config}
        formatters={formatters}
        showPrefix={showPrefix}
        showSuffix={showSuffix}
      />

      <div className="mt-2 flex items-center gap-2">
        <Checkbox
          checked={config?.showValueLabel}
          onChange={toggleShowValueLabel}
          label="Show value label"
        />
        <Checkbox
          checked={config?.tooltipTotal}
          onChange={toggleTooltipTotal}
          label="Show total in tooltip"
        />
      </div>
    </DisclosurePanel>
  )
}
