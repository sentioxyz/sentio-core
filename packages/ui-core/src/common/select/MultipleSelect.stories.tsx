import '../../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { MultipleSelect } from './MultipleSelect'

const fruits = [
  'Apple',
  'Banana',
  'Cherry',
  'Date',
  'Elderberry',
  'Fig',
  'Grape'
]

export const Basic: Story = () => {
  const [value, setValue] = useState<string[]>(['Apple'])
  return (
    <div className="border-light w-80 border p-2">
      <MultipleSelect
        options={fruits}
        value={value}
        onChange={setValue}
        unSelectedText="Pick fruits"
      />
    </div>
  )
}
Basic.meta = {
  description: 'Headless-UI combobox multi-select with removable chips'
}
