import {
  cloneElement,
  forwardRef,
  MutableRefObject,
  ReactNode,
  useMemo
} from 'react'
import { PopoverTooltip } from './DivTooltip'
import { cva, cx, VariantProps } from 'class-variance-authority'

export const buttonClass = cva(['inline-flex', 'items-center', 'font-medium'], {
  variants: {
    role: {
      primary: ['btn-primary'],
      secondary: ['btn-secondary'],
      dashed: ['btn-dashed'],
      text: ['btn-text'],
      link: ['btn-link'],
      tertiary: ['btn-tertiary'],
      tertiarytext: ['btn-tertiarytext'],
      custom: [] // custom button
    },
    status: {
      default: 'btn-status-default',
      danger: 'btn-status-danger'
    },
    size: {
      sm: ['px-2', 'py-1.5', 'text-xs', 'font-normal', 'gap-2'],
      default: ['px-2.5', 'text-ilabel', 'font-ilabel', 'gap-2', 'py-1'],
      md: ['px-2.5 text-ilabel font-ilabel gap-2', 'py-1.5'],
      lg: ['px-3 text-sm gap-3', 'py-2']
    },
    disabled: {
      false: [''],
      true: ['btn-disabled']
    },
    position: {
      begin: ['rounded-l-md'],
      end: ['rounded-r-md'],
      middle: [''],
      full: ['rounded-md']
    }
  },
  compoundVariants: [
    {
      role: 'secondary',
      size: 'default',
      class: 'py-[3px]'
    },
    {
      role: 'dashed',
      size: 'default',
      class: 'py-[3px]'
    },
    {
      role: 'secondary',
      size: 'md',
      class: 'py-[5px]'
    },
    {
      role: 'dashed',
      size: 'md',
      class: 'py-[5px]'
    },
    {
      role: 'secondary',
      size: 'lg',
      class: 'py-[7px]'
    },
    {
      role: 'dashed',
      size: 'lg',
      class: 'py-[7px]'
    }
  ],
  defaultVariants: {
    role: 'secondary',
    status: 'default',
    size: 'default',
    position: 'full',
    disabled: false
  }
})

export interface ButtonProps extends VariantProps<typeof buttonClass> {
  className?: string
  // role?: 'primary' | 'secondary' | 'dashed' | 'text' | 'link'
  // status?: 'default' | 'danger'
  onClick?: (evt: React.MouseEvent) => void
  children?: ReactNode
  // size?: 'md' | 'lg'
  type?: 'submit' | 'button' | 'reset'
  processing?: boolean
  // disabled?: boolean
  disabledHint?: React.ReactNode
  disabledHintPortal?: boolean
  // position?: 'begin' | 'end' | 'middle' | 'full'
  ref?: MutableRefObject<any>
  icon?: React.ReactElement
  title?: string
  value?: string
  id?: string
}

export function Proccessing({
  className,
  light
}: {
  className?: string
  light?: boolean
}) {
  return (
    <svg className={`h-5 w-5 animate-spin ${className}`} viewBox="0 0 24 24">
      <circle
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="4"
        style={{
          opacity: light ? 0.05 : 0.1
        }}
      ></circle>
      <path
        style={{
          opacity: light ? 0.5 : 0.75
        }}
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
      ></path>
    </svg>
  )
}

const iconClass = cva('', {
  variants: {
    size: {
      default: 'w-4 h-4',
      md: 'w-[18px] h-[18px]',
      lg: 'w-5 h-4',
      sm: 'w-4 h-4'
    },
    disabled: {
      true: 'saturate-0',
      false: ''
    }
  },
  defaultVariants: {
    size: 'default',
    disabled: false
  }
})

const pIconClass = cva('', {
  variants: {
    size: {
      default: 'w-4! h-4!',
      md: 'w-[18px]! h-[18px]!',
      lg: 'w-5! h-5!',
      sm: 'w-4! h-4!'
    }
  },
  defaultVariants: {
    size: 'default'
  }
})

function Button(
  {
    className,
    size,
    type,
    role,
    status,
    onClick,
    children,
    processing,
    disabled,
    disabledHint,
    disabledHintPortal,
    position,
    icon,
    title,
    value,
    id
  }: ButtonProps,
  ref: any
) {
  const iconClasses = iconClass({ size: size, disabled: disabled })

  const iconEl = useMemo(() => {
    let iconEl: ReactNode = null

    if (processing) {
      iconEl = (
        <Proccessing
          className={cx(
            pIconClass({ size: size }),
            role == 'primary' ? 'text-white' : ''
          )}
          light={role !== 'primary'}
        />
      )
    } else if (icon) {
      iconEl = cloneElement(icon, {
        className: cx(icon.props.className, iconClasses)
      })
    }
    return iconEl
  }, [icon, iconClasses, processing, role])

  // const cls = classNames(className, sizeClasses, typeClasses, shapeClasses)
  const cls = cx(
    className,
    buttonClass({
      size: size,
      status: status,
      role: role,
      disabled: disabled,
      position: position
    })
  )

  const btn = (
    <button
      title={title}
      onClick={onClick}
      type={type}
      disabled={disabled || processing}
      className={cls}
      ref={ref}
      value={value}
      suppressHydrationWarning
      id={id}
    >
      {iconEl}
      {children}
    </button>
  )

  if (disabled && disabledHint) {
    return (
      <PopoverTooltip
        usePortal={disabledHintPortal}
        buttonClassName={disabledHintPortal ? 'w-full' : ''}
        className="text-text-foreground-secondary"
        text={<p className="text-sm text-text-foreground-secondary">{disabledHint}</p>}
        hideArrow
      >
        {btn}
      </PopoverTooltip>
    )
  }

  return btn
}

export const NewButton = forwardRef(Button)
export default NewButton
