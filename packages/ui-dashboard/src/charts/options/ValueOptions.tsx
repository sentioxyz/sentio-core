import { produce } from 'immer'
import { ComboInput, classNames } from '@sentio/ui-core'
import { ValueStringMapping } from './ValueStringMapping'
import type {
  MappingRuleLike,
  ValueConfigLike,
  ValueFormatterLike,
  ValueStyleLike
} from '../../types'

export interface ValueFormatter {
  label: string
  value: ValueFormatterLike
}

export const ValueFormatters: ValueFormatter[] = [
  { label: 'Number', value: 'NumberFormatter' },
  { label: 'Date', value: 'DateFormatter' },
  { label: 'String', value: 'StringFormatter' }
]

interface Props {
  config: ValueConfigLike
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
  style: 'None'
}

const dateFormats = [
  { label: 'Localized format', value: 'LLL' },
  { label: 'ISO String', value: 'YYYY-MM-DDTHH:mm:ss.sssZ' }
]

const CurrencySymbols = [
  { label: 'USD', value: '$' },
  { label: 'EUR', value: '€' },
  { label: 'GBP', value: '£' },
  { label: 'CNY or JPY', value: '¥' },
  { label: 'BTC', value: 'Ƀ' },
  { label: 'ETH', value: 'Ξ' }
]

// Inline addon label sitting flush against a select/input. `className` carries
// the per-use border-side / rounded variant.
const AddonLabel = ({
  className,
  children
}: {
  className?: string
  children: React.ReactNode
}) => (
  <span
    className={classNames(
      'sm:text-ilabel border-main inline-flex items-center whitespace-nowrap bg-gray-50 px-3',
      className
    )}
  >
    {children}
  </span>
)

