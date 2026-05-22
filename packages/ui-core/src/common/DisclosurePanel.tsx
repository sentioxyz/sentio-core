import { type ReactNode, type FC, useState, useCallback } from 'react'
import { cx as classNames } from 'class-variance-authority'
import isFunction from 'lodash/isFunction'
import { LuChevronRight } from 'react-icons/lu'

interface Props {
  defaultOpen?: boolean
  children?: ReactNode
  title: string | ReactNode | ((open: boolean) => ReactNode)
  titleClassName?: string
  containerClassName?: string
  iconClassName?: string
  className?: string
}

export const DisclosurePanel: FC<Props> = ({
  title,
  children,
  defaultOpen,
  className,
  containerClassName,
  iconClassName = 'h-4 w-4',
  titleClassName
}) => {
  const [open, setOpen] = useState(defaultOpen || false)

  const toggle = useCallback(() => {
    setOpen((prev) => !prev)
  }, [])

  return (
    <div
      className={
        containerClassName ||
        'w-full rounded-sm border border-main'
      }
    >
      <button
        className={classNames(
          open ? 'rounded-t' : 'rounded-sm',
          'focus-visible:ring-primary-500/75 text-ilabel font-medium text-text-foreground hover:bg-sentio-gray-100 dark:hover:bg-sentio-gray-800 flex w-full px-2 py-1.5 text-left focus:outline-hidden focus-visible:ring-3',
          titleClassName
        )}
        onClick={toggle}
        type="button"
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
