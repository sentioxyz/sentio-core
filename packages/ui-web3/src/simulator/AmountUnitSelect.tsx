import { AmountUnit } from './types'
import { BigDecimal } from '@sentio/bigdecimal'

interface Props {
  value: AmountUnit
  onChange: (value: AmountUnit) => void
  className?: string
  nativeToken?: string
}

export const AmountUnitSelect = ({
  value,
  onChange,
  className,
  nativeToken
}: Props) => {
  return (
    <select
      className={`rounded-md border border-gray-300 px-3 py-2 text-sm ${className || ''}`}
      value={value}
      onChange={(e) => onChange(e.target.value as AmountUnit)}
    >
      <option value={AmountUnit.Wei}>Wei</option>
      <option value={AmountUnit.Gwei}>Gwei</option>
      <option value={AmountUnit.Ether}>{nativeToken || 'Ether'}</option>
    </select>
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
  return new BigDecimal(1)
}

export function getWeiAmount(value: string, unit: AmountUnit): BigDecimal {
  return new BigDecimal(value).multipliedBy(
    genCoefficient(unit, AmountUnit.Wei)
  )
}
