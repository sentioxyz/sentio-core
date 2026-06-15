import '../../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { ComboSelect } from './ComboSelect'

const chains = [
  'Ethereum',
  'Polygon',
  'Arbitrum',
  'Optimism',
  'Base',
  'Avalanche'
]

export const Basic: Story = () => {
  const [value, setValue] = useState<string>()
  return (
    <div className="w-72 p-8">
      <ComboSelect
        options={chains}
        value={value}
        onChange={setValue}
        label="Chain"
        placeholder="Select a chain"
      />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        Selected: {value || '—'}
      </p>
    </div>
  )
}
Basic.meta = { description: 'Generic combobox select with label and filter' }

export const WithCopy: Story = () => {
  const [value, setValue] = useState<string | undefined>('0xab12…cd34')
  return (
    <div className="w-72 p-8">
      <ComboSelect
        options={['0xab12…cd34', '0x5678…90ef', '0xdead…beef']}
        value={value}
        onChange={setValue}
        supportCopy
      />
    </div>
  )
}
WithCopy.meta = { description: 'Options expose a copy button (supportCopy)' }
