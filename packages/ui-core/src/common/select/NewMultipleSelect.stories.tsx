import '../../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { NewMultipleSelect } from './NewMultipleSelect'

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
  const [value, setValue] = useState<string[]>(['Apple'])
  return (
    <div className="border-light w-80 border p-2">
      <NewMultipleSelect
        options={fruits}
        value={value}
        onChange={setValue}
        unSelectedText="Pick fruits"
      />
    </div>
  )
}
Basic.meta = {
  description: 'Layered multi-select with popover options and removable chips'
}

export const WithTopLevelGroups: Story = () => {
  const [value, setValue] = useState<string[]>([])
  return (
    <div className="border-light w-80 border p-2">
      <NewMultipleSelect
        options={fruits}
        value={value}
        onChange={setValue}
        unSelectedText="Pick fruits"
        topLevelOptions={[
          { id: 'short', label: 'Short names', filterFn: (o) => o.length <= 5 },
          { id: 'long', label: 'Long names', filterFn: (o) => o.length > 5 }
        ]}
      />
    </div>
  )
}
WithTopLevelGroups.meta = {
  description: 'Two-level navigation via topLevelOptions filters'
}
