import { Combobox } from '@headlessui/react'
import { LuCheck } from 'react-icons/lu'
import { useMemo, useState } from 'react'
import { KeyboardEvent } from 'react'
import { isEqual } from 'lodash'
import { classNames } from '../../utils/classnames'

interface Props<T> {
  options: T[]
  value: T[]
  onChange: (value: T[]) => void
  displayFn?: (option: T) => string | undefined
  className?: string
  unSelectedText?: string
  disabled?: boolean
  maxInputSize?: number
  maxOptions?: number
}

export function MultipleSelect<T>({
  value,
  options,
  onChange,
  displayFn,
  className,
  unSelectedText,
  disabled,
  maxInputSize,
  maxOptions = 100
}: Props<T>) {
  const onDelete = (v: T, idx: number) => {
    const newValue = [...value]
    newValue.splice(idx, 1)
    onChange(newValue)
  }
  const [filter, setFilter] = useState('')

  const inputSize = useMemo(() => {
    let size
    if (value.length === 0) {
      const optionLabels = options.map(
        (o) => (displayFn ? displayFn(o) : `${o}`) || ''
      )
      const minWidth = Math.max(...optionLabels.map((o) => o.length))
      const length = filter == '' ? unSelectedText?.length || 1 : filter.length
      size = Math.max(minWidth, length)
    } else {
      size = Math.max(1, filter.length)
    }
    return (maxInputSize || 0) > 0 ? Math.min(size, maxInputSize || 0) : size
  }, [filter, unSelectedText, value, options, displayFn, maxInputSize])

  const filteredOptions = useMemo(() => {
    const filtered = options.filter((o) => {
      const label = (displayFn ? displayFn(o) : `${o}`) || ''
      return label.toLowerCase().includes(filter.toLowerCase())
    })
    return (maxOptions || 0) > 0 ? filtered.slice(0, maxOptions) : filtered
  }, [options, filter, displayFn, maxOptions])

  function onInputKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Backspace') {
      if (filter.length === 0) {
        onDelete(value[value.length - 1], value.length - 1)
      }
    }
    if (e.key === 'Escape') {
      ;(e.target as HTMLInputElement).blur()
      e.preventDefault()
    }
  }

  function onValueChange(v: T[]) {
    setFilter('')
    onChange(v)
  }

  return (
    <Combobox
      as="div"
      className={className}
      value={value}
      onChange={onValueChange}
      disabled={disabled}
      multiple
      nullable
    >
      <div className="relative inline-flex h-full min-w-full grow">
        <Combobox.Button as="div" className={classNames('h-full w-full')}>
          <Combobox.Input
            onChange={() => {
              /**/
            }}
            as="div"
            className={classNames(
              'inline-flex h-full w-full items-center gap-1 border-0 p-0',
              disabled ? 'bg-gray-100' : 'bg-default-bg'
            )}
          >
            {value.map((l, idx) => (
              <Label
                key={idx}
                value={l}
                invalid={!options.some((o) => isEqual(o, l))}
                onDelete={() => onDelete(l, idx)}
                displayFn={displayFn}
              />
            ))}

            <input
              disabled={disabled}
              onChange={(e) => setFilter(e.target.value)}
              value={filter}
              size={inputSize}
              placeholder={value.length == 0 ? unSelectedText : ''}
              onKeyDown={(e) => onInputKeyDown(e)}
              className={classNames(
                'focus:outline-hidden ml-1 h-full min-w-fit pr-6',
                disabled
                  ? 'dark:bg-primary-200/50! bg-gray-100!'
                  : 'bg-default-bg'
              )}
              autoComplete="off"
            />
          </Combobox.Input>
        </Combobox.Button>
        <Combobox.Options
          className={classNames(
            'sm:text-icontent focus:outline-hidden bg-default-bg absolute top-full z-10 max-h-60 overflow-auto py-1 text-base shadow-lg ring-1 ring-black/5 dark:ring-gray-100'
          )}
        >
          {filteredOptions.map((m, idx) => (
            <Combobox.Option
              key={idx}
              value={m}
              className={({ active }) =>
                classNames(
                  'relative cursor-default select-none py-2 pl-3 pr-9',
                  active ? 'bg-primary-600 text-white' : 'text-text-foreground'
                )
              }
            >
              {({ active, selected }) => {
                const text = displayFn ? displayFn(m) : `${m}`
                return (
                  <>
                    <span
                      title={text}
                      className={classNames(
                        'block truncate',
                        selected && 'font-semibold'
                      )}
                    >
                      {text}
                    </span>

                    {selected && (
                      <span
                        className={classNames(
                          'absolute inset-y-0 right-0 flex items-center pr-3',
                          active ? 'text-white' : 'text-primary-600'
                        )}
                      >
                        <LuCheck className="h-4.5 w-4.5" aria-hidden="true" />
                      </span>
                    )}
                  </>
                )
              }}
            </Combobox.Option>
          ))}
        </Combobox.Options>
      </div>
    </Combobox>
  )
}

function Label<T>({
  value,
  onDelete,
  displayFn,
  invalid
}: {
  value: T
  invalid: boolean
  onDelete: (value: T) => void
  displayFn?: (value: T) => string | undefined
}) {
  const title = displayFn ? displayFn(value) : `${value}`
  return (
    <span
      className={classNames(
        invalid ? 'bg-red-100 text-red-700' : 'text-primary-700 bg-primary-100',
        'inline-flex h-full max-w-xs items-center pl-2  pr-0.5 text-xs font-medium'
      )}
    >
      <span title={title} className="truncate">
        {title}
      </span>
      <div
        onClick={() => onDelete(value)}
        className={classNames(
          invalid
            ? 'text-red-400 hover:bg-red-200 hover:text-red-500'
            : 'text-primary-400 hover:bg-primary-200 hover:text-primary-500',
          'ml-0.5 inline-flex h-4 w-4 shrink-0 cursor-pointer items-center justify-center rounded-full  '
        )}
      >
        <span className="sr-only">Remove</span>
        <svg
          className="h-2 w-2"
          stroke="currentColor"
          fill="none"
          viewBox="0 0 8 8"
        >
          <path strokeLinecap="round" strokeWidth="1.5" d="M1 1l6 6m0-6L1 7" />
        </svg>
      </div>
    </span>
  )
}
