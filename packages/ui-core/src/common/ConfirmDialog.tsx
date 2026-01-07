import { Fragment, useRef, useState, ReactNode } from 'react'
import { Dialog, Transition } from '@headlessui/react'
import {
  ExclamationCircleIcon as ExclamationIcon,
  QuestionMarkCircleIcon
} from '@heroicons/react/24/outline'
import { NewButton as Button } from './NewButton'

interface Props {
  message?: string
  title: string
  open: boolean
  onClose: (open: boolean) => void
  onConfirm: () => void | Promise<void>
  disabled?: boolean
  buttonLabel?: string
  type: 'danger' | 'question'
  buttons?: ReactNode
  children?: ReactNode
}

export function ConfirmDialog({
  message,
  title,
  open,
  onClose,
  onConfirm,
  buttonLabel = 'Delete',
  type,
  buttons,
  children,
  disabled
}: Props) {
  const cancelButtonRef = useRef(null)
  const [processing, setProcessing] = useState(false)

  async function confirm() {
    setProcessing(true)
    try {
      await onConfirm()
    } finally {
      setProcessing(false)
      onClose(false)
    }
  }

  return (
    <Transition.Root show={open} as={Fragment}>
      <Dialog
        as="div"
        className="relative z-10"
        aria-label="confirm"
        initialFocus={cancelButtonRef}
        onClose={onClose}
      >
        <Transition.Child
          as={Fragment}
          enter="ease-out duration-300"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-200"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div className="fixed inset-0 bg-gray-500/75 transition-opacity dark:bg-gray-200/50" />
        </Transition.Child>

        <div className="fixed inset-0 z-10 overflow-y-auto">
          <div className="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0">
            <Transition.Child
              as={Fragment}
              enter="ease-out duration-300"
              enterFrom="opacity-0 translate-y-4 sm:translate-y-0 sm:scale-95"
              enterTo="opacity-100 translate-y-0 sm:scale-100"
              leave="ease-in duration-200"
              leaveFrom="opacity-100 translate-y-0 sm:scale-100"
              leaveTo="opacity-0 translate-y-4 sm:translate-y-0 sm:scale-95"
            >
              <Dialog.Panel className="dark:bg-sentio-gray-100 relative transform overflow-hidden rounded-lg bg-white text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-lg">
                <div className="dark:bg-sentio-gray-100 bg-white px-4 pb-4 pt-5 sm:p-6 sm:pb-4">
                  <div className="sm:flex sm:items-start">
                    {type == 'danger' && (
                      <div className="mx-auto flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full bg-red-100 dark:bg-red-300 sm:mx-0 sm:h-10 sm:w-10">
                        <ExclamationIcon
                          className="h-6 w-6 text-red-600 dark:text-red-800"
                          aria-hidden="true"
                        />
                      </div>
                    )}
                    {type == 'question' && (
                      <div className="bg-primary-100 dark:bg-primary-500 mx-auto flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-full sm:mx-0 sm:h-10 sm:w-10">
                        <QuestionMarkCircleIcon
                          className="text-primary-600 dark:text-primary-800 h-6 w-6"
                          aria-hidden="true"
                        />
                      </div>
                    )}
                    <div className="mt-3 text-center sm:ml-4 sm:mt-0 sm:text-left">
                      <Dialog.Title
                        as="h3"
                        className="text-text-foreground text-lg font-medium leading-6"
                      >
                        {title}
                      </Dialog.Title>
                      <div className="mt-2">
                        {message && (
                          <p className="text-sm text-gray-500">{message}</p>
                        )}
                        {children}
                      </div>
                    </div>
                  </div>
                </div>
                <div className="flex gap-2 bg-gray-50 px-4 py-3 sm:flex-row-reverse sm:px-6">
                  {buttons ? (
                    buttons
                  ) : (
                    <>
                      <Button
                        type="button"
                        processing={processing}
                        status={type == 'danger' ? 'danger' : undefined}
                        role="primary"
                        onClick={() => confirm()}
                        disabled={disabled}
                        size="lg"
                      >
                        {buttonLabel}
                      </Button>
                      <Button
                        type="button"
                        role="secondary"
                        onClick={() => onClose(false)}
                        ref={cancelButtonRef}
                        size="lg"
                      >
                        Cancel
                      </Button>
                    </>
                  )}
                </div>
              </Dialog.Panel>
            </Transition.Child>
          </div>
        </div>
      </Dialog>
    </Transition.Root>
  )
}
