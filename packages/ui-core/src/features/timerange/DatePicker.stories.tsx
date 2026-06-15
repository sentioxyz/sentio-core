import '../../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import dayjs, { Dayjs } from 'dayjs'
import { DatePicker } from './DatePicker'

export const Basic: Story = () => {
  const [value, setValue] = useState<Dayjs>()
  return (
    <div className="w-64 p-8">
      <DatePicker value={value} onChange={setValue} />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        Selected: {value ? value.utc().format('YYYY-MM-DD') : '—'}
      </p>
    </div>
  )
}
Basic.meta = { description: 'Popover-anchored single date picker' }

export const WithRangeLimit: Story = () => {
  const [value, setValue] = useState<Dayjs>()
  return (
    <div className="w-64 p-8">
      <DatePicker
        value={value}
        onChange={setValue}
        start={dayjs().subtract(7, 'day')}
        end={dayjs().add(7, 'day')}
        placeholder="Within ±7 days"
      />
    </div>
  )
}
WithRangeLimit.meta = {
  description: 'Date picker limited to a start/end window'
}
