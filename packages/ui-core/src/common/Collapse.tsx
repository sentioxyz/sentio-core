import { ChevronDownIcon } from '@heroicons/react/20/solid'
import { cx as classNames } from 'class-variance-authority'
import { useBoolean } from '../utils/use-boolean'
import { type ReactNode, type FC, useEffect } from 'react'

interface CollapseProps {
  title: ReactNode | string
  children?: ReactNode
  className?: string
  titleClassName?: string
  defaultOpen?: boolean
  iconClassName?: string
}

export const Collapse: FC<CollapseProps> = ({
  title,
  children,
  className,
  titleClassName,
  defaultOpen = false,
  iconClassName = 'h-5 w-5'
}) => {
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
          visible ? 'opacity-100' : 'opacity-0'
        )}
        style={{
          maxHeight: visible ? '2000px' : '0px'
        }}
      >
        {children}
      </div>
    </div>
  )
}
