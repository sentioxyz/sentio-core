import { Fragment, ReactNode, useCallback, useEffect, useRef } from 'react'
import { Dialog, Transition } from '@headlessui/react'
import { XMarkIcon as XIcon } from '@heroicons/react/24/outline'

interface Props {
  title?: string | ReactNode
  open: boolean
  onClose: () => void
  children?: ReactNode
  size?: '2xl' | '3xl' | '4xl' | '5xl' | '6xl' | '7xl' | 'full' | string
  headAddon?: ReactNode
  triggerClose?: 'all' | 'button'
  noAnimation?: boolean
}

export default function SlideOver({
  title,
  open,
  onClose,
  children,
  size,
  headAddon,
  triggerClose = 'all',
  noAnimation
}: Props) {
  const onDialogClose = useCallback(() => {
    if (triggerClose === 'all') {
      onClose()
    }
  }, [triggerClose, onClose])
  const openRef = useRef(open)
  openRef.current = open

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        if (openRef.current) {
          onClose()
        }
      }
    }
    if (triggerClose === 'button') {
      window.addEventListener('keydown', handleKeyDown)
      return () => {
        window.removeEventListener('keydown', handleKeyDown)
      }
    }
  }, [triggerClose, onClose])

  const contentNode = (
    <div className="fixed inset-0 overflow-hidden">
      <div className="absolute inset-y-0 right-0 flex max-w-full pl-10 sm:pl-16">
        <div className="max-w-2xl max-w-3xl max-w-4xl max-w-5xl max-w-6xl max-w-7xl">
          {/*  hack, make tailwind css compile all these widths */}
        </div>
        <Dialog.Panel
          className={`dark:bg-sentio-gray-100 pointer-events-auto flex h-full w-screen flex-col overflow-x-hidden border-l bg-white shadow-md max-w-${
            size || '2xl'
          }`}
        >
          {/* Header */}
          <div className="dark:bg-sentio-gray-100 relative border-b bg-white px-4 py-3">
            <div className="flex h-auto items-start justify-between space-x-3 sm:h-5">
              <div className="flex-1 space-y-1">
                <Dialog.Title className="text-text-foreground text-[15px] font-semibold">
                  {title}
                </Dialog.Title>
              </div>
              <div className="flex-0 flex h-auto items-center sm:h-5">
                {headAddon}
                <button
                  type="button"
                  className="hover:text-text-foreground ml-2 text-gray-800 dark:text-gray-500"
                  onClick={() => onClose()}
                >
                  <span className="sr-only">Close panel</span>
                  <XIcon className="h-5 w-5" aria-hidden="true" />
                </button>
              </div>
            </div>
          </div>
          {/* Divider container */}
          <div className="flex flex-1 overflow-y-auto overflow-x-hidden">
            {children}
          </div>
        </Dialog.Panel>
      </div>
    </div>
  )

  if (noAnimation) {
    return (
      <Dialog
        open={open}
        as="div"
        className="relative z-10"
        id={'test'}
        onClose={onDialogClose}
      >
        {contentNode}
      </Dialog>
    )
  }

  return (
    <Transition.Root show={open} as={'div'}>
      <Dialog
        static
        as="div"
        className="relative z-10"
        id={'test'}
        onClose={onDialogClose}
      >
        <Transition.Child
          as={Fragment}
          enter="transform transition ease-in-out duration-100 sm:duration-300"
          enterFrom="translate-x-full"
          enterTo="translate-x-0"
          leave="transform transition ease-in-out duration-100 sm:duration-300"
          leaveFrom="translate-x-0"
          leaveTo="translate-x-full"
        >
          {contentNode}
        </Transition.Child>
      </Dialog>
    </Transition.Root>
  )
}
