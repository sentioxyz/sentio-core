import { ReactNode, useRef } from 'react'
import {
  useFloating,
  arrow as arrowMiddleware,
  shift,
  FloatingPortal,
  Placement,
  autoUpdate
} from '@floating-ui/react'
import { Popover } from '@headlessui/react'
import { classNames } from '../../utils/classnames'

interface Props {
  children: React.ReactNode
  content:
    | React.ReactNode
    | ((renderProps: { open: boolean; close: () => void }) => ReactNode)
  className?: string
  contentClassName?: string
  extra?: React.ReactNode
  arrow?: boolean
  portal?: boolean
  as?: React.ElementType
  ariaLabel?: string
  placement?: Placement
  containerClassName?: string
}

export const PopoverButton = (props: Props) => {
  const {
    children,
    arrow,
    content,
    extra,
    className,
    contentClassName,
    portal,
    as,
    ariaLabel,
    placement,
    containerClassName
  } = props
  const arrowRef = useRef<HTMLDivElement>(null)
  const middleware = [shift()]
  if (arrow) {
    middleware.push(arrowMiddleware({ element: arrowRef }))
  }
  const {
    x,
    y,
    strategy,
    refs,
    middlewareData: { arrow: { x: arrowX, y: arrowY } = {} }
  } = useFloating({
    placement: placement,
    middleware,
    whileElementsMounted: autoUpdate
  })

  const floatingElement = (
    <Popover.Panel
      ref={refs.setFloating}
      className={classNames(
        'sentio-tooltip bg-default-bg shadow-xs border-main z-10 rounded-md border',
        contentClassName
      )}
      style={{
        position: strategy,
        top: y ?? undefined,
        left: x ?? undefined
      }}
    >
      {(renderProps) => (
        <>
          {arrow && (
            <div
              className="arrow before:border-border-color -translate-y-2 bg-white before:visible before:border before:border-b-0 before:border-r-0 dark:bg-[#2e2e2e]"
              ref={arrowRef}
              style={{
                left: arrowX ?? 0,
                top: arrowY ?? 0,
                position: 'absolute'
              }}
            />
          )}
          {typeof content === 'function' ? content(renderProps) : content}
        </>
      )}
    </Popover.Panel>
  )
  return (
    <Popover className={containerClassName}>
      <Popover.Button
        as={as}
        ref={refs.setReference}
        className={className}
        aria-label={ariaLabel}
      >
        {children}
      </Popover.Button>
      {extra}
      {portal ? (
        <FloatingPortal>{floatingElement}</FloatingPortal>
      ) : (
        floatingElement
      )}
    </Popover>
  )
}
