/**
 * The differece between this and the PopoverTooltip.tsx is that this one pass mouse click event to the parent
 */
import React, { type FC, useRef, useState, useEffect } from 'react'
import {
  useFloating,
  useHover,
  useInteractions,
  safePolygon,
  arrow,
  offset,
  shift,
  Strategy,
  flip,
  autoUpdate,
  FloatingPortal,
  useDelayGroup
} from '@floating-ui/react'
import { OffsetOptions, Placement } from '@floating-ui/core'
import { cx as classNames } from 'class-variance-authority'
import { isString } from 'lodash'

interface Props {
  text?: string | React.ReactNode
  className?: string
  buttonClassName?: string
  activeButtonClassName?: string
  icon?: React.ReactNode
  children?: React.ReactNode
  strategy?: Strategy
  hideArrow?: boolean
  offsetOptions?: OffsetOptions
  placementOption?: Placement
  maxWidth?: string
  usePortal?: boolean
  enableFadeAnimation?: boolean
  animationDuration?: number
}

export const PopoverTooltip: FC<Props> = ({
  icon,
  text,
  className,
  buttonClassName,
  activeButtonClassName,
  children,
  strategy: propStrategy,
  hideArrow,
  offsetOptions = 8,
  placementOption = 'bottom',
  maxWidth = 'max-w-[300px]',
  usePortal = false,
  enableFadeAnimation = false,
  animationDuration = 150
}) => {
  const arrowRef = useRef(null)
  const [open, setOpen] = useState(false)
  const [isVisible, setIsVisible] = useState(false)
  const timeoutRef = useRef<NodeJS.Timeout>()

  const {
    x,
    y,
    refs,
    strategy,
    middlewareData: { arrow: { x: arrowX, y: arrowY } = {} },
    context,
    placement
  } = useFloating({
    open,
    onOpenChange: (newOpen) => {
      setOpen(newOpen)
      if (enableFadeAnimation) {
        if (newOpen) {
          setIsVisible(true)
        } else {
          if (timeoutRef.current) {
            clearTimeout(timeoutRef.current)
          }
          timeoutRef.current = setTimeout(() => {
            setIsVisible(false)
          }, animationDuration)
        }
      } else {
        setIsVisible(newOpen)
      }
    },
    middleware: [
      offset(offsetOptions),
      flip(),
      shift(),
      arrow({ element: arrowRef, padding: 8 })
    ],
    strategy: propStrategy,
    placement: placementOption,
    whileElementsMounted: autoUpdate
  })

  const {
    delay = {
      open: 500,
      close: 0
    }
  } = useDelayGroup(context)

  const { getReferenceProps, getFloatingProps } = useInteractions([
    useHover(context, {
      handleClose: safePolygon({
        buffer: -Infinity
      }),
      delay: delay
    })
  ])

  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current)
      }
    }
  }, [])

  if (!text)
    return (
      <>
        {icon}
        {children}
      </>
    )

  const Portal = usePortal ? FloatingPortal : React.Fragment

  return (
    <div className={classNames('relative flex items-center', className)}>
      <div
        ref={refs.setReference}
        {...getReferenceProps()}
        className={classNames(buttonClassName, open && activeButtonClassName)}
      >
        {icon}
        {children}
      </div>

      {(enableFadeAnimation ? isVisible : open) && (
        <Portal>
          <div className="_sentio_">
            <div
              className={classNames(
                'sentio-tooltip dark:bg-sentio-gray-200 z-10 rounded-md bg-white p-2 text-xs shadow-lg ring-1 ring-black/5 dark:ring-gray-100/5',
                enableFadeAnimation &&
                  `transition-opacity duration-[${animationDuration}ms] ease-in-out`,
                enableFadeAnimation ? (open ? 'opacity-100' : 'opacity-0') : ''
              )}
              ref={refs.setFloating}
              style={{
                position: strategy,
                top: y ?? 0,
                left: x ?? 0
              }}
              {...getFloatingProps}
            >
              {!hideArrow && (() => {
                const side = placement.split('-')[0]
                const borderClass = {
                  bottom: 'border-l border-t',
                  top: 'border-r border-b',
                  right: 'border-l border-b',
                  left: 'border-t border-r'
                }[side]
                const arrowStyle: React.CSSProperties = { position: 'absolute' }
                if (side === 'bottom') { arrowStyle.top = -4; arrowStyle.left = arrowX ?? 0 }
                else if (side === 'top') { arrowStyle.bottom = -4; arrowStyle.left = arrowX ?? 0 }
                else if (side === 'right') { arrowStyle.left = -4; arrowStyle.top = arrowY ?? 0 }
                else if (side === 'left') { arrowStyle.right = -4; arrowStyle.top = arrowY ?? 0 }
                return (
                  <div
                    ref={arrowRef}
                    className={classNames(
                      'h-2 w-2 rotate-45 bg-white dark:bg-sentio-gray-200 border-black/5 dark:border-gray-100/5',
                      borderClass
                    )}
                    style={arrowStyle}
                  />
                )
              })()}
              {isString(text) ? (
                <p
                  className={classNames('w-max whitespace-pre-wrap', maxWidth)}
                >
                  {text}
                </p>
              ) : (
                <div className={classNames('w-max overflow-auto', maxWidth)}>
                  {text}
                </div>
              )}
            </div>
          </div>
        </Portal>
      )}
    </div>
  )
}
