import {
  ReactNode,
  Fragment,
  useState,
  useContext,
  CSSProperties,
  createContext,
  useMemo,
  useEffect,
  useRef
} from 'react'
import { Menu, Transition } from '@headlessui/react'
import {
  FloatingPortal,
  useFloating,
  shift,
  flip,
  autoUpdate,
  Placement,
  offset as FloatingOffset
} from '@floating-ui/react'
import { MenuContext, MenuItem } from './SubMenu'
import { classNames } from '../../utils/classnames'
import { NavSizeContext } from '../../utils/nav-size-context'
import { IMenuItem, OnSelectMenuItem } from './types'

interface Props {
  items: IMenuItem[][]
  groupLabels?: string[]
  buttonIcon: ReactNode | ((menuOpen: boolean) => ReactNode)
  buttonClassName?: string
  itemsClassName?: string
  itemLabelClassName?: string
  onSelect?: OnSelectMenuItem
  ariaLabel?: string
  header?: ReactNode
  footer?: ReactNode
  renderItem?: (data: IMenuItem) => React.ReactNode
  placement?: Placement
  offset?: any
  portal?: boolean
  width?: CSSProperties['width']
  selectedKey?: IMenuItem['key']
  onOpenCallback?: () => void
}

export function PopupMenuButton({
  buttonIcon,
  items,
  groupLabels,
  onSelect,
  ariaLabel,
  header,
  footer,
  buttonClassName,
  itemsClassName = '',
  itemLabelClassName,
  renderItem,
  placement = 'bottom-start',
  offset = 0,
  portal = true,
  width,
  selectedKey,
  onOpenCallback
}: Props) {
  const [menuOpen, setMenuOpen] = useState(false)
  const { small } = useContext(NavSizeContext)
  const { refs, floatingStyles, context } = useFloating({
    open: menuOpen,
    onOpenChange: setMenuOpen,
    middleware: [FloatingOffset(offset), flip(), shift()],
    placement,
    whileElementsMounted: autoUpdate
  })
  const itemStyle = useMemo(() => {
    return {
      width
    }
  }, [width])
  const onOpenCallbackRef = useRef(onOpenCallback)
  onOpenCallbackRef.current = onOpenCallback
  useEffect(() => {
    if (menuOpen) {
      onOpenCallbackRef.current?.()
    }
  }, [menuOpen])
  let menuItems: React.ReactNode = null
  if (menuOpen && items.length > 0) {
    menuItems = (
      <MenuContext.Provider value={{ selectedKey }}>
        <div ref={refs.setFloating} style={floatingStyles}>
          <Transition
            as={Fragment}
            enter="transition ease-out duration-100"
            enterFrom="transform opacity-0 scale-95"
            enterTo="transform opacity-100 scale-100"
            leave="transition ease-in duration-75"
            leaveFrom="transform opacity-100 scale-100"
            leaveTo="transform opacity-0 scale-95"
          >
            <Menu.Items
              className="dark:bg-sentio-gray-200 dark:divide-sentio-gray-400/50 z-10 mt-1 w-[80vw] origin-top cursor-pointer divide-y divide-gray-200 rounded-md bg-white shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none dark:ring-gray-100 sm:w-64"
              style={itemStyle}
            >
              {header}
              <div className={`${itemsClassName} divide-y`}>
                {items.map((items, i) => (
                  <div key={i} className="py-1">
                    {groupLabels?.[i] ? (
                      <div className="px-4 py-0.5 text-[10px] font-medium leading-[12px] text-gray-500">
                        {groupLabels[i]}
                      </div>
                    ) : null}
                    {items.map((item) =>
                      renderItem ? (
                        renderItem(item)
                      ) : (
                        <MenuItem
                          item={item}
                          onSelect={onSelect}
                          key={item.key}
                          labelClassName={itemLabelClassName}
                        />
                      )
                    )}
                  </div>
                ))}
              </div>
              {footer}
            </Menu.Items>
          </Transition>
        </div>
      </MenuContext.Provider>
    )
  }
  return (
    <Menu>
      {({ open }) => {
        setTimeout(() => {
          setMenuOpen(open)
        }, 0)
        return (
          <>
            <Menu.Button
              className={classNames(
                'text-gray w-fit px-1 hover:text-gray-500 active:text-gray-700',
                buttonClassName
              )}
              aria-label={ariaLabel}
              ref={refs.setReference}
              as={buttonIcon ? 'div' : undefined}
            >
              {typeof buttonIcon === 'function'
                ? buttonIcon(menuOpen)
                : buttonIcon}
            </Menu.Button>
            {portal ? <FloatingPortal>{menuItems}</FloatingPortal> : menuItems}
          </>
        )
      }}
    </Menu>
  )
}
