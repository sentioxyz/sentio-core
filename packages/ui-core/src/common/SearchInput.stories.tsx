import '../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { SearchInput } from './SearchInput'

export const BasicSearchInput: Story = () => {
  const [value, setValue] = useState('')

  return (
    <div className="max-w-md p-8">
      <h3 className="mb-4 text-lg font-semibold">Basic Search Input</h3>
      <SearchInput value={value} onChange={setValue} />
      <p className="mt-2 text-sm text-gray-600">
        Current value: {value || '(empty)'}
      </p>
    </div>
  )
}

BasicSearchInput.meta = {
  description: 'Basic search input with icon'
}

export const WithCustomPlaceholder: Story = () => {
  const [value, setValue] = useState('')

  return (
    <div className="max-w-md p-8">
      <h3 className="mb-4 text-lg font-semibold">Custom Placeholder</h3>
      <SearchInput
        value={value}
        onChange={setValue}
        placeholder="Search users..."
      />
      <p className="mt-2 text-sm text-gray-600">
        Searching for: {value || '(empty)'}
      </p>
    </div>
  )
}

WithCustomPlaceholder.meta = {
  description: 'Search input with custom placeholder text'
}

export const WithAddonButton: Story = () => {
  const [value, setValue] = useState('')

  const handleSearch = () => {
    alert(`Searching for: ${value}`)
  }

  return (
    <div className="max-w-md p-8">
      <h3 className="mb-4 text-lg font-semibold">With Addon Button</h3>
      <SearchInput
        value={value}
        onChange={setValue}
        addonButton={
          <button
            onClick={handleSearch}
            className="rounded-r-md bg-blue-500 px-4 text-sm text-white hover:bg-blue-600"
          >
            Go
          </button>
        }
      />
    </div>
  )
}

WithAddonButton.meta = {
  description: 'Search input with addon button on the right'
}

export const WithBlurHandler: Story = () => {
  const [value, setValue] = useState('')
  const [lastBlurValue, setLastBlurValue] = useState('')

  return (
    <div className="max-w-md p-8">
      <h3 className="mb-4 text-lg font-semibold">With Blur Handler</h3>
      <SearchInput
        value={value}
        onChange={setValue}
        onBlur={() => setLastBlurValue(value)}
        placeholder="Type and click outside"
      />
      <p className="mt-2 text-sm text-gray-600">
        Current: {value || '(empty)'}
      </p>
      <p className="text-sm text-gray-600">
        Last blur: {lastBlurValue || '(empty)'}
      </p>
    </div>
  )
}

WithBlurHandler.meta = {
  description: 'Search input with blur event handler'
}

export const WithKeyboardHandler: Story = () => {
  const [value, setValue] = useState('')
  const [searches, setSearches] = useState<string[]>([])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && value.trim()) {
      setSearches([...searches, value])
      setValue('')
    }
  }

  return (
    <div className="max-w-md p-8">
      <h3 className="mb-4 text-lg font-semibold">Press Enter to Search</h3>
      <SearchInput
        value={value}
        onChange={setValue}
        onKeydown={handleKeyDown}
        placeholder="Type and press Enter"
      />
      {searches.length > 0 && (
        <div className="mt-4">
          <h4 className="mb-2 text-sm font-semibold">Search History:</h4>
          <ul className="list-inside list-disc text-sm text-gray-600">
            {searches.map((search, idx) => (
              <li key={idx}>{search}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

WithKeyboardHandler.meta = {
  description: 'Search input that handles Enter key to submit'
}

export const Disabled: Story = () => {
  const [value, setValue] = useState('Search is disabled')

  return (
    <div className="max-w-md p-8">
      <h3 className="mb-4 text-lg font-semibold">Disabled State</h3>
      <SearchInput value={value} onChange={setValue} disabled />
    </div>
  )
}

Disabled.meta = {
  description: 'Disabled search input'
}

export const ReadOnly: Story = () => {
  const [value] = useState('This is read-only')

  return (
    <div className="max-w-md p-8">
      <h3 className="mb-4 text-lg font-semibold">Read-Only State</h3>
      <SearchInput value={value} onChange={() => {}} readOnly />
    </div>
  )
}

ReadOnly.meta = {
  description: 'Read-only search input'
}
