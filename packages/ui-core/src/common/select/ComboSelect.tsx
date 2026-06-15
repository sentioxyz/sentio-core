import { Combobox } from '@headlessui/react'
import { classNames } from '../../utils/classnames'
import { BiCaretDown } from 'react-icons/bi'
import { useMemo, useRef, useState, useCallback } from 'react'
import { CopyButton } from '../CopyButton'
import { LuSquare, LuSquareCheck, LuX as XIcon } from 'react-icons/lu'

/** True when a value is meaningfully set (not undefined/null/empty string). */
function hasValue<T>(val: T | undefined): val is T {
  return val !== undefined && val !== null && (val as unknown) !== ''
}

export interface Props<T> {
  options: T[]
  value?: T
  onChange: (value?: T) => void
  error?: any
  display?: (option: T) => string | undefined
  renderIcon?: (option?: T) => React.ReactNode
  className?: string
  inputClassName?: string
  placeholder?: string
  maxOptions?: number
  label?: string
  defaultValue?: T
  hideClearButton?: boolean
  filterFn?: (options: T[], term: string) => T[]
  input?: string
  onInputChange?: (input: string) => void
  filterInputClassName?: string
  supportCopy?: boolean
  optionClassName?: string
  noCheckBox?: boolean
  maxOptionHeight?: number
  readonly?: boolean
}

function defaultFilterFn<T>(
  options: T[],
  term: string,
  display?: (option: T) => string | undefined
) {
  return options.filter((o) =>
    (display ? display(o) : String(o))
      ?.toLowerCase()
      .includes(term?.toLowerCase())
  )
}

// headlessui types Combobox.Input as a native <input>; here it is rendered
// `as="div"` with custom children/handlers, which is valid at runtime but not
// expressible in its prop types. Cast to bypass typing for this single usage.
const ComboboxInput = Combobox.Input as unknown as (
  props: any
) => React.ReactElement

