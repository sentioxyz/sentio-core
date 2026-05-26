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
          <div className="fixed inset-0 transition-opacity bg-gray-200/40 dark:bg-gray-200/50" />
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
              <Dialog.Panel className="border border-main bg-default-bg relative transform overflow-hidden rounded-lg text-left shadow-xs transition-all sm:my-8 sm:w-full sm:max-w-lg">
                <div className="px-4 pb-4 pt-5 sm:p-6 sm:pb-4">
                  <div className="sm:flex sm:items-start">
                    {type == 'danger' && (
                      <div className="mx-auto flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-red-100 sm:mx-0 sm:h-10 sm:w-10">
                        <ExclamationIcon
                          className="h-6 w-6 text-red-600"
                          aria-hidden="true"
                        />
                      </div>
                    )}
                    {type == 'question' && (
                      <div className="bg-primary-100 mx-auto flex h-12 w-12 shrink-0 items-center justify-center rounded-full sm:mx-0 sm:h-10 sm:w-10">
                        <QuestionMarkCircleIcon
                          className="text-primary-600 h-6 w-6"
                          aria-hidden="true"
                        />
                      </div>
                    )}
                    <div className="mt-3 text-center sm:ml-4 sm:mt-0 sm:text-left">
                      <Dialog.Title
                        as="h3"
                        className="text-text-foreground text-base font-medium leading-6"
                      >
                        {title}
                      </Dialog.Title>
                      <div className="mt-2">
                        {message && (
                          <p className="text-icontent text-text-foreground-secondary">{message}</p>
                        )}
                        {children}
                      </div>
                    </div>
                  </div>
                </div>
                <div className="flex gap-2 px-4 py-3 sm:flex-row-reverse sm:px-6">
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
