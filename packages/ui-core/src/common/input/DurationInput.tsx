import { useMemo } from 'react'
import { LuTriangleAlert } from 'react-icons/lu'
import { classNames } from '../../utils/classnames'
import { PopoverTooltip } from '../DivTooltip'

// Minimal structural mirror of the proto `Duration` message — ui-core stays
// proto-free, callers pass their generated `Duration` (structurally compatible).
export interface DurationLike {
  value?: number | 'NaN' | 'Infinity' | '-Infinity'
  unit?: string
}

interface Props {
  value?: DurationLike
  onChange: (value: DurationLike) => void
  disabled?: boolean
  className?: string
  inputClassName?: string
  enableDays?: boolean
  optionDisabled?: (unit: string) => string | undefined
  optionHint?: (unit: string) => string | undefined
}

export function DurationInput({
  className,
  inputClassName,
  disabled,
  value,
  onChange,
  enableDays,
  optionDisabled,
  optionHint
}: Props) {
  function setDuration(value?: DurationLike['value'], unit?: string) {
    onChange({ value, unit })
  }

  const options = useMemo(() => {
    let options: { value: string; label: string; disabled?: string }[] = [
      { value: 's', label: 'seconds' },
      { value: 'm', label: 'minutes' },
      { value: 'h', label: 'hours' }
    ]
    if (enableDays) {
      options = [
        ...options,
        { value: 'd', label: 'days' },
        { value: 'w', label: 'weeks' }
      ]
    }
    options.forEach((o) => {
      o.disabled = optionDisabled && optionDisabled(o.value)
    })

    return options
  }, [enableDays, optionDisabled])

  const hint = optionHint && optionHint(value?.unit || '')

  return (
    <div
      className={classNames(
        className,
        'border-main hover:border-primary-600 focus-within:border-primary focus-within:ring-3 focus-within:ring-primary-600/30 flex w-fit items-center gap-2 rounded-md border'
      )}
    >
      <input
        type="number"
        name="duration"
        min={0}
        step={1}
        disabled={disabled}
        className={classNames(
          'border-0 focus:border-0 focus:ring-0',
          inputClassName
        )}
        value={disabled ? '' : value?.value}
        onChange={(e) => setDuration(parseInt(e.target.value), value?.unit)}
      />
      <div className="inline-flex items-center">
        <label htmlFor="unit" className="sr-only">
          unit
        </label>
        <select
          id="unit"
          name="unit"
          className="text-text-foreground text-icontent h-full border-transparent bg-transparent py-0 pl-2 focus:border-transparent focus:ring-0"
          value={value?.unit}
          disabled={disabled}
          onChange={(e) => value && setDuration(value.value!, e.target.value)}
        >
          {options.map((o) => {
            return (
              <option
                key={o.value}
                value={o.value}
                disabled={!!o.disabled}
                title={o.disabled}
              >
                {o.label}
              </option>
            )
          })}
        </select>
        {hint && (
          <PopoverTooltip text={hint} buttonClassName="mr-2">
            <LuTriangleAlert className="h-4 w-4" />
          </PopoverTooltip>
        )}
      </div>
    </div>
  )
}
