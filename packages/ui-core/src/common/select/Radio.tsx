import { RadioGroup } from '@headlessui/react'
import { classNames } from '../../utils/classnames'

interface Props<T> {
  value?: T
  onChange?: (v: T) => void
  label?: string
  labelClassName?: string
  containerClassName?: string
  options: {
    name: string
    value: T
  }[]
  vertical?: boolean
}

export function RadioSelect<T>({
  value,
  onChange,
  label,
  labelClassName,
  options,
  vertical,
  containerClassName
}: Props<T>) {
  return (
    <RadioGroup value={value} onChange={onChange}>
      {label && (
        <RadioGroup.Label className="text-ilabel text-text-foreground mr-4 font-medium">
          {label}:
        </RadioGroup.Label>
      )}
      <div
        className={classNames(
          'item-center',
          vertical ? 'flex flex-col gap-2' : 'inline-flex gap-4',
          containerClassName
        )}
      >
        {options.map(({ name, value }, index) => (
          <RadioGroup.Option key={index} value={value}>
            {({ checked }) => (
              <span className="group/radio">
                <input
                  readOnly
                  type="radio"
                  checked={checked}
                  className="border-sentio-gray-300 group-hover/radio:border-primary-500"
                />
                <label
                  className={classNames(
                    'text-ilabel group-hover/radio:text-primary-500 group-hover/radio:dark:text-primary-600  ml-2 font-medium ',
                    checked
                      ? 'text-primary dark:text-primary-700'
                      : 'text-gray',
                    labelClassName
                  )}
                >
                  {name}
                </label>
              </span>
            )}
          </RadioGroup.Option>
        ))}
      </div>
    </RadioGroup>
  )
}
