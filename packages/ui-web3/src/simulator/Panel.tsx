import { Disclosure } from '@headlessui/react'
import { ReactNode } from 'react'
import { MinusIcon, PlusIcon } from '@heroicons/react/24/outline'

interface Props {
  defaultOpen?: boolean
  children: ReactNode
  title: string | ReactNode | ((open: boolean) => ReactNode)
  titleClassName?: string
  containerClassName?: string
  className?: string
}

export function DisclosurePanel({
  title,
  children,
  defaultOpen,
  className,
  containerClassName,
  titleClassName
}: Props) {
  const isFunction = (val: any): val is Function => typeof val === 'function'

  return (
    <Disclosure defaultOpen={defaultOpen}>
      {({ open }) => (
        <div className={containerClassName || 'w-full rounded'}>
          <Disclosure.Button
            className={`sticky -top-2 z-[1] flex w-full items-center justify-between bg-gray-50 py-2 text-left text-sm font-medium text-gray-900 hover:bg-gray-100 ${titleClassName || ''}`}
          >
            {isFunction(title) ? title(open) : title}
            {open ? (
              <MinusIcon className="h-4 w-4 text-gray-600 hover:text-blue-600" />
            ) : (
              <PlusIcon className="h-4 w-4 text-gray-600 hover:text-blue-600" />
            )}
          </Disclosure.Button>
          <Disclosure.Panel className={className} unmount={false}>
            {children}
          </Disclosure.Panel>
        </div>
      )}
    </Disclosure>
  )
}
