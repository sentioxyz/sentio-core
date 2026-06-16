import '../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import NewButtonGroup from './NewButtonGroup'

const buttons = [
  { label: 'Line', value: 'line' },
  { label: 'Bar', value: 'bar' },
  { label: 'Area', value: 'area' }
]

export const Basic: Story = () => {
  const [value, setValue] = useState('line')
  return (
    <div className="p-8">
      <NewButtonGroup buttons={buttons} value={value} onChange={setValue} />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        Selected: {value}
      </p>
    </div>
  )
}
Basic.meta = {
  description: 'Segmented single-select button group'
}

export const LightSmall: Story = () => {
  const [value, setValue] = useState('bar')
  return (
    <div className="p-8">
      <NewButtonGroup
        buttons={buttons}
        value={value}
        onChange={setValue}
        theme="light"
        small
      />
    </div>
  )
}
LightSmall.meta = {
  description: 'Light theme, small size'
}
