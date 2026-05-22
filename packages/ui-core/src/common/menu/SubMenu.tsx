import { useState, createContext, useContext } from 'react'
import {
  useFloating,
  useHover,
  useInteractions,
  autoUpdate,
  flip,
  shift,
  safePolygon,
  Placement
} from '@floating-ui/react'
import { Menu } from '@headlessui/react'
import { classNames } from '../../utils/classnames'
import { cva } from 'class-variance-authority'
import { HiCheck } from 'react-icons/hi'
import { PopoverTooltip } from '../DivTooltip'
import { ChevronRightIcon } from '@heroicons/react/20/solid'
import { IMenuItem, OnSelectMenuItem } from './types'

export const COLOR_MAP: Record<
  string,
  { active: string; default: string; disabled: string }
> = {
  default: {
    active:
      'bg-primary-50 text-primary-600',
    default: 'text-text-foreground',
    disabled: 'text-text-foreground-disabled cursor-not-allowed'
  },
  danger: {
    active: 'bg-red-100 text-red-600 dark:bg-red-600 dark:text-white',
    default: 'text-red-600',
    disabled: 'text-red-200 dark:text-red-600/40 cursor-not-allowed'
  }
}

const menuItemClass = cva(
  'text-ilabel font-ilabel flex w-full items-center px-4 py-1.5',
  {
    variants: {
      status: {
        default: '',
        danger: ''
      },
      disabled: {
        true: 'cursor-not-allowed',
        false: ''
      },
      active: {
        true: '',
        false: ''
      },
      selected: {
        true: 'bg-primary-600 text-white',
        false: ''
      }
    },
    compoundVariants: [
      { status: 'default', disabled: false, active: false, selected: false, class: 'text-text-foreground' },
      { status: 'default', disabled: false, active: true, selected: false, class: 'bg-primary-50 text-primary-600' },
      { status: 'default', disabled: true, selected: false, class: 'text-text-foreground-disabled' },
      { status: 'danger', disabled: false, active: false, selected: false, class: 'text-red-600' },
      { status: 'danger', disabled: false, active: true, selected: false, class: 'bg-red-100 text-red-600 dark:bg-red-600 dark:text-white' },
      { status: 'danger', disabled: true, selected: false, class: 'text-red-200 dark:text-red-600/40' }
    ],
    defaultVariants: {
      status: 'default',
      disabled: false,
      active: false,
      selected: false
    }
  }
)

export const MenuContext = createContext<{ selectedKey?: string }>({})

type Props = IMenuItem & {
  items: IMenuItem[][]
  onSelect?: OnSelectMenuItem
  active: boolean
  name: string
  placement?: Placement
  buttonClass?: string
}

interface ItemProps {
  item: IMenuItem
  onSelect?: OnSelectMenuItem
  labelClassName?: string
}

export const MenuItem = ({ item, onSelect, labelClassName }: ItemProps) => {
  const { selectedKey } = useContext(MenuContext)
  return (
    <Menu.Item disabled={item.disabled}>
      {({ active }) => {
        if (item.items) {
          return (
            <SubMenuButton
              items={item.items}
              icon={item.icon}
              key={item.key}
              name={item.key}
              label={item.label}
              onSelect={onSelect}
              active={active}
            />
          )
        }
        const buttonNode = (
          <button
            onClick={(e) => onSelect?.(item.key, e, item)}
            className={menuItemClass({
              status: item.status as 'default' | 'danger' || 'default',
              disabled: !!item.disabled,
              active,
              selected: item.key === selectedKey
            })}
            disabled={item.disabled}
          >
            {item.icon}
            <span
              className={classNames(
                'flex-1 truncate text-left',
                labelClassName
              )}
            >
              {item.label}
            </span>
            {item.key === selectedKey ? (
              <HiCheck className="icon-lg ml-2" />
            ) : null}
          </button>
        )
        if (item.disabled && item.disabledHint) {
          return (
            <PopoverTooltip
              text={
                <span className="text-icontent font-icontent text-text-foreground-secondary cursor-auto">
                  {item.disabledHint}
                </span>
              }
              strategy="fixed"
            >
              {buttonNode}
            </PopoverTooltip>
          )
        }
        return buttonNode
      }}
    </Menu.Item>
  )
}

export const SubMenuButton = (props: Props) => {
  const {
    label,
    status,
    items,
    disabled,
    onSelect,
    active,
    placement = 'right-start',
    buttonClass
  } = props
  const [open, setOpen] = useState(false)
  const { refs, floatingStyles, context } = useFloating({
    open,
    onOpenChange: setOpen,
    placement,
    whileElementsMounted: autoUpdate,
    middleware: [flip(), shift()]
  })
  const { getReferenceProps, getFloatingProps } = useInteractions([
    useHover(context, {
      handleClose: safePolygon()
    })
  ])

  return (
    <Menu
      as="div"
      className={classNames(
        'group flex items-center',
        'text-ilabel rounded-md',
        disabled
          ? 'pointer-events-none cursor-not-allowed text-text-foreground-disabled'
          : 'cursor-pointer'
      )}
    >
      <Menu.Button
        className={classNames(
          active || open
            ? COLOR_MAP[status || 'default'].active
            : COLOR_MAP[status || 'default'].default,
          'text-ilabel font-ilabel flex w-full items-center px-4 py-1.5',
          buttonClass
        )}
        ref={refs.setReference}
        onClick={(e) => {
          e.preventDefault()
          onSelect && onSelect(props.name, e)
        }}
        {...getReferenceProps}
      >
        {props.icon}
        <span className="shrink grow text-left">{label}</span>
        <ChevronRightIcon
          className={classNames(
            open ? 'text-text-foreground-secondary' : 'text-text-foreground-disabled',
            'h-4.5 w-4.5 shrink-0  group-hover:text-text-foreground-secondary',
            placement?.startsWith('bottom') ? 'rotate-90' : ''
          )}
          aria-label="expand items"
        />
      </Menu.Button>
      {open && (
        <Menu.Items
          static
          ref={refs.setFloating}
          style={floatingStyles}
          className="dark:bg-sentio-gray-100 dark:divide-sentio-gray-400/50 w-48 origin-top cursor-pointer divide-y divide-gray-200 rounded-md bg-white shadow-lg ring-1 ring-black/5 focus:outline-hidden dark:ring-gray-100/5"
          {...getFloatingProps}
        >
          {items.map((items, i) =>
            items && items.length > 0 ? (
              <div
                key={i}
                className="overflow-auto py-1"
                style={{ maxHeight: '60vh' }}
              >
                {items.map((item) => (
                  <MenuItem key={item.key} item={item} onSelect={onSelect} />
                ))}
              </div>
            ) : null
          )}
        </Menu.Items>
      )}
    </Menu>
  )
}
