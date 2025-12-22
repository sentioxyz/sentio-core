import { ReactNode } from 'react'
import { cx as classNames } from 'class-variance-authority'
import isFunction from 'lodash/isFunction'
import { useState, useCallback } from 'react'
import { LuChevronRight } from 'react-icons/lu'

interface Props {
  defaultOpen?: boolean
  children: ReactNode
  title: string | ReactNode | ((open: boolean) => ReactNode)
  titleClassName?: string
  containerClassName?: string
  iconClassName?: string
  className?: string
}

export function DisclosurePanel({
  title,
  children,
  defaultOpen,
  className,
  containerClassName,
  iconClassName = 'h-5 w-5',
  titleClassName
}: Props) {
  const [open, setOpen] = useState(defaultOpen || false)

  const toggle = useCallback(() => {
    setOpen((prev) => !prev)
  }, [])

  return (
    <div
      className={
        containerClassName ||
        'dark:bg-sentio-gray-200 w-full rounded bg-[#F6F8FA]'
      }
    >
      <button
        className={classNames(
          open ? 'rounded-t' : 'rounded',
          'focus-visible:ring-primary-500 text-ilabel font-ilabel text-text-foreground hover:bg-sentio-gray-100 dark:hover:bg-sentio-gray-400 flex w-full px-2 py-1.5 text-left focus:outline-none focus-visible:ring focus-visible:ring-opacity-75',
          titleClassName
        )}
        onClick={toggle}
      >
        <LuChevronRight
          className={classNames(
            open ? 'rotate-90 transform' : '',
            'mr-1 self-center transition-all',
            iconClassName
          )}
        />
        {isFunction(title) ? title(open) : title}
      </button>
      {open && <div className={classNames('p-2', className)}>{children}</div>}
    </div>
  )
}
