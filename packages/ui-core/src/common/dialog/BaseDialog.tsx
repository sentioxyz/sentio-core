import {
  type ReactElement,
  type FC,
  Fragment,
  memo,
  createContext
} from 'react'
import { Dialog, Transition } from '@headlessui/react'
import Button, { ButtonProps } from '../NewButton'
import { cx as classNames } from 'class-variance-authority'

export const BaseZIndexContext = createContext(10)

interface Props {
  title?: string | ReactElement
  titleBorder?: boolean
  footerBorder?: boolean
  children?: ReactElement
  open: boolean
  onClose: () => void
  cancelText?: string
  cancelProps?: ButtonProps
  onCancel?: () => void
  okText?: string
  okProps?: ButtonProps
  onOk?: () => void
  buttonsClassName?: string
  extraButtons?: ReactElement | ReactElement[]
  panelClassName?: string
  initialFocus?: React.MutableRefObject<HTMLElement | null> | undefined
  errorMessages?: string
  footer?: ReactElement // customize whole footer
  zIndex?: number
  mask?: 'normal' | 'light'
}

const _BaseDialog: FC<Props> = ({
  title,
  open,
  onClose,
  onCancel,
  cancelText,
  cancelProps = {},
  onOk,
  okText,
  okProps = {},
  children,
  buttonsClassName,
  panelClassName,
  titleBorder = true,
  footerBorder = true,
  initialFocus,
  extraButtons,
  errorMessages,
  footer,
  zIndex = 10,
  mask = 'normal'
}) => {
  return (
    <Transition appear as={Fragment} show={open}>
      <Dialog
        className={classNames('relative', '_sentio_')}
        as="div"
        onClose={onClose}
        initialFocus={initialFocus}
        style={{
          zIndex: zIndex
        }}
      >
        <BaseZIndexContext.Provider value={zIndex}>
          <Transition.Child
            as={Fragment}
            enter="ease-out duration-300"
            enterFrom="opacity-0"
            enterTo="opacity-100"
            leave="ease-in duration-200"
            leaveFrom="opacity-100"
            leaveTo="opacity-0"
          >
            <div
              className={classNames(
                'fixed inset-0 transition-opacity',
                mask === 'light'
                  ? 'bg-gray-500/30 dark:bg-gray-200/30'
                  : 'bg-gray-500/75 dark:bg-gray-200/50'
              )}
            />
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
                <Dialog.Panel
                  data-testid="create-dashboard"
                  className={classNames(
                    'dark:bg-sidebar relative transform overflow-hidden rounded-lg bg-white pb-4 pt-5 text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-3xl',
                    panelClassName
                  )}
                >
                  {title ? (
                    <Dialog.Title
                      as="h3"
                      className={classNames(
                        'text-ilabel font-ilabel text-text-foreground pl-4',
                        titleBorder && 'border-border-color border-b pb-4'
                      )}
                    >
                      {title}
                    </Dialog.Title>
                  ) : null}
                  {children}
                  {footer ? (
                    footer
                  ) : (
                    <div
                      className={classNames(
                        'flex items-center justify-between pt-4 ',
                        footerBorder && 'border-border-color border-t'
                      )}
                    >
                      <div
                        className="truncate px-4 text-sm font-semibold text-red-500"
                        title={errorMessages || ''}
                      >
                        {errorMessages || ' '}
                      </div>
                      <div
                        className={classNames(
                          `flex flex-row-reverse items-center gap-3 px-4`,
                          buttonsClassName ?? ''
                        )}
                      >
                        {extraButtons}
                        {onOk && (
                          <Button role="primary" onClick={onOk} {...okProps}>
                            {okText || 'OK'}
                          </Button>
                        )}
                        {onCancel && (
                          <Button onClick={onCancel} {...cancelProps}>
                            {cancelText || 'Cancel'}
                          </Button>
                        )}
                      </div>
                    </div>
                  )}
                </Dialog.Panel>
              </Transition.Child>
            </div>
          </div>
        </BaseZIndexContext.Provider>
      </Dialog>
    </Transition>
  )
}

export const BaseDialog = memo(_BaseDialog)
