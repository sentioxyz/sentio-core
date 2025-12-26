import { Listbox } from '@headlessui/react'
import { CheckIcon, ChevronDownIcon } from '@heroicons/react/20/solid'
import { classNames } from '../../utils/classnames'
import { isEqual } from 'lodash'
import { cva } from 'class-variance-authority'
import {
  ReactElement,
  ReactNode,
  useMemo,
  useRef,
  useState,
  useContext,
  Key
} from 'react'
import isFunction from 'lodash/isFunction'
import { FieldError } from 'react-hook-form'
import { ClipLoader } from 'react-spinners'
import { useLayer } from 'react-laag'
import { useResizeDetector } from 'react-resize-detector'
import { BaseZIndexContext } from '../dialog/BaseDialog'

const buttonClass = cva(
  [
    'focus:ring-primary focus:border-primary',
    'relative w-full rounded-md border bg-white dark:bg-sentio-gray-100 text-left focus:outline-none focus:ring-1 text-ilabel'
  ],
  {
    variants: {
      open: {
        true: 'bg-sentio-gray-100 ring-1 ring-primary border-primary',
        false: ''
      },
      size: {
        sm: 'py-1 px-2',
        md: 'py-2 px-3'
      },
      error: {
        true: 'border-red-300 text-red-900 placeholder-red-300 focus-within:ring-red-500',
        false: 'border-border-color'
      },
      disabled: {
        true: 'cursor-not-allowed',
        false: 'cursor-default'
      }
    },
    defaultVariants: {
      open: false,
      size: 'sm',
      error: false,
      disabled: false
    },
    compoundVariants: [
      {
        open: true,
        error: true,
        class: '!ring-red-300 border-red-300'
      }
    ]
  }
)

const optionClass = cva(['relative cursor-default select-none'], {
  variants: {
    disabled: {
      true: 'cursor-not-allowed text-gray-400',
      false: 'text-text-foreground'
    },
    size: {
      sm: 'py-1 pl-3 pr-5',
      md: 'py-2 pl-3 pr-6'
    },
    active: {
      true: 'bg-primary-50 dark:bg-primary-400/50',
      false: ''
    },
    selected: {
      true: '!bg-primary-100 dark:!bg-primary-400',
      false: ''
    }
  },
  defaultVariants: {
    disabled: false,
    active: false,
    selected: false,
    size: 'sm'
  }
})

const iconClass = cva([], {
  variants: {
    size: {
      sm: 'h-3.5 w-3.5',
      md: 'h-4 w-4'
    },
    disabled: {
      true: 'opacity-50',
      false: ''
    }
  },
  defaultVariants: {
    size: 'sm',
    disabled: false
  }
})

type labelProps = {
  selected?: boolean
  active?: boolean
}

export interface IOption<T> {
  label: ReactNode | ((props: labelProps) => ReactNode)
  value: T
  disabled?: boolean
  title?: string
}

export interface SelectProps<T> {
  options: IOption<T>[]
  value: T
  onChange: (value: T) => void
  className?: string
  buttonClassName?: string
  optionsClassName?: string
  placeholder?: string
  size?: 'sm' | 'md'
  renderOption?: (option: IOption<T>, state: labelProps) => ReactElement
  noOptionsMessage?: ReactElement
  error?: FieldError
  disabled?: boolean
  fetchMore?: () => void
  isFetchingMore?: boolean
  scrollBottomThreshold?: number
  groupedOptions?: boolean
  groupedOrder?: {
    key: string
    label: string
  }[]
  unmountOptions?: boolean
  direction?: 'up' | 'down'
  asLayer?: boolean
}

function generateLabel(
  label: ReactNode | ((props: labelProps) => ReactNode),
  props: labelProps
) {
  if (isFunction(label)) {
    return label(props)
  }
  return label
}

