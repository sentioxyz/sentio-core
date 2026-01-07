import React, { ComponentProps, ElementType } from 'react'
import { Tab as HeadlessTab } from '@headlessui/react'
import { classNames } from '../utils/classnames'

export const getTabClassName = ({ selected }: { selected: boolean }) =>
  classNames(
    'font-ilabel py-1 text-sm leading-5 border-b-2 outline-0',
    selected
      ? 'text-primary border-primary-500'
      : 'text-gray hover:text-primary border-transparent hover:border-primary-500'
  )

export const Group = ({
  children,
  className,
  onChange,
  defaultIndex,
  selectedIndex
}: {
  children: React.ReactNode
  className?: string
  onChange?: (index: number) => void
  defaultIndex?: number
  selectedIndex?: number
}) => {
  return (
    <HeadlessTab.Group
      onChange={onChange}
      defaultIndex={defaultIndex}
      selectedIndex={selectedIndex}
    >
      <div className={classNames('flex flex-col', className)}>{children}</div>
    </HeadlessTab.Group>
  )
}

export const List = ({
  tabs,
  disabledTabs,
  className,
  noBorder,
  children
}: {
  tabs: React.ReactNode[]
  disabledTabs?: number[]
  className?: string
  noBorder?: boolean
  children?: React.ReactNode
}) => {
  return (
    <HeadlessTab.List
      className={classNames(
        'flex-0 border-border-color flex justify-start space-x-6 overflow-x-auto px-4',
        noBorder ? 'border-b-0' : 'border-b'
      )}
    >
      {tabs.map((tab, index) => (
        <HeadlessTab
          key={index}
          className={({ selected }) =>
            classNames(
              'font-ilabel outline-primary/50 whitespace-nowrap border-b-2 py-1 text-sm leading-5 outline-offset-2',
              selected
                ? 'border-primary text-primary'
                : 'text-gray hover:text-primary border-transparent',
              className
            )
          }
          disabled={disabledTabs?.includes(index)}
        >
          {tab}
        </HeadlessTab>
      ))}
      {children}
    </HeadlessTab.List>
  )
}

export const Panels: ElementType = React.forwardRef<
  any,
  ComponentProps<typeof HeadlessTab.Panels>
>(function Panels({ children, className, ...props }, ref) {
  return (
    <HeadlessTab.Panels
      className={classNames('flex-1', className || 'w-full overflow-auto')}
      {...props}
      ref={ref}
    >
      {children}
    </HeadlessTab.Panels>
  )
})

export const Panel = (props: any) => {
  const { className, ...otherProps } = props
  return (
    <HeadlessTab.Panel
      className={classNames('space-y-2 outline-0', className)}
      {...otherProps}
    />
  )
}
