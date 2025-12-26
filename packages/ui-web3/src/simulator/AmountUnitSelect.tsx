import { Select, type SelectProps } from '@sentio/ui-core'
import { AmountUnit } from './types'
import BigDecimal from '@sentio/bigdecimal'

interface Props {
  value: AmountUnit
  onChange: (value: AmountUnit) => void
  className?: SelectProps<AmountUnit>['className']
  buttonClassName?: SelectProps<AmountUnit>['buttonClassName']
  nativeToken?: string
}

export const AmountUnitSelect = ({
  value,
  onChange,
  className,
  buttonClassName,
  nativeToken
}: Props) => {
  return (
    <Select
      size="md"
      buttonClassName={buttonClassName}
      className={className}
      value={value}
      onChange={onChange}
      options={[
        {
          label: 'Wei',
          value: AmountUnit.Wei
        },
        {
          label: 'Gwei',
          value: AmountUnit.Gwei
        },
        {
          label: nativeToken ?? 'Ether',
          value: AmountUnit.Ether
        }
      ]}
    />
  )
}

export function genCoefficient(
  prev: AmountUnit,
  current: AmountUnit
): BigDecimal {
  switch (prev) {
    case AmountUnit.Wei:
      switch (current) {
        case AmountUnit.Wei:
          return new BigDecimal(1)
        case AmountUnit.Gwei:
          return new BigDecimal(1e-9)
        case AmountUnit.Ether:
          return new BigDecimal(1e-18)
      }
      break
    case AmountUnit.Gwei:
      switch (current) {
        case AmountUnit.Wei:
          return new BigDecimal(1e9)
        case AmountUnit.Gwei:
          return new BigDecimal(1)
        case AmountUnit.Ether:
          return new BigDecimal(1e-9)
      }
      break
    case AmountUnit.Ether:
      switch (current) {
        case AmountUnit.Wei:
          return new BigDecimal(1e18)
        case AmountUnit.Gwei:
          return new BigDecimal(1e9)
        case AmountUnit.Ether:
          return new BigDecimal(1)
      }
  }
}

export function getWeiAmount(value: string, unit: AmountUnit): BigDecimal {
  return BigDecimal(value).multipliedBy(genCoefficient(unit, AmountUnit.Wei))
}
