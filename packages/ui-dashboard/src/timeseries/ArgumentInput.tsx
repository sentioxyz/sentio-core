import { DurationInput, classNames } from '@sentio/ui-core'
import { ArgumentDef, ArgumentType } from './functions'
import type { ArgumentLike } from '../types/metrics'

interface Props {
  argument: ArgumentDef
  value?: ArgumentLike
  onChange?: (value: ArgumentLike) => void
  className?: string
}

export function ArgumentInput({ className, argument, value, onChange }: Props) {
  switch (argument.type) {
    case ArgumentType.String:
      return (
        <input
          type="text"
          className={className}
          value={value?.stringValue}
          onChange={(v) =>
            onChange && onChange({ stringValue: v.target.value })
          }
        />
      )
    case ArgumentType.Double:
      return (
        <input
          type="number"
          className={className}
          value={value?.doubleValue}
          step="any"
          onChange={(v) =>
            onChange && onChange({ doubleValue: parseFloat(v.target.value) })
          }
        />
      )
    case ArgumentType.Integer:
      return (
        <input
          step="1"
          type="number"
          className={className}
          value={value?.intValue}
          onChange={(v) =>
            onChange && onChange({ intValue: parseInt(v.target.value) })
          }
        />
      )
    case ArgumentType.Bool:
      return (
        <input
          type="checkbox"
          className={className}
          checked={value?.boolValue}
          onChange={(e) =>
            onChange && onChange({ boolValue: e.target.value == 'true' })
          }
        />
      )
    case ArgumentType.Duration:
      return (
        <DurationInput
          className="rounded-none! border-transparent! hover:border-primary-600! focus-within:border-primary-600!"
          inputClassName={classNames(className)}
          value={value?.durationValue}
          onChange={(e) => onChange && onChange({ durationValue: e })}
          enableDays
        />
      )
  }
}
