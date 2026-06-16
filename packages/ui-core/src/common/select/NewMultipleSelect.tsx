import { Combobox } from '@headlessui/react'
import {
  useCallback,
  useMemo,
  useState,
  ReactElement,
  useRef,
  useEffect
} from 'react'
import { KeyboardEvent } from 'react'
import { isEqual, debounce } from 'lodash'
import { useResizeDetector } from 'react-resize-detector'
import { useLayer } from 'react-laag'
import ClipLoader from 'react-spinners/ClipLoader'
import { LuCheck, LuChevronLeft, LuChevronRight, LuX } from 'react-icons/lu'
import { classNames } from '../../utils/classnames'
// Headless UI's Combobox exposes `open` only via its render-prop. We mirror it into
// local state for react-laag, but setState during render warns and can cascade
// updates into other components — sync from an effect instead.
function SyncOpen({
  open,
  setOpen
}: {
  open: boolean
  setOpen: (v: boolean) => void
}) {
  useEffect(() => {
    setOpen(open)
  }, [open, setOpen])
  return null
}

interface TopLevelOption<T> {
  id: string
  label: string
  filterFn: (option: T) => boolean
  icon?: ReactElement
}

interface Props<T> {
  options: T[]
  value: T[]
  onChange?: (value: T[]) => void
  displayFn?: (option: T) => string | undefined
  displayIcon?: (option: T) => ReactElement | null
  filterFn?: (option: T, filter: string) => boolean
  className?: string
  labelClassName?: string
  optionsClassName?: string
  inputClassName?: string
  unSelectedText?: string
  disabled?: boolean
  maxInputSize?: number
  maxOptions?: number
  maxOptionsHeight?: number
  onSelected?: (selected: T) => void
  input?: string
  onInputChange?: (input: string) => void
  onFilterTextChange?: (filterText: string) => void
  debounceMs?: number
  validateFn?: (option: T) => boolean
  renderLabel?: (value: T, idx: number) => ReactElement
  renderOption?: (option: T, active: boolean, selected: boolean) => ReactElement
  focusOnClickLabels?: boolean
  autoOptionsWidth?: boolean
  topLevelOptions?: TopLevelOption<T>[]
  loading?: boolean
  loadingText?: string
}

