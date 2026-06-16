import '../../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { DurationInput, type DurationLike } from './DurationInput'

export const Basic: Story = () => {
  const [value, setValue] = useState<DurationLike>({ value: 5, unit: 'm' })
  return (
    <div className="w-72 p-8">
      <DurationInput value={value} onChange={setValue} />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        {value.value} {value.unit}
      </p>
    </div>
  )
}
Basic.meta = {
  description: 'Numeric duration with a unit selector (s / m / h)'
}

export const WithDays: Story = () => {
  const [value, setValue] = useState<DurationLike>({ value: 2, unit: 'd' })
  return (
    <div className="w-72 p-8">
      <DurationInput value={value} onChange={setValue} enableDays />
    </div>
  )
}
WithDays.meta = {
  description: 'enableDays adds day/week units'
}
