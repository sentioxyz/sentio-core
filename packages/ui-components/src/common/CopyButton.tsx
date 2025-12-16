import { useState, useRef, memo } from 'react'
import { CopyToClipboard, Props as CopyProps } from 'react-copy-to-clipboard'
import { HiCheck } from 'react-icons/hi'
import {
  useFloating,
  useHover,
  autoUpdate,
  offset,
  useInteractions,
  arrow,
  FloatingPortal,
  useDelayGroup
} from '@floating-ui/react'
import { cx as classNames } from 'class-variance-authority'
import { Square2StackIcon } from '@heroicons/react/24/outline'

interface Props extends CopyProps {
  iconClass?: string
  noHint?: boolean
  className?: string
  hintFixed?: boolean
}

const DEFAULT_DELAY = {
  open: 0,
  close: 100
}

export const CopyButton = memo(function CopyButton({
  text,
  children,
  onCopy,
  iconClass,
  noHint,
  hintFixed,
  ...props
}: Props) {
  const [copied, setCopied] = useState(false)
  const [open, setOpen] = useState(false)
  const arrowRef = useRef(null)
  const {
    x,
    y,
    refs,
    strategy,
    context,
    // @ts-ignore pnpm
    middlewareData: { arrow: { x: arrowX, y: arrowY } = {} }
  } = useFloating({
    open,
    onOpenChange: setOpen,
    placement: 'right',
    // Make sure the tooltip stays on the screen
    whileElementsMounted: autoUpdate,
    middleware: [offset(10), arrow({ element: arrowRef })]
    // strategy: 'fixed',
  })
  const { delay } = useDelayGroup(context)
  const { getReferenceProps, getFloatingProps } = useInteractions([
    useHover(context, {
      delay: delay ?? DEFAULT_DELAY
    })
  ])

  return (
    <>
      <CopyToClipboard
        text={text}
        onCopy={(text: any, result: any) => {
          onCopy?.(text, result)
          if (!copied) {
            setCopied(true)
            setTimeout(() => {
              setCopied(false)
            }, 1000)
          }
        }}
        {...props}
      >
        <div className="inline-flex cursor-pointer" ref={refs.setReference} {...getReferenceProps}>
          {children ?? (
            <button className={`h-5 w-5 flex-shrink-0 ${iconClass}`} type="button">
              {copied ? (
                <HiCheck className="h-full w-full text-green-600" />
              ) : (
                <Square2StackIcon className="hover:text-primary h-full w-full" />
              )}
            </button>
          )}
        </div>
      </CopyToClipboard>
      {noHint ? (
        copied && (
          <div
            className={classNames(
              'text-icontent dark:bg-text-background dark:text-text-foreground whitespace-nowrap rounded bg-gray-800 px-2 py-1 text-white',
              hintFixed ? 'fixed' : 'absolute'
            )}
          >
            Copied
          </div>
        )
      ) : (
        <FloatingPortal id="copy_hint">
          <div
            className={classNames(
              open ? 'block' : 'hidden',
              'select-none rounded-md bg-gray-600 px-2 py-1 text-sm text-white dark:bg-gray-200'
            )}
            ref={refs.setFloating}
            style={{
              position: strategy,
              top: y ?? 0,
              left: x ?? 0,
              zIndex: 1000
            }}
            {...getFloatingProps()}
          >
            <div
              className="arrow before:visible before:rotate-45"
              ref={arrowRef}
              style={{
                right: arrowX ?? 0,
                top: arrowY ?? 0,
                position: 'absolute',
                zIndex: -1
              }}
            />
            <div>{copied ? 'Copied to clipboard' : 'Click to copy'}</div>
          </div>
        </FloatingPortal>
      )}
    </>
  )
})
