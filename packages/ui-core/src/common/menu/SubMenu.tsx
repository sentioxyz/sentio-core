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
      'bg-gray-100 text-text-foreground dark:bg-primary-600 dark:text-white',
    default: 'text-text-foreground',
    disabled: 'text-gray-400 cursor-not-allowed'
  },
  danger: {
    active: 'bg-red-100 text-red-600 dark:bg-red-600 dark:text-white',
    default: 'text-red-600',
    disabled: 'text-red-200 dark:text-red-600/40 cursor-not-allowed'
  }
}

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
            className={classNames(
              item.disabled
                ? COLOR_MAP[item.status || 'default'].disabled
                : active
                  ? COLOR_MAP[item.status || 'default'].active
                  : COLOR_MAP[item.status || 'default'].default,
              'text-ilabel font-ilabel flex w-full items-center px-4 py-1.5 transition-colors duration-200'
            )}
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
                <span className="text-icontent font-icontent text-gray cursor-auto">
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
          ? 'pointer-events-none cursor-not-allowed text-gray-400'
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
        <span className="flex-shrink flex-grow text-left">{label}</span>
        <ChevronRightIcon
          className={classNames(
            open ? 'text-gray-500' : 'text-gray-400',
            'h-4.5 w-4.5 flex-shrink-0  group-hover:text-gray-500',
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
          className="dark:bg-sentio-gray-100 dark:divide-sentio-gray-400/50 w-48 origin-top cursor-pointer divide-y divide-gray-200 rounded-md bg-white shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none dark:ring-gray-100"
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
