import { classNames, mergeClasses } from '../utils/classnames'
import { ReactNode, useId } from 'react'

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
  inputClassName?: string
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
  className,
  inputClassName
}: Props) => {
  // Fall back to an auto-generated id so htmlFor always has a target,
  // even when the caller doesn't provide one.
  const reactId = useId()
  const inputId = id ?? reactId

  return (
    <div
      className={classNames('group inline-flex items-center gap-2', className)}
      // Stop the click from bubbling to ancestor handlers, but don't toggle
      // here — the input's native onChange (also triggered by the linked
      // <label>) is the single source of truth.
      onClick={(e) => e.stopPropagation()}
    >
      <input
        id={inputId}
        name={name}
        type="checkbox"
        className={mergeClasses(
          'focus:ring-primary-500 h-3.5 w-3.5 shrink-0 rounded-[3px] border border-gray-800 align-middle',
          disabled ? 'opacity-50' : 'hover:border-primary-600 cursor-pointer checked:border-primary-600',
          inputClassName
        )}
        disabled={disabled}
        checked={checked}
        onChange={(e) => onChange?.(e.target.checked)}
        {...inputProps}
      />
      {label && (
        <label
          htmlFor={inputId}
          className={classNames(
            'text-ilabel text-text-foreground-secondary select-none',
            disabled ? 'cursor-not-allowed' : 'cursor-pointer hover:text-primary-600',
            labelClassName
          )}
        >
          {label}
        </label>
      )}
      {labelNode ? <label htmlFor={inputId}>{labelNode}</label> : null}
    </div>
  )
}
