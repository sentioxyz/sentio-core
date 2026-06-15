import '../../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { ComboInput } from './ComboInput'

const fruits = [
  'Apple',
  'Banana',
  'Cherry',
  'Date',
  'Elderberry',
  'Fig',
  'Grape',
  'Honeydew'
]

export const Basic: Story = () => {
  const [value, setValue] = useState<string>()
  return (
    <div className="w-72 p-8">
      <ComboInput
        options={fruits}
        value={value}
        onChange={setValue}
        placeholder="Pick a fruit"
      />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        Selected: {value || '—'}
      </p>
    </div>
  )
}
Basic.meta = {
  description: 'Combobox text input filtering a string[] option list'
}

export const WithCustomOptionRender: Story = () => {
  const [value, setValue] = useState<string>()
  return (
    <div className="w-72 p-8">
      <ComboInput
        options={fruits}
        value={value}
        onChange={setValue}
        placeholder="Pick a fruit"
        displayFn={(o, active) => (
          <span className={active ? 'font-semibold' : ''}>🍓 {o}</span>
        )}
      />
    </div>
  )
}
WithCustomOptionRender.meta = {
  description: 'Custom option rendering via displayFn'
}
