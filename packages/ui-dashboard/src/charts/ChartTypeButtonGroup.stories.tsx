import type { Story } from '@ladle/react'
import { useState } from 'react'
import { ChartTypeButtonGroup } from './ChartTypeButtonGroup'
import type { ChartTypeLike } from '../types'

export const Default: Story = () => {
  const [value, setValue] = useState<ChartTypeLike>('LINE')
  return (
    <div className="w-[60rem] p-8">
      <ChartTypeButtonGroup value={value} onChange={setValue} />
      <pre className="text-text-foreground-secondary mt-4 text-xs">{value}</pre>
    </div>
  )
}
Default.meta = {
  description: 'Chart-type selector (collapses labels under 800px)'
}
