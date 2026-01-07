import { classNames } from '../utils/classnames'
import { ReactNode } from 'react'

interface Props {
  checked?: boolean
  onChange?: (v: boolean) => void
  label?: string
  labelNode?: ReactNode
  id?: string
  name?: string
  disabled?: boolean
  inputProps?: any // extra props for input
  labelClassName?: string
  className?: string
}

export const Checkbox = ({
  checked,
  onChange,
  label,
  labelNode,
  id,
  name,
  inputProps,
  disabled,
  labelClassName,
  className
}: Props) => {
  return (
    <div
      className={classNames('inline-flex items-center gap-2', className)}
      onClick={(e) => {
        e.stopPropagation()
        onChange?.(!checked)
      }}
    >
      <input
        id={id}
        name={name}
        type="checkbox"
        className="text-primary-600 focus:ring-primary-500 h-4 w-4 rounded border-gray-300 "
        disabled={disabled}
        checked={checked}
        readOnly
        {...inputProps}
      />
      {label && (
        <span
          className={classNames(
            'text-ilabel text-gray font-medium',
            labelClassName
          )}
        >
          {label}
        </span>
      )}
      {labelNode}
    </div>
  )
}