export function Select<T>({
  className,
  buttonClassName,
  optionsClassName,
  options,
  value,
  onChange,
  placeholder,
  size = 'sm',
  renderOption,
  noOptionsMessage,
  error,
  disabled,
  fetchMore,
  isFetchingMore,
  scrollBottomThreshold = 100,
  groupedOptions,
  groupedOrder,
  unmountOptions = true,
  direction = 'down',
  asLayer
}: SelectProps<T>) {
  const selectedIndex = options.findIndex((o) => isEqual(o.value, value))
  const listRef = useRef<HTMLUListElement>(null)
  const [open, setOpen] = useState(false)
  const { width, ref } = useResizeDetector({
    refreshMode: 'debounce',
    refreshRate: 100,
    handleHeight: false
  })
  const baseZIndex = useContext(BaseZIndexContext)

  const { renderLayer, triggerProps, layerProps } = useLayer({
    isOpen: open,
    auto: true,
    preferX: 'left',
    preferY: direction === 'up' ? 'top' : 'bottom',
    placement: direction === 'up' ? 'top-start' : 'bottom-start',
    triggerOffset: 4,
    onOutsideClick: () => setOpen(false)
  })

  const grouped = useMemo(() => {
    if (!groupedOptions || !options || options.length === 0) {
      return options
    }
    const groupedOptionsList = options.reduce((acc: any, option: any) => {
      ;(acc[option.group] = acc[option.group] || []).push(option)
      return acc
    }, {})
    return groupedOrder?.reduce((acc: any, group: any) => {
      return [
        ...acc,
        {
          label: group.label,
          options: groupedOptionsList[group.key] || []
        }
      ]
    }, [])
  }, [groupedOptions, groupedOrder, options])

  const optionsElement = (
    <Listbox.Options
      ref={listRef}
      onScroll={() => {
        if (listRef.current?.scrollHeight) {
          const bottomHeight =
            listRef.current?.scrollHeight -
            listRef.current?.clientHeight -
            listRef.current?.scrollTop
          if (bottomHeight < scrollBottomThreshold) {
            fetchMore?.()
          }
        }
      }}
      unmount={unmountOptions}
      className={classNames(
        'text-ilabel dark:bg-sentio-gray-100 scrollbar-thin max-h-60 w-full overflow-auto rounded-md bg-white py-1 shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none dark:ring-gray-100 sm:text-sm',
        asLayer ? '' : 'absolute z-10',
        optionsClassName
      )}
    >
      {!options || options.length === 0 ? (
        <Listbox.Option
          value={null}
          disabled
          className={optionClass({ disabled: true, size })}
        >
          {noOptionsMessage ?? (
            <span className="text-gray-400">No options</span>
          )}
        </Listbox.Option>
      ) : null}
      {groupedOptions && grouped
        ? grouped.map(({ label, options }) => {
            if (!options || options.length === 0) {
              return null
            }
            return (
              <div key={label}>
                <div className="text-gray px-3.5 py-1 text-xs font-medium">
                  {label}
                </div>
                <div>
                  {options.map(
                    (option: IOption<T>, i: Key | null | undefined) => (
                      <Listbox.Option
                        key={i}
                        value={option.value}
                        disabled={option.disabled}
                        className={({ active }) =>
                          optionClass({
                            disabled: option.disabled,
                            size,
                            active,
                            selected: isEqual(value, option.value)
                          })
                        }
                        title={option.title}
                      >
                        {({ selected, active }) => {
                          if (renderOption) {
                            return renderOption(option, { selected, active })
                          }
                          return (
                            <>
                              <span
                                className={classNames(
                                  selected ? 'font-medium' : 'font-normal',
                                  'text-ilabel block truncate'
                                )}
                              >
                                {generateLabel(option.label, {
                                  selected,
                                  active
                                })}
                              </span>

                              {selected ? (
                                <span
                                  className={classNames(
                                    'text-primary-600 absolute inset-y-0 right-0 flex items-center pr-2 dark:text-white'
                                  )}
                                >
                                  <CheckIcon
                                    className={iconClass({ size })}
                                    aria-hidden="true"
                                  />
                                </span>
                              ) : null}
                            </>
                          )
                        }}
                      </Listbox.Option>
                    )
                  )}
                </div>
              </div>
            )
          })
        : options.map((option, i) => (
            <Listbox.Option
              key={i}
              value={option.value}
              disabled={option.disabled}
              className={({ active }) =>
                optionClass({
                  disabled: option.disabled,
                  size,
                  active,
                  selected: selectedIndex === i
                })
              }
              title={option.title}
            >
              {({ selected, active }) => {
                if (renderOption) {
                  return renderOption(option, { selected, active })
                }
                return (
                  <>
                    <span
                      className={classNames(
                        selected ? 'font-medium' : 'font-normal',
                        'text-ilabel block truncate'
                      )}
                    >
                      {generateLabel(option.label, { selected, active })}
                    </span>

                    {selected ? (
                      <span
                        className={classNames(
                          'text-primary-600 absolute inset-y-0 right-0 flex items-center pr-2 dark:text-white'
                        )}
                      >
                        <CheckIcon
                          className={iconClass({ size })}
                          aria-hidden="true"
                        />
                      </span>
                    ) : null}
                  </>
                )
              }}
            </Listbox.Option>
          ))}
      {isFetchingMore && (
        <Listbox.Option
          value={null}
          disabled
          className={optionClass({ disabled: true, size })}
        >
          <div className="flex w-full items-center justify-center gap-2">
            <ClipLoader size={16} color="#A6C2F0" />
            <span className="text-gray/50 text-xs">Loading more...</span>
          </div>
        </Listbox.Option>
      )}
    </Listbox.Options>
  )

  return (
    <Listbox value={value} onChange={onChange} disabled={disabled}>
      {({ open: headlessOpen }) => {
        // Sync the internal state with Headless UI state
        if (headlessOpen !== open && asLayer) {
          setOpen(headlessOpen)
        }

        return (
          <div className={classNames(className, 'relative')} ref={ref}>
            <Listbox.Button
              as="div"
              className={classNames(
                buttonClass({
                  open: headlessOpen,
                  size,
                  error: !!error,
                  disabled: !!disabled
                }),
                buttonClassName
              )}
              {...triggerProps}
            >
              <div className="pr-4">
                {selectedIndex > -1 ? (
                  generateLabel(options[selectedIndex].label, {})
                ) : placeholder ? (
                  <span
                    className={classNames(
                      'font-normal',
                      error ? 'text-red-400' : 'text-gray-400'
                    )}
                  >
                    {placeholder}
                  </span>
                ) : (
                  ''
                )}
              </div>
              <button
                type="button"
                disabled={disabled}
                className={classNames(
                  'absolute inset-y-0 right-0 flex items-center rounded-r-md px-2 focus:outline-none'
                )}
              >
                <ChevronDownIcon
                  className={iconClass({ size, disabled: !!disabled })}
                />
              </button>
            </Listbox.Button>

            {headlessOpen
              ? asLayer
                ? renderLayer(
                    <div
                      {...layerProps}
                      style={{
                        ...layerProps.style,
                        zIndex: Math.max(10, baseZIndex + 1),
                        minWidth: width
                      }}
                    >
                      {optionsElement}
                    </div>
                  )
                : optionsElement
              : null}

            {error && (
              <p className="mt-2 text-xs font-medium text-red-600">
                {typeof error == 'string' ? error : error.message}
              </p>
            )}
          </div>
        )
      }}
    </Listbox>
  )
}
