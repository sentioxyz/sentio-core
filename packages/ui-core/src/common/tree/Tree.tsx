import React, { useCallback, useState, useMemo, forwardRef } from 'react'
import * as Icons from './TreeIcons'
import { cx as classNames } from 'class-variance-authority'
import { useEffect } from 'react'

interface TreeProps {
  className?: string
  content?: React.ReactNode
  contentClassName?: string
  children?: React.ReactNode
  prefix?: React.ReactNode
  suffix?: React.ReactNode
  type?: React.ReactNode
  open?: boolean
  style?: React.CSSProperties
  depth?: number
  onClick?: () => void
  expandIcon?: React.ReactNode
  collapseIcon?: React.ReactNode

  // controled open function
  onOpenClick?: (open: boolean) => void
  showToggle?: boolean
}

const Line = () => {
  return (
    <div className="-my-1.5 mr-[19px] h-full min-h-[24px] w-[px] translate-x-2 border-l border-dashed border-gray-400"></div>
  )
}

export default forwardRef<HTMLDivElement, TreeProps>(function Tree(
  {
    open: defaultOpen,
    content,
    children,
    depth = 0,
    type,
    contentClassName,
    prefix,
    suffix,
    onClick,
    onOpenClick,
    showToggle,
    className,
    expandIcon = <Icons.PlusSquareO className="h-4 w-4 align-middle" />,
    collapseIcon = <Icons.MinusSquareO className="h-4 w-4 align-middle" />
  }: TreeProps,
  ref
) {
  const [open, setOpen] = useState(defaultOpen)

  useEffect(() => {
    setOpen(defaultOpen)
  }, [defaultOpen])

  const toggle = useCallback(
    (evt: React.MouseEvent) => {
      evt.stopPropagation()
      if (onOpenClick) {
        onOpenClick(!open)
      } else {
        setOpen((val) => {
          return !val
        })
      }
    },
    [open, onOpenClick]
  )
  const lineNodes = useMemo(() => {
    const lines: React.ReactNode[] = []
    for (let i = 0; i < depth; i++) {
      lines.push(<Line key={i} />)
    }
    return lines
  }, [depth])
  return (
    <div
      className={classNames(
        'text-icontent font-icontent overflow-hidden text-ellipsis whitespace-nowrap align-middle',
        className
      )}
    >
      <div
        ref={ref}
        className={classNames(
          'flex items-center px-2 py-1 hover:bg-gray-100',
          contentClassName
        )}
        onClick={onClick}
      >
        <div className="inline-flex shrink-0 items-center self-stretch">
          {lineNodes}
        </div>
        <div className="inline-flex shrink-0 items-center">
          {children || showToggle ? (
            <button
              className="text-gray hover:text-primary-500 dark:hover:text-primary-700 mr-1.5 cursor-pointer"
              onClick={toggle}
            >
              {open ? collapseIcon : expandIcon}
            </button>
          ) : (
            <span className="mr-[19px] h-1 w-px"></span>
          )}
        </div>
        {type}
        <span className="flex-1 align-middle">{content}</span>
      </div>
      {prefix}
      {open ? children : null}
      {suffix}
    </div>
  )
})
