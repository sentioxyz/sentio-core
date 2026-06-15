import '../../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import dayjs from 'dayjs'
import Calendar from './Calendar'

export const Basic: Story = () => {
  const [value, setValue] = useState(dayjs())
  return (
    <div className="p-8">
      <Calendar value={value} onSelect={setValue} />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        Selected: {value.format('YYYY-MM-DD')}
      </p>
    </div>
  )
}
Basic.meta = {
  description: 'Month/year grid date picker (visible at md+ widths)'
}

export const RangeHighlight: Story = () => {
  const [value, setValue] = useState(dayjs())
  const start = dayjs().subtract(5, 'day')
  const end = dayjs().add(3, 'day')
  return (
    <div className="p-8">
      <Calendar
        value={value}
        start={start}
        end={end}
        enableRangeLimit
        onSelect={setValue}
      />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        Range {start.format('MM-DD')} → {end.format('MM-DD')} highlighted; dates
        outside are disabled.
      </p>
    </div>
  )
}
RangeHighlight.meta = {
  description: 'Start/end range highlight with range limiting'
}
