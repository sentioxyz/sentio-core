import '../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { Input } from './Input'

export const Default: Story = () => {
  const [value, setValue] = useState('')

  return (
    <div className="max-w-md p-8">
      <h3 className="mb-4 text-lg font-semibold">Default Input</h3>
      <Input
        name="default"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Enter text..."
      />
      <p className="mt-2 text-sm text-text-foreground-secondary">Value: {value || '(empty)'}</p>
    </div>
  )
}

Default.meta = {
  description: 'Basic input with default (md) size'
}

export const Sizes: Story = () => {
  return (
    <div className="max-w-md space-y-4 p-8">
      <h3 className="mb-4 text-lg font-semibold">Sizes</h3>
      <div className="space-y-2">
        <label className="text-sm text-text-foreground-secondary">Small (sm)</label>
        <Input name="sm" size="sm" placeholder="Small input" />
      </div>
      <div className="space-y-2">
        <label className="text-sm text-text-foreground-secondary">Medium (md) — default</label>
        <Input name="md" size="md" placeholder="Medium input" />
      </div>
      <div className="space-y-2">
        <label className="text-sm text-text-foreground-secondary">Large (lg)</label>
        <Input name="lg" size="lg" placeholder="Large input" />
      </div>
    </div>
  )
}

Sizes.meta = {
  description: 'Input in all three available sizes: sm, md, lg'
}

export const WithError: Story = () => {
  const [value, setValue] = useState('')

  return (
    <div className="max-w-md p-8">
      <h3 className="mb-4 text-lg font-semibold">With Error</h3>
      <Input
        name="with-error"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Enter a value..."
        error={{ type: 'required', message: 'This field is required' } as any}
      />
    </div>
  )
}

WithError.meta = {
  description: 'Input displaying an error state with a message'
}

export const ErrorAllSizes: Story = () => {
  const errorObj = { type: 'required', message: 'This field is required' } as any

  return (
    <div className="max-w-md space-y-4 p-8">
      <h3 className="mb-4 text-lg font-semibold">Error — All Sizes</h3>
      <Input name="sm-err" size="sm" placeholder="Small error" error={errorObj} />
      <Input name="md-err" size="md" placeholder="Medium error" error={errorObj} />
      <Input name="lg-err" size="lg" placeholder="Large error" error={errorObj} />
    </div>
  )
}

ErrorAllSizes.meta = {
  description: 'Error state across all sizes'
}

export const Disabled: Story = () => {
  return (
    <div className="max-w-md p-8">
      <h3 className="mb-4 text-lg font-semibold">Disabled</h3>
      <Input name="disabled" disabled value="Cannot edit this" placeholder="Disabled input" />
    </div>
  )
}

Disabled.meta = {
  description: 'Input in disabled (read-only) state'
}

export const Controlled: Story = () => {
  const [value, setValue] = useState('Hello, world!')

  return (
    <div className="max-w-md p-8">
      <h3 className="mb-4 text-lg font-semibold">Controlled Input</h3>
      <Input
        name="controlled"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Type something..."
      />
      <div className="mt-3 flex gap-2">
        <button
          className="rounded bg-primary-500 px-3 py-1 text-sm text-white hover:bg-primary-600"
          onClick={() => setValue('')}
        >
          Clear
        </button>
        <button
          className="rounded bg-gray-200 px-3 py-1 text-sm hover:bg-gray-300"
          onClick={() => setValue('Reset value')}
        >
          Reset
        </button>
      </div>
      <p className="mt-2 text-sm text-text-foreground-secondary">Current: {value || '(empty)'}</p>
    </div>
  )
}

Controlled.meta = {
  description: 'Controlled input with external value manipulation'
}
