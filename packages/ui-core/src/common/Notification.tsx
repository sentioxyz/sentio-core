import { Fragment, ReactNode } from 'react'
import { Transition } from '@headlessui/react'
import { XMarkIcon as XIcon } from '@heroicons/react/20/solid'
import {
  CheckCircleIcon,
  ExclamationCircleIcon,
  InformationCircleIcon
} from '@heroicons/react/24/outline'

interface Props {
  show: boolean
  setShow: (show: boolean) => void
  title: string
  message: string
  type: 'success' | 'error' | 'warning' | 'info'
  buttons?: () => ReactNode
}

export function Notification({
  show,
  setShow,
  title,
  message,
  buttons,
  type
}: Props) {
  let icon: ReactNode
  switch (type) {
    case 'success':
      icon = (
        <CheckCircleIcon
          className="h-6 w-6 text-green-400"
          aria-hidden="true"
        />
      )
      break
    case 'error':
      icon = (
        <ExclamationCircleIcon
          className="h-6 w-6 text-red-400"
          aria-hidden="true"
        />
      )
      break
    case 'warning':
      icon = (
        <ExclamationCircleIcon
          className="h-6 w-6 text-yellow-400"
          aria-hidden="true"
        />
      )
      break
    case 'info':
      icon = (
        <InformationCircleIcon
          className="h-6 w-6 text-blue-400"
          aria-hidden="true"
        />
      )
      break
  }

  return (
    <>
      <div
        aria-live="assertive"
        className="pointer-events-none fixed inset-0 z-40 flex items-end px-4 py-6 sm:items-start sm:p-6"
        onClick={(evt) => {
          // To prevent triggger slideOver's onClose
          evt.stopPropagation()
        }}
      >
        <div className="flex w-full flex-col items-center space-y-4 sm:items-end">
          <Transition
            show={show}
            as={Fragment}
            enter="transform ease-out duration-300 transition"
            enterFrom="translate-y-2 opacity-0 sm:translate-y-0 sm:translate-x-2"
            enterTo="translate-y-0 opacity-100 sm:translate-x-0"
            leave="transition ease-in duration-100"
            leaveFrom="opacity-100"
            leaveTo="opacity-0"
          >
            <div className="dark:bg-sentio-gray-100 pointer-events-auto w-full max-w-sm rounded-lg bg-white shadow-lg ring-1 ring-black ring-opacity-5 dark:ring-gray-100">
              <div className="p-4">
                <div className="flex items-start">
                  <div className="flex-shrink-0">{icon}</div>
                  <div className="ml-3 w-0 flex-1">
                    <p className="text-text-foreground text-sm font-medium">
                      {title}
                    </p>
                    <p className="mt-1 text-sm text-gray-500">{message}</p>
                    {buttons && <div className="mt-4 flex">{buttons()}</div>}
                  </div>
                  <div className="ml-4 flex flex-shrink-0">
                    <button
                      type="button"
                      className="focus:ring-primary-500 dark:bg-sentio-gray-100 inline-flex rounded-md bg-white text-gray-400 hover:text-gray-500 focus:outline-none focus:ring-2 focus:ring-offset-2"
                      onClick={() => {
                        setShow(false)
                      }}
                    >
                      <span className="sr-only">Close</span>
                      <XIcon className="h-5 w-5" aria-hidden="true" />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </Transition>
        </div>
      </div>
    </>
  )
}