export function NewMultipleSelect<T>({
  value,
  options,
  onChange,
  displayFn,
  displayIcon,
  filterFn,
  className,
  labelClassName,
  optionsClassName,
  inputClassName,
  unSelectedText,
  disabled,
  maxInputSize,
  maxOptions = 100,
  maxOptionsHeight = 240,
  onSelected,
  input,
  onInputChange,
  onFilterTextChange,
  debounceMs = 300,
  validateFn,
  renderLabel,
  renderOption,
  focusOnClickLabels = true,
  autoOptionsWidth,
  topLevelOptions = [],
  loading = false,
  loadingText = 'Loading...'
}: Props<T>) {
  const onDelete = (v: T, idx: number) => {
    const newValue = [...value]
    newValue.splice(idx, 1)
    onChange && onChange(newValue)
  }
  const [filterText, setFilterText] = useState('')
  const [activeTopLevelFilter, setActiveTopLevelFilter] =
    useState<TopLevelOption<T> | null>(null)
  const { ref, width } = useResizeDetector({
    handleHeight: false,
    handleWidth: autoOptionsWidth
  })
  const hasTopLevel = topLevelOptions.length > 0

  const filterString = useMemo(() => {
    return onInputChange ? input || '' : filterText
  }, [filterText, input, onInputChange])

  const debouncedFilterTextChange = useMemo(
    () =>
      onFilterTextChange ? debounce(onFilterTextChange, debounceMs) : undefined,
    [onFilterTextChange, debounceMs]
  )

  useEffect(() => {
    return () => {
      if (debouncedFilterTextChange) {
        debouncedFilterTextChange.cancel()
      }
    }
  }, [debouncedFilterTextChange])

  const setFilter = useCallback(
    (filter: string) => {
      if (onInputChange) {
        onInputChange(filter)
      } else {
        setFilterText(filter)
      }
      if (debouncedFilterTextChange) {
        debouncedFilterTextChange(filter)
      }
    },
    [onInputChange, setFilterText, debouncedFilterTextChange]
  )

  const filteredOptions = useMemo(() => {
    let filtered = options

    // Apply top-level filter if active
    if (activeTopLevelFilter) {
      filtered = filtered.filter(activeTopLevelFilter.filterFn)
    }

    // Apply text filter
    const fn =
      filterFn ||
      ((o: T, f: string) =>
        !!(displayFn ? displayFn(o) : `${o}`)
          ?.toLowerCase()
          .includes(f.toLowerCase()))
    filtered = filtered.filter((o) => fn(o, filterString))

    return (maxOptions || 0) > 0 ? filtered.slice(0, maxOptions) : filtered
  }, [
    options,
    filterString,
    filterFn,
    displayFn,
    maxOptions,
    activeTopLevelFilter
  ])

  const inputSize = useMemo(() => {
    let size
    if (value.length === 0) {
      const optionLabels = filteredOptions.map(
        (o) => (displayFn ? displayFn(o) : `${o}`) || ''
      )
      const minWidth = Math.max(...optionLabels.map((o) => o.length))
      const length =
        filterString == '' ? unSelectedText?.length || 1 : filterString.length
      size = Math.max(minWidth, length)
    } else {
      size = Math.max(1, filterString.length)
    }
    return (maxInputSize || 0) > 0 ? Math.min(size, maxInputSize || 0) : size
  }, [
    filterString,
    unSelectedText,
    value,
    filteredOptions,
    displayFn,
    maxInputSize
  ])

  function onInputKeyDown(
    e: KeyboardEvent<HTMLInputElement>,
    activeIndex: number | null
  ) {
    if (e.key === 'Backspace') {
      if (filterString.length === 0) {
        if (activeTopLevelFilter) {
          setActiveTopLevelFilter(null)
          setFilter('')
        } else {
          onDelete(value[value.length - 1], value.length - 1)
        }
      }
    }
    if (e.key === 'Escape') {
      ;(e.target as HTMLInputElement).blur()
      e.preventDefault()
    }
    if (
      e.key === 'Enter' &&
      activeIndex != null &&
      activeIndex >= 0 &&
      onSelected
    ) {
      onSelected(filteredOptions[activeIndex])
    }
  }

  function onValueChange(v: T[]) {
    // check if the value is a top level option
    const lastValue = v[v.length - 1]
    if (topLevelOptions.some((o) => isEqual(o, lastValue))) {
      setActiveTopLevelFilter(
        topLevelOptions.find((o) => isEqual(o, lastValue)) || null
      )
      setFilter('')
      return
    }
    if (onChange) {
      setFilter('')
      onChange(v)
    }
  }

  const isValid = useCallback(
    (l: T) => {
      return options.some((o) => isEqual(o, l))
    },
    [options, displayFn]
  )

  const labels = useMemo(() => {
    return (
      <>
        {value.map((l, idx) => {
          if (renderLabel) {
            return renderLabel(l, idx)
          } else {
            return (
              <Label
                key={idx}
                value={l}
                invalid={validateFn ? !validateFn(l) : !isValid(l)}
                onDelete={() => onDelete(l, idx)}
                displayFn={displayFn}
                className={labelClassName}
                displayIcon={displayIcon}
              />
            )
          }
        })}
      </>
    )
  }, [
    value,
    renderLabel,
    onDelete,
    displayFn,
    validateFn,
    isValid,
    labelClassName
  ])

  const [open, setOpen] = useState(false)
  const { renderLayer, triggerProps, layerProps } = useLayer({
    isOpen: open,
    auto: true,
    preferX: 'left',
    preferY: 'top',
    placement: 'bottom-start',
    triggerOffset: 4
  })

  const inputRef = useRef<HTMLInputElement>(null)
  const handleTopLevelClick = (topLevelOption: TopLevelOption<T>) => {
    setActiveTopLevelFilter(topLevelOption)
    setFilter('')
    inputRef.current?.focus()
  }

  const handleBackClick = (evt: React.MouseEvent<HTMLButtonElement>) => {
    evt.stopPropagation()
    evt.preventDefault()
    setActiveTopLevelFilter(null)
    setFilter('')
    inputRef.current?.focus()
  }

  return (
    <Combobox
      as="div"
      className={classNames(
        className,
        'focus-within:border-primary-600 focus-within:ring-3 focus-within:ring-primary-600/30'
      )}
      value={value}
      onChange={onValueChange}
      disabled={disabled}
      multiple
      nullable
    >
      {({ activeIndex, open }) => {
        return (
          <div
            className="focus-within:ring-primary-500 relative inline-flex w-full grow"
            ref={ref}
          >
            <SyncOpen open={open} setOpen={setOpen} />
            {!focusOnClickLabels && (
              <div
                className={classNames(
                  'flex items-center gap-1 border-0 p-0 px-1',
                  disabled ? 'bg-gray-100' : 'bg-default-bg'
                )}
              >
                {labels}
              </div>
            )}
            <Combobox.Button
              as="div"
              className={classNames('flex h-full w-full items-center')}
              {...triggerProps}
            >
              <Combobox.Input
                onChange={() => {
                  /**/
                }}
                as="div"
                className={classNames(
                  'inline-flex h-full w-full flex-wrap items-center gap-1 border-0 p-0 align-text-top',
                  disabled ? 'bg-gray-100' : 'bg-default-bg'
                )}
              >
                {focusOnClickLabels && labels}
                {hasTopLevel ? (
                  <div className="relative w-full">
                    {activeTopLevelFilter && (
                      <button
                        type="button"
                        onClick={handleBackClick}
                        className="text-text-foreground-secondary hover:text-text-foreground absolute left-0 top-1/2 inline-flex -translate-y-1/2 items-center px-1 py-1"
                      >
                        <LuChevronLeft className="h-4.5 w-4.5" />
                      </button>
                    )}
                    <input
                      disabled={disabled}
                      onChange={(e) => setFilter(e.target.value)}
                      value={filterString}
                      size={inputSize}
                      placeholder={
                        value.length == 0 || filterString.length == 0
                          ? activeTopLevelFilter
                            ? `Search ${activeTopLevelFilter.label}...`
                            : unSelectedText
                          : ''
                      }
                      onKeyDown={(e) => onInputKeyDown(e, activeIndex)}
                      className={classNames(
                        'text-icontent focus:outline-hidden ml-1 h-full min-w-fit pr-6',
                        disabled
                          ? 'dark:bg-default! bg-gray-100!'
                          : 'bg-default-bg',
                        activeTopLevelFilter ? 'pl-6' : '',
                        inputClassName
                      )}
                      autoComplete="off"
                      ref={inputRef}
                    />
                  </div>
                ) : (
                  <input
                    disabled={disabled}
                    onChange={(e) => setFilter(e.target.value)}
                    value={filterString}
                    size={inputSize}
                    placeholder={value.length == 0 ? unSelectedText : ''}
                    onKeyDown={(e) => onInputKeyDown(e, activeIndex)}
                    className={classNames(
                      'text-icontent focus:outline-hidden ml-1 h-full min-w-fit pr-6',
                      disabled
                        ? 'dark:bg-default! bg-gray-100!'
                        : 'bg-default-bg',
                      inputClassName
                    )}
                    autoComplete="off"
                    ref={inputRef}
                  />
                )}
              </Combobox.Input>
            </Combobox.Button>
            {open &&
              renderLayer(
                <div
                  {...layerProps}
                  style={{ ...layerProps.style, zIndex: 20 }}
                >
                  <div className="ring-border-color shadow-xs rounded ring-1">
                    <Combobox.Options
                      className={classNames(
                        'text-icontent focus:outline-hidden bg-default-bg overflow-auto py-1 sm:text-sm',
                        'scrollbar-thin rounded-sm',
                        optionsClassName
                      )}
                      style={{
                        maxHeight: maxOptionsHeight,
                        width: autoOptionsWidth ? width : undefined
                      }}
                    >
                      {/* Show top-level options only when no top-level filter is active */}
                      {!activeTopLevelFilter && topLevelOptions.length > 0 && (
                        <>
                          {topLevelOptions.map((topLevelOption, idx) => (
                            <Combobox.Option
                              value={topLevelOption}
                              key={`top-${topLevelOption.id}`}
                              className={({ active }) =>
                                classNames(
                                  'relative cursor-pointer select-none px-3 py-2 text-xs',
                                  active ? 'bg-hover' : ''
                                )
                              }
                              onClick={() =>
                                handleTopLevelClick(topLevelOption)
                              }
                            >
                              <div className="flex items-center gap-2">
                                {topLevelOption.icon && topLevelOption.icon}
                                <span className="block truncate font-medium">
                                  {topLevelOption.label}
                                </span>
                                <span className="flex flex-1 justify-end">
                                  <LuChevronRight className="text-text-foreground-secondary h-4 w-4" />
                                </span>
                              </div>
                            </Combobox.Option>
                          ))}
                          {filteredOptions.length > 0 && (
                            <div className="border-border-color my-1 border-t" />
                          )}
                        </>
                      )}
                      {/* Show loading state */}
                      {loading && (
                        <div className="relative cursor-default select-none px-3 py-4 text-center">
                          <div className="flex items-center justify-center gap-2">
                            <ClipLoader size={16} color="#A6C2F0" />
                            <span className="text-text-foreground-secondary text-xs">
                              {loadingText}
                            </span>
                          </div>
                        </div>
                      )}
                      {/* Show options when not loading */}
                      {!loading &&
                        filteredOptions.map((m, idx) => {
                          const text = displayFn ? displayFn(m) : `${m}`
                          return (
                            <Combobox.Option
                              key={idx}
                              title={text}
                              value={m}
                              className={({ active }) =>
                                classNames(
                                  'relative cursor-default select-none truncate py-2 pl-3 text-xs',
                                  active
                                    ? 'bg-primary-50 text-primary dark:bg-primary-600 dark:text-white'
                                    : 'text-text-foreground',
                                  autoOptionsWidth ? '' : 'max-w-sm',
                                  'pr-10'
                                )
                              }
                              onClick={() => onSelected && onSelected(m)}
                            >
                              {({ active, selected }) => {
                                if (renderOption) {
                                  return renderOption(m, active, selected)
                                } else
                                  return (
                                    <>
                                      <span
                                        title={text}
                                        className={classNames(
                                          'block truncate',
                                          selected && 'font-medium'
                                        )}
                                      >
                                        {text}
                                      </span>

                                      {selected && (
                                        <span
                                          className={classNames(
                                            'absolute inset-y-0 right-0 flex items-center pr-4'
                                          )}
                                        >
                                          <LuCheck
                                            className="h-4 w-4"
                                            aria-hidden="true"
                                          />
                                        </span>
                                      )}
                                    </>
                                  )
                              }}
                            </Combobox.Option>
                          )
                        })}
                      {/* Show "No options" message when not loading and no options available */}
                      {!loading &&
                        filteredOptions.length === 0 &&
                        !activeTopLevelFilter &&
                        topLevelOptions.length === 0 && (
                          <div className="relative cursor-default select-none px-3 py-2 text-center">
                            <span className="text-text-foreground-secondary text-xs">
                              No options available
                            </span>
                          </div>
                        )}
                      {!loading &&
                        filteredOptions.length === 0 &&
                        activeTopLevelFilter && (
                          <div className="relative cursor-default select-none px-3 py-2 text-center">
                            <span className="text-text-foreground-secondary text-xs">
                              No results found in {activeTopLevelFilter.label}
                            </span>
                          </div>
                        )}
                    </Combobox.Options>
                  </div>
                </div>
              )}
          </div>
        )
      }}
    </Combobox>
  )
}

