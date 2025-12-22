import { ChevronDownIcon } from '@heroicons/react/20/solid'
import { cx as classNames } from 'class-variance-authority'
import { useBoolean } from '../utils/use-boolean'
import { ReactNode, useEffect } from 'react'

interface CollapseProps {
  title: ReactNode | string
  children: React.ReactNode
  className?: string
  titleClassName?: string
  defaultOpen?: boolean
  iconClassName?: string
}

export const Collapse = ({
  title,
  children,
  className,
  titleClassName,
  defaultOpen = false,
  iconClassName = 'h-5 w-5'
}: CollapseProps) => {
  const { toggle, value: visible, setTrue, setFalse } = useBoolean(defaultOpen)

  useEffect(() => {
    if (defaultOpen) {
      setTrue()
    } else {
      setFalse()
    }
  }, [defaultOpen, setTrue, setFalse])

  return (
    <div className={classNames('space-y-2', className)}>
      <span
        className={classNames(
          'text-gray hover:text-primary active:text-primary-700 inline-flex cursor-pointer items-center gap-2',
          titleClassName
        )}
        onClick={toggle}
      >
        {title}
        <ChevronDownIcon
          className={classNames(
            'transition',
            iconClassName,
            visible ? 'rotate-180' : ''
          )}
        />
      </span>
      <div
        className={classNames(
          'overflow-hidden transition-all duration-200',
          visible ? 'max-h-[2000px] opacity-100' : 'max-h-0 opacity-0'
        )}
      >
        {children}
      </div>
    </div>
  )
}
