import { classNames } from '../../utils/classnames'
import { Combobox } from '@headlessui/react'
import {
  LuCheck,
  LuChevronDown as SelectorIcon,
  LuX as XIcon
} from 'react-icons/lu'
import { useMemo, useState } from 'react'
import { useResizeDetector } from 'react-resize-detector'
import isString from 'lodash/isString'

interface Props {
  options: string[]
  value?: string
  onChange: (value?: string) => void
  className?: string
  inputClassName?: string
  maxOptions?: number
  placeholder?: string | JSX.Element
  error?: string
  name?: string
  displayFn?: (option: string, active?: boolean) => React.ReactNode
  clearAfterSelect?: boolean // clear input after select, make it easier to select multiple options
  loading?: boolean // show a loading hint in the options list instead of "no results"
}

export function ComboInput({
  options,
  value,
  onChange,
  className,
  inputClassName,
  maxOptions,
  error,
  placeholder,
  name,
  displayFn,
  clearAfterSelect,
  loading
}: Props) {
  const [input, setInput] = useState('')
  const { width, ref } = useResizeDetector({
    refreshMode: 'debounce',
    refreshRate: 100,
    handleHeight: false
  })

  const filteredOptions = useMemo(() => {
    const filtered = input
      ? options.filter((o) =>
          String(o).toLowerCase().includes(input?.toLowerCase())
        )
      : options
    return maxOptions || 0 > 0 ? filtered.slice(0, maxOptions) : filtered
  }, [options, input, maxOptions])

  function clearSelection() {
    setInput('')
    onChange(undefined)
  }

  function onSelectChange(value: string | null) {
    onChange(value ?? undefined)
    if (clearAfterSelect) {
      setInput('')
    } else {
      setInput(value ?? '')
    }
  }

  const optionsElement = (
    <Combobox.Options
      as="div"
      className={classNames(
        'ring-primary-600 divide-primary-600 dark:ring-primary-700 dark:divide-primary-700 bg-default-bg gap-0 divide-y rounded-sm py-1 text-sm shadow-lg ring-1',
        'absolute top-0 z-10 min-w-full'
      )}
      unmount={false}
    >
      <div>
        <Combobox.Input
          type="text"
          className={classNames(
            'text-ilabel focus:outline-hidden w-full border-0 px-3 py-0 leading-6 ring-0 focus:ring-0'
          )}
          value={input}
          tabIndex={0}
          name={name}
          onChange={(event) => setInput(event.target.value)}
          autoComplete="off"
          placeholder={isString(placeholder) ? placeholder : undefined}
        />
      </div>
      <ul className="scrollbar-thin max-h-60 overflow-auto pt-1">
        {filteredOptions.map((m, idx) => (
          <Combobox.Option
            key={idx}
            value={m}
            className={({ active }) =>
              classNames(
                'text-ilabel relative flex cursor-default select-none py-2 pl-3 pr-1',
                active ? 'bg-primary-600 text-white' : 'text-text-foreground'
              )
            }
          >
            {({ active, selected }) => (
              <>
                <span
                  className={classNames(
                    'block flex-1 truncate',
                    selected && 'font-semibold'
                  )}
                  title={m}
                >
                  {displayFn ? displayFn(m, active) : m}
                </span>

                {selected && (
                  <span
                    className={classNames(
                      'flex items-center pr-4',
                      active ? 'text-white' : 'text-primary-600'
                    )}
                  >
                    <LuCheck className="h-4.5 w-4.5" aria-hidden="true" />
                  </span>
                )}
              </>
            )}
          </Combobox.Option>
        ))}
        {loading ? (
          <li className="text-icontent text-text-foreground-disabled w-40 select-none px-3 py-2">
            Loading…
          </li>
        ) : (
          filteredOptions.length === 0 && (
            <li className="text-icontent text-text-foreground-disabled w-40 select-none px-3 py-2">
              No matching results.
            </li>
          )
        )}
      </ul>
    </Combobox.Options>
  )

  return (
    <Combobox
      nullable
      as="div"
      className={classNames(className, 'flex h-full grow')}
      value={value}
      onChange={onSelectChange}
    >
      {({ open: headlessOpen }) => {
        return (
          <div
            className={classNames(
              inputClassName,
              'relative inline-flex w-full grow items-center border hover:shadow-sm',
              error
                ? 'border-red-600 hover:border-red-600 focus:border-red-600 focus:ring-red-600'
                : 'focus:border-primary-600 focus:ring-primary-600 border-main hover:border-primary-600'
            )}
            ref={ref}
          >
            <Combobox.Input
              as="div"
              tabIndex={0}
              className={classNames(
                'text-ilabel focus:outline-hidden w-[calc(100%-6rem)] grow rounded-sm border-0 py-1 pr-0 ring-0 focus:ring-0'
              )}
              onChange={() => {
                /**/
              }}
              aria-label={name}
            >
              <Combobox.Button
                className={classNames(
                  value
                    ? 'text-text-foreground'
                    : 'text-text-foreground-disabled',
                  'left-0 w-full cursor-pointer truncate pl-2'
                )}
                as="div"
              >
                {value || placeholder}
              </Combobox.Button>
            </Combobox.Input>
            <div
              role={'button'}
              aria-label={'clear'}
              className={classNames(
                'inset-y-0 flex cursor-pointer items-center p-1',
                value ? '' : 'hidden'
              )}
              onClick={() => clearSelection()}
            >
              <XIcon
                className="hover:bg-primary-400 text-text-foreground-disabled h-4 w-4 rounded-lg hover:text-white"
                aria-hidden="true"
              />
            </div>
            <Combobox.Button
              className="focus:outline-hidden inset-y-0 mx-2 flex items-center rounded-r-md"
              aria-label="select"
            >
              <SelectorIcon
                className="text-text-foreground-secondary h-4 w-4"
                aria-hidden="true"
              />
            </Combobox.Button>

            {headlessOpen ? optionsElement : null}
          </div>
        )
      }}
    </Combobox>
  )
}