type LabelProps<T> = {
  value: T
  invalid: boolean
  onDelete: (value: T) => void
  displayFn?: (value: T) => string | undefined
  displayIcon?: (value: T) => ReactElement | null
  className?: string
}

function Label<T>({
  value,
  onDelete,
  displayFn,
  displayIcon,
  invalid,
  className
}: LabelProps<T>) {
  const title = displayFn ? displayFn(value) : `${value}`
  return (
    <span
      className={classNames(
        invalid
          ? 'bg-red-100 text-red-700'
          : 'text-primary-700 bg-primary-100/50 hover:bg-primary-100 dark:bg-primary-200 dark:hover:bg-primary-300 dark:text-white',
        'inline-flex h-full max-w-xs items-center rounded py-0.5 pl-2 pr-0.5 text-xs font-medium',
        className
      )}
    >
      {displayIcon && displayIcon(value)}
      <span title={title} className="truncate">
        {title}
      </span>
      <div
        onClick={() => onDelete(value)}
        className={classNames(
          invalid
            ? 'text-red-400 hover:bg-red-200 hover:text-red-500'
            : 'text-primary-400 hover:bg-primary-200 hover:text-primary-500 dark:hover:bg-sentio-gray-500 dark:text-white/50 dark:hover:text-white',
          'ml-0.5 inline-flex h-4 w-4 shrink-0 cursor-pointer items-center justify-center rounded-full  '
        )}
      >
        <span className="sr-only">Remove</span>
        <LuX className="h-3 w-3" />
      </div>
    </span>
  )
}
