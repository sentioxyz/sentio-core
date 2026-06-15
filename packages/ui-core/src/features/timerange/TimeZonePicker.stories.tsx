import '../../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { TimeZonePicker } from './TimeZonePicker'

export const Basic: Story = () => {
  const [tz, setTz] = useState<string>(' ')
  return (
    <div className="w-96 p-8">
      <TimeZonePicker value={tz} onChange={(v) => setTz(v || ' ')} />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        Selected: {tz.trim() || 'Browser Time'}
      </p>
    </div>
  )
}
Basic.meta = {
  description:
    'Timezone combobox (offsets shown per zone; empty = browser time)'
}