export function ComboSelect<T>({
  options,
  value,
  onChange,
  error,
  display,
  renderIcon,
  className,
  inputClassName,
  placeholder,
  maxOptions,
  label,
  defaultValue,
  filterFn,
  input: inputFromProps,
  onInputChange,
  filterInputClassName,
  supportCopy,
  optionClassName,
  noCheckBox,
  maxOptionHeight = 240,
  readonly
}: Props<T>) {
  const isControlled = typeof inputFromProps != 'undefined'

  const [internalInput, setInternalInput] = useState<string>('')
  const input = isControlled ? inputFromProps : internalInput
  const setInput = (v?: string) => {
    if (onInputChange) {
      onInputChange(v ?? '')
    }

    if (!isControlled) {
      setInternalInput(v ?? '')
    }
  }

  const inputRef = useRef<HTMLInputElement>(null)

  const filteredOptions = useMemo(() => {
    const filtered = input
      ? filterFn
        ? filterFn(options, input)
        : defaultFilterFn(options, input, display)
      : options
    return maxOptions || 0 > 0 ? filtered.slice(0, maxOptions) : filtered
  }, [options, input, filterFn, maxOptions, display])

  const clearSelection = useCallback(
    function clearSelection(evt: React.MouseEvent<HTMLDivElement>) {
      evt.stopPropagation()
      onChange(undefined)
      setInput('')
    },
    [onChange]
  )

  const currentValue = hasValue(value)
    ? display
      ? display(value)
      : String(value)
    : hasValue(defaultValue)
      ? display
        ? display(defaultValue)
        : String(defaultValue)
      : placeholder

  return (
    <div className={'relative w-full'}>
      {label && (
        <label
          className="text-text-foreground-secondary bg-default-bg absolute -top-2.5 left-2 px-0.5 text-[10px]"
          style={{ zIndex: 1 }}
        >
          {label}
        </label>
      )}
      <Combobox
        defaultValue={value || defaultValue}
        onChange={(value) => {
          onChange(value)
          setInput(undefined)
        }}
        disabled={readonly}
      >
        <div
          className={classNames(
            className,
            'hover:border-primary-600 group flex grow border',
            inputClassName,
            'relative w-full items-center',
            error
              ? 'focus:ring-3 border-red-600 focus:border-red-600 focus:ring-red-600/30'
              : 'focus:border-primary-600 focus:ring-3 focus:ring-primary-600/30 border-border-color'
          )}
        >
          <Combobox.Button as="div" className="flex w-full">
            <ComboboxInput
              as="div"
              tabIndex={0}
              placeholder={placeholder}
              className={classNames(
                'text-icontent focus:outline-hidden w-[calc(100%-6rem)] grow border-0 py-1 pl-2 pr-0 ring-0 focus:ring-0'
              )}
              onClick={() => {
                setTimeout(() => {
                  inputRef.current?.focus()
                }, 0)
              }}
              onFocus={() => inputRef.current?.focus()}
              onChange={(event: React.ChangeEvent<HTMLInputElement>) =>
                setInput(event.target.value)
              }
            >
              <div
                className={classNames(
                  hasValue(value)
                    ? 'text-text-foreground'
                    : 'text-text-foreground-disabled',
                  'left-0 w-full cursor-pointer truncate'
                )}
              >
                {renderIcon ? renderIcon(value || defaultValue) : null}
                {currentValue}
              </div>
              <div className="h-0 text-sm opacity-0">{label} </div>
            </ComboboxInput>
            {defaultValue == null && hasValue(value) ? (
              <div
                role={'button'}
                aria-label={'clear'}
                className={classNames(
                  'invisible inset-y-0 flex cursor-pointer items-center p-1 group-hover:visible'
                )}
                onClick={clearSelection}
              >
                <XIcon
                  className="hover:bg-primary-400 text-text-foreground-disabled mr-2 h-4 w-4 rounded-lg hover:text-white"
                  aria-hidden="true"
                />
              </div>
            ) : (
              <div
                role={'button'}
                aria-label={'clear'}
                className={classNames(
                  'inset-y-0 flex cursor-pointer items-center p-1'
                )}
              >
                <BiCaretDown
                  className="group-hover:text-text-foreground-secondary text-text-foreground-disabled mr-2 h-4 w-4"
                  aria-hidden="true"
                />
              </div>
            )}
          </Combobox.Button>

          <Combobox.Options
            as="div"
            className={classNames(
              'ring-primary-600 focus:ring-primary-600 divide-primary-600 bg-default-bg absolute z-10 min-w-full gap-0 divide-y rounded-sm text-sm shadow-lg ring-1',
              label
                ? 'left-px top-[2px] -translate-x-px translate-y-[-2px] py-1'
                : 'left-0 top-0 py-0.5',
              'max-w-[80vw] sm:max-w-sm xl:max-w-md',
              optionClassName
            )}
          >
            <div>
              <label
                className="text-text-foreground-secondary bg-default-bg absolute left-[7px] top-[-11px] px-0.5 text-[10px]"
                style={{ zIndex: 1 }}
              >
                {label}
              </label>
              <input
                type="text"
                ref={inputRef}
                className={classNames(
                  'text-icontent focus:outline-hidden mb-0.5 h-6 w-full border-0 px-2 py-0 ring-0 focus:ring-0',
                  filterInputClassName
                )}
                tabIndex={0}
                onChange={(e) => setInput(e.target.value)}
                value={input ?? ''}
                autoComplete="off"
                placeholder={currentValue}
              />
            </div>
            <ul
              className="overflow-auto pt-1"
              style={{
                maxHeight: `${maxOptionHeight}px`
              }}
            >
              {filteredOptions.map((m, idx) => (
                <Combobox.Option key={idx} value={m}>
                  {({ active }) => {
                    const text = (display ? display(m) : String(m)) || '(empty)'
                    const currentSelection = hasValue(value)
                      ? value
                      : defaultValue
                    const selected = m === currentSelection
                    return (
                      <div
                        className={classNames(
                          'text-icontent relative cursor-default select-none py-2 pl-2',
                          selected
                            ? 'bg-primary-600 text-white'
                            : active
                              ? 'bg-hover text-primary-600'
                              : 'text-text-foreground-secondary',
                          supportCopy ? 'group/option pr-7' : 'pr-2'
                        )}
                      >
                        <span
                          title={text}
                          className={classNames(
                            'block truncate pl-4',
                            selected && 'font-medium'
                          )}
                        >
                          {renderIcon ? renderIcon(m) : null}
                          {text}
                          {supportCopy ? (
                            <span
                              className="absolute right-1 top-2 hidden group-hover/option:block"
                              onClick={(evt) => {
                                evt.preventDefault()
                                evt.stopPropagation()
                              }}
                            >
                              <CopyButton text={text} size={14} />
                            </span>
                          ) : null}
                        </span>
                        {!noCheckBox && (
                          <span
                            className={classNames(
                              'absolute inset-y-0 left-2 flex items-center pr-4',
                              active ? 'text-white' : 'text-primary-600'
                            )}
                          >
                            <span>
                              {selected ? (
                                <LuSquareCheck
                                  className={classNames(
                                    'h-3.5 w-3.5 text-white'
                                  )}
                                  aria-hidden="true"
                                />
                              ) : (
                                <LuSquare
                                  className={classNames(
                                    'h-3.5 w-3.5',
                                    active
                                      ? 'text-primary-600'
                                      : 'text-text-foreground-secondary'
                                  )}
                                  aria-hidden="true"
                                />
                              )}
                            </span>
                          </span>
                        )}
                      </div>
                    )
                  }}
                </Combobox.Option>
              ))}
            </ul>
          </Combobox.Options>
        </div>
      </Combobox>
    </div>
  )
}

ComboSelect.defaultProps = {
  maxOptions: 100,
  placeholder: '*'
}
