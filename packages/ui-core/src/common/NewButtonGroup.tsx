import { ReactElement } from 'react'
import { classNames } from '../utils/classnames'

interface ButtonItem<T> {
  label: string | ReactElement
  value: T
  icon?: React.ReactNode
  disabled?: boolean
  disabledHint?: string
}

interface Props<T> {
  buttons: ButtonItem<T>[]
  value?: T
  onChange?: (key: T) => void
  small?: boolean
  flexGrow?: boolean
  theme?: 'light' | 'default'
  buttonClassName?: string
  hideLabel?: boolean
}

const ThemeColor = {
  light:
    'text-primary bg-primary/10 border-primary hover:bg-primary/30 active:bg-primary/20',
  default:
    'text-white bg-primary border-primary hover:bg-primary-500 active:bg-primary-700'
}

export default function ButtonGroup<T>({
  buttons,
  value,
  onChange,
  small,
  flexGrow,
  theme = 'default',
  buttonClassName,
  hideLabel
}: Props<T>) {
  const selectedIndex = buttons.findIndex((button) => button.value === value)
  return (
    <span
      className={classNames(
        'relative z-0 rounded-md',
        flexGrow ? 'flex w-full' : 'inline-flex'
      )}
    >
      {buttons.map((button, idx) => (
        <button
          key={idx}
          type="button"
          onClick={() => !button.disabled && onChange && onChange(button.value)}
          className={classNames(
            'border-border-color relative inline-flex items-center border' +
              ' focus:ring-primary-500 focus:border-primary-500 focus:outline-hidden focus:z-10 focus:ring-1',
            idx === selectedIndex
              ? ThemeColor[theme]
              : 'text-text-foreground hover:bg-hover bg-default-bg',
            idx == 0
              ? 'rounded-l-md'
              : idx == buttons.length - 1
                ? 'rounded-r-md'
                : '',
            idx !== 0 && value !== button.value ? 'border-l-transparent' : '',
            selectedIndex - 1 === idx ? 'border-r-transparent' : '',
            small ? 'text-ilabel px-2.5 py-1.5' : 'px-4 py-1.5 text-sm',
            flexGrow ? 'basis-full justify-center whitespace-nowrap' : '',
            buttonClassName,
            button.disabled ? 'cursor-not-allowed opacity-50' : ''
          )}
          aria-pressed={idx === selectedIndex}
          title={button.disabled ? button.disabledHint : ''}
        >
          {button?.icon ?? null}
          {!hideLabel && button.label}
        </button>
      ))}
    </span>
  )
}