export const ValueOptions = ({
  config,
  onChange,
  formatters = ValueFormatters,
  showPrefix,
  showSuffix
}: Props) => {
  function onChangeDateFormat(f: string) {
    onChange(produce(config, (draft) => void (draft.dateFormat = f)))
  }
  function onChangeFormatter(f: ValueFormatterLike) {
    onChange(produce(config, (draft) => void (draft.valueFormatter = f)))
  }
  function onChangeSymbol(symbol?: string) {
    onChange(produce(config, (draft) => void (draft.currencySymbol = symbol)))
  }
  function onStyleChange(notation: ValueStyleLike) {
    onChange(
      produce(config, (draft) => {
        draft.style = notation
      })
    )
  }
  function onDigitsChange(value: string, option: string) {
    onChange(
      produce(config, (draft) => {
        const d = draft as Record<string, any>
        if (value) {
          const maxSignificantDigits = parseInt(value)
          if (maxSignificantDigits >= 0 && maxSignificantDigits <= 20) {
            d[option] = maxSignificantDigits
          }
        } else {
          delete d[option]
        }
      })
    )
  }

  function onMappingRulesChange(rules: MappingRuleLike[]) {
    onChange(produce(config, (draft) => void (draft.mappingRules = rules)))
  }

  function onPrefixChange(value: string) {
    onChange(
      produce(config, (draft) => {
        if (value) {
          draft.prefix = value
        } else {
          delete draft.prefix
        }
      })
    )
  }

  function onSuffixChange(value: string) {
    onChange(
      produce(config, (draft) => {
        if (value) {
          draft.suffix = value
        } else {
          delete draft.suffix
        }
      })
    )
  }

  function numberAddons(style: ValueStyleLike) {
    switch (style) {
      case 'None':
        return (
          <>
            <AddonLabel className="border border-l-0">
              Fraction Digits
            </AddonLabel>
            <input
              disabled
              className="focus:border-primary-500 sm:text-ilabel min-w-20  border-main rounded-r-md border border-l-0  py-1"
              value={''}
            />
          </>
        )
      case 'Percent':
      case 'Standard':
        return (
          <>
            <AddonLabel className="border border-x-0">
              Fraction Digits
            </AddonLabel>
            <input
              type="number"
              min={0}
              max={20}
              className="focus:border-primary-500 sm:text-ilabel min-w-20 border-main focus:ring-3 focus:ring-primary-600/30 hover:border-primary-600 rounded-r-md border py-1"
              value={config.maxFractionDigits}
              placeholder={'0-20'}
              onChange={(e) =>
                onDigitsChange(e.target.value, 'maxFractionDigits')
              }
            />
          </>
        )
      case 'Currency':
        return (
          <>
            <AddonLabel className="border border-r-0">Symbol</AddonLabel>
            <div className="w-28 ">
              <ComboInput
                onChange={onChangeSymbol}
                options={CurrencySymbols.map((s) => s.value)}
                displayFn={(s) => {
                  const name = CurrencySymbols.find((c) => c.value === s)?.label
                  return `${name} (${s})`
                }}
                placeholder={'$'}
                value={config?.currencySymbol}
              />
            </div>
            <AddonLabel className="border">Precision</AddonLabel>
            <input
              type="number"
              min={0}
              max={20}
              className="focus:border-primary-500 sm:text-ilabel min-w-20  border-main rounded-r-md border border-l-0  py-1"
              value={config.precision}
              defaultValue={2}
              placeholder={'0-20'}
              onChange={(e) => onDigitsChange(e.target.value, 'precision')}
            />
          </>
        )
      default:
        return (
          <>
            <AddonLabel className="border border-x-0">
              Max significant digits
            </AddonLabel>
            <input
              type="number"
              min={1}
              max={21}
              className="focus:border-primary-600 sm:text-ilabel min-w-20 border-main hover:border-primary-600 focus:ring-3 focus:ring-primary-600/30 rounded-r-md border py-1"
              value={config.maxSignificantDigits}
              placeholder={'1-21'}
              onChange={(e) =>
                onDigitsChange(e.target.value, 'maxSignificantDigits')
              }
            />
          </>
        )
    }
  }

  return (
    <>
      <div>
        <div className="flex">
          <AddonLabel className="rounded-l-md border border-r-0">
            Value formatter
          </AddonLabel>
          <select
            value={config.valueFormatter}
            className={classNames(
              'sm:text-ilabel border-main text-text-foreground hover:border-primary-600 inline-flex items-center border py-1.5 pl-4 pr-7 focus:ring-0',
              config.valueFormatter == 'StringFormatter' ? 'rounded-r-md' : ''
            )}
            onChange={(e) =>
              onChangeFormatter(e.target.value as ValueFormatterLike)
            }
          >
            {formatters.map((d) => {
              return (
                <option key={d.value} value={d.value}>
                  {d.label}
                </option>
              )
            })}
          </select>
          {config.valueFormatter == 'NumberFormatter' && (
            <>
              <AddonLabel className="border border-l-0 border-r-0">
                Style
              </AddonLabel>
              <select
                value={config.style}
                className="sm:text-ilabel border-main text-text-foreground hover:border-primary-600 inline-flex items-center border py-1 pl-4 pr-7 focus:ring-0"
                onChange={(e) =>
                  onStyleChange(e.target.value as ValueStyleLike)
                }
              >
                <option value={'None'}>None</option>
                <option value={'Compact'}>Compact</option>
                <option value={'Standard'}>Standard</option>
                <option value={'Scientific'}>Scientific</option>
                <option value={'Percent'}>Percent</option>
                <option value={'Currency'}>Currency</option>
              </select>
              {config.style && numberAddons(config.style)}
            </>
          )}
          {config.valueFormatter == 'DateFormatter' && (
            <>
              <AddonLabel className="border border-l-0">Date format</AddonLabel>
              <select
                value={config.dateFormat}
                className="sm:text-ilabel border-main text-text-foreground inline-flex items-center rounded-r-md border  border-l-0  py-1  pl-4 pr-7"
                onChange={(e) => onChangeDateFormat(e.target.value)}
              >
                {dateFormats.map((d) => {
                  return (
                    <option key={d.value} value={d.value}>
                      {d.label}
                    </option>
                  )
                })}
              </select>
            </>
          )}
        </div>
      </div>

      {/* Prefix and Suffix Configuration */}
      {(showPrefix || showSuffix) && (
        <div>
          <div className="mt-2 flex items-center gap-4">
            {showPrefix && (
              <div className="border-main hover:border-primary-600 focus-within:border-primary-600 focus-within:ring-3 focus-within:ring-primary-500/30 text-icontent inline-flex items-center rounded-md border">
                <div className="h-7.5 leading-7.5 border-r px-3">Prefix</div>
                <input
                  type="text"
                  className="border-0 px-3 py-1.5 focus:ring-0"
                  value={config.prefix || ''}
                  placeholder="e.g., $, #"
                  onChange={(e) => onPrefixChange(e.target.value)}
                />
              </div>
            )}
            {showSuffix && (
              <div className="border-main hover:border-primary-600 focus-within:border-primary-600 focus-within:ring-3 focus-within:ring-primary-500/30 text-icontent inline-flex items-center rounded-md border">
                <div className="h-7.5 leading-7.5 border-r px-3">Suffix</div>
                <input
                  type="text"
                  className="min-w-32 border-0 px-3 py-1.5 focus:ring-0"
                  value={config.suffix || ''}
                  placeholder="e.g., %, USD, tokens"
                  onChange={(e) => onSuffixChange(e.target.value)}
                />
              </div>
            )}
          </div>
        </div>
      )}

      {config.valueFormatter == 'StringFormatter' && (
        <ValueStringMapping
          rules={config.mappingRules || []}
          onChange={onMappingRulesChange}
        />
      )}
    </>
  )
}
