import { useCallback, useEffect, useState } from 'react'
import { Switch as HeadlessSwitch } from '@headlessui/react'
import { cva } from 'class-variance-authority'
import { isFunction } from 'lodash'

const switchClass = cva(
  [
    'relative inline-flex shrink-0 rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-2  focus-visible:ring-white focus-visible:ring-opacity-75'
  ],
  {
    variants: {
      size: {
        lg: 'h-8 w-[68px]',
        default: 'h-6 w-[52px]',
        sm: 'h-[16px] w-[30px]'
      },
      enabled: {
        true: 'bg-primary group-hover:bg-primary-500',
        false: 'bg-gray-400/50 group-hover:bg-primary-200'
      },
      disabled: {
        true: 'cursor-not-allowed opacity-50',
        false: 'cursor-pointer'
      }
    },
    defaultVariants: {
      size: 'default',
      enabled: false
    },
    compoundVariants: [
      {
        enabled: false,
        disabled: true,
        class: '!bg-gray-400/50'
      }
    ]
  }
)

const dotClass = cva(
  'pointer-events-none inline-block transform rounded-full bg-white dark:bg-sentio-gray-100 shadow-lg ring-0 transition duration-200 ease-in-out',
  {
    variants: {
      size: {
        lg: 'h-7 w-7',
        default: 'h-5 w-5',
        sm: 'h-3 w-3'
      },
      enabled: {
        true: '',
        false: 'translate-x-0'
      }
    },
    defaultVariants: {
      size: 'default'
    },
    compoundVariants: [
      {
        size: 'sm',
        enabled: true,
        class: 'translate-x-3.5 switch-dot-sm'
      },
      {
        size: 'default',
        enabled: true,
        class: 'translate-x-7'
      },
      {
        size: 'lg',
        enabled: true,
        class: 'translate-x-9'
      }
    ]
  }
)

const labelClass = cva(
  'text-text-foreground ml-2 font-medium align-text-bottom',
  {
    variants: {
      size: {
        lg: 'text-sm leading-8',
        default: 'text-icontent leading-6 ',
        sm: 'text-icontent leading-5'
      },
      disabled: {
        true: 'cursor-not-allowed opacity-50',
        false: 'cursor-pointer'
      }
    },
    defaultVariants: {
      size: 'default',
      disabled: false
    }
  }
)

export interface SwitchProps {
  checked?: boolean
  onChange?: (checked: boolean) => void
  srText?: string
  size?: 'lg' | 'default' | 'sm'
  disabled?: boolean
  label?: string
}

export function Switch({
  checked,
  onChange: _onChange,
  srText,
  size = 'default',
  disabled,
  label
}: SwitchProps) {
  const [enabled, setState] = useState(checked)

  const onChange = useCallback(() => {
    setState((enabled) => {
      if (isFunction(_onChange)) {
        setTimeout(() => {
          _onChange(!enabled)
        })
      }
      return !enabled
    })
  }, [_onChange])

  useEffect(() => {
    setState(checked)
  }, [checked])

  return (
    <HeadlessSwitch.Group>
      <HeadlessSwitch
        checked={enabled}
        onChange={onChange || setState}
        className={switchClass({
          enabled,
          size,
          disabled
        })}
        disabled={disabled}
      >
        {srText && <span className="sr-only">{srText}</span>}
        <span
          aria-hidden="true"
          className={dotClass({
            enabled,
            size
          })}
        />
      </HeadlessSwitch>
      {label && (
        <HeadlessSwitch.Label
          className={labelClass({
            size,
            disabled
          })}
        >
          {label}
        </HeadlessSwitch.Label>
      )}
    </HeadlessSwitch.Group>
  )
}
