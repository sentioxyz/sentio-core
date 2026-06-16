import '../../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import TimeRangePicker from './TimeRangePicker'
import { ago, now, type DateTimeValue } from '../../utils/time'

export const Basic: Story = () => {
  const [range, setRange] = useState<{
    start?: DateTimeValue
    end?: DateTimeValue
    tz?: string
  }>({
    start: ago(7, 'days'),
    end: now
  })
  return (
    <div className="p-8">
      <TimeRangePicker
        startTime={range.start}
        endTime={range.end}
        tz={range.tz}
        onChange={(start, end, tz) => setRange({ start, end, tz })}
      />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        tz: {range.tz || 'browser'}
      </p>
    </div>
  )
}
Basic.meta = {
  description:
    'Controlled time range picker (presets / calendars / time / timezone)'
}
