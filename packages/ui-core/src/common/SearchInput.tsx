import { MagnifyingGlassIcon as SearchIcon } from '@heroicons/react/20/solid'
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
    ...args
  }: Props = props

  return (
    <div className="min-w-0 flex-1">
      <label htmlFor="search" className="sr-only">
        Search
      </label>
      <div className="focus-within:ring-primary-500 focus-within:border-primary-500 relative flex  rounded-md  border border-gray-300">
        <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-1 sm:pl-3">
          <SearchIcon
            className="sm:h-4.5 sm:w-4.5 h-4 w-4 text-gray-400"
            aria-hidden="true"
          />
        </div>
        <input
          onChange={(e) => onChange(e.target.value)}
          onBlur={onBlur}
          onKeyDown={onKeydown}
          type="search"
          className={classNames(
            'md:text-ilabel block w-full rounded-md border-0 pl-6 text-xs focus:ring-0 sm:pl-10 sm:text-sm',
            'h-[30px] py-1'
          )}
          placeholder={placeholder}
          value={value}
          ref={ref}
          {...args}
        />
        {addonButton}
      </div>
    </div>
  )
})

SearchInput.displayName = 'SearchInput'
