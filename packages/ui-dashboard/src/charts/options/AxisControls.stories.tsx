import type { Story } from '@ladle/react'
import { useState } from 'react'
import YaxisControls from './YaxisControls'
import { XAxisControls } from './XaxisControls'
import type { XAxisConfigLike, YAxisConfigLike } from '../../types'

export const Yaxis: Story = () => {
  const [yAxis, setYAxis] = useState<YAxisConfigLike>({
    name: 'TVL',
    scale: true
  })
  return (
    <div className="w-[48rem] p-8">
      <YaxisControls yAxis={yAxis} setYAxis={setYAxis} defaultOpen />
      <pre className="text-text-foreground-secondary mt-4 text-xs">
        {JSON.stringify(yAxis, null, 2)}
      </pre>
    </div>
  )
}
Yaxis.meta = { description: 'Y-axis name / min-max / stack / show-zero' }

export const Xaxis: Story = () => {
  const [xAxis, setXAxis] = useState<XAxisConfigLike>({
    name: 'Time',
    column: 'ts',
    type: 'category'
  })
  return (
    <div className="w-[48rem] p-8">
      <XAxisControls
        xAxis={xAxis}
        setXAxis={(v) => setXAxis(v || {})}
        defaultOpen
        supportSetType
        supportSort
        columnIsNonTime
      />
      <pre className="text-text-foreground-secondary mt-4 text-xs">
        {JSON.stringify(xAxis, null, 2)}
      </pre>
    </div>
  )
}
Xaxis.meta = {
  description: 'X-axis name / type / sort (sort shown for non-time column)'
}
