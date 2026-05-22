// import { MagnifyingGlassIcon as SearchIcon } from '@heroicons/react/20/solid'
import { LuSearch } from 'react-icons/lu'
import { FocusEvent, forwardRef, KeyboardEvent } from 'react'
import { classNames } from '../utils/classnames'

type Props = Omit<React.InputHTMLAttributes<HTMLInputElement>, 'onChange'> & {
  onChange: (value: string) => void
  value: string
  onBlur?: (e: FocusEvent<HTMLInputElement>) => void
  onKeydown?: (e: KeyboardEvent<HTMLInputElement>) => void
  placeholder?: string
  ref?: React.Ref<HTMLInputElement>
  addonButton?: React.ReactNode
}

export const SearchInput = forwardRef<HTMLInputElement, Props>((props, ref) => {
  const {
    onChange,
    value,
    onBlur,
    onKeydown,
    addonButton,
    placeholder = 'Search',
    disabled,
    ...args
  }: Props = props

  return (
    <div className="min-w-0 flex-1">
      <label htmlFor="search" className="sr-only">
        Search
      </label>
      <div
        className={classNames(
          'focus-within:ring-primary-600/30 focus-within:ring-3 focus-within:border-primary-600 border-light relative flex rounded-md border',
          disabled
            ? 'cursor-not-allowed opacity-50'
            : 'hover:border-primary-600'
        )}
      >
        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-2">
          <LuSearch
            className="text-text-foreground-secondary h-3.5 w-3.5"
            aria-hidden="true"
          />
        </div>
        <input
          onChange={(e) => onChange(e.target.value)}
          onBlur={onBlur}
          onKeyDown={onKeydown}
          type="search"
          className={classNames(
            'text-ilabel block w-full rounded-md border-0 pl-7 focus:ring-0',
            'h-7.5 py-1',
            'pr-2'
          )}
          placeholder={placeholder}
          value={value}
          ref={ref}
          disabled={disabled}
          {...args}
        />
        {addonButton}
      </div>
    </div>
  )
})

SearchInput.displayName = 'SearchInput'
