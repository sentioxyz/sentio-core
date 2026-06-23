import type { Story } from '@ladle/react'
import { useState } from 'react'
import YaxisControls from './YaxisControls'
import { XAxisControls } from './XaxisControls'
import type { XAxisConfigLike, YAxisConfigLike } from '../../types'

export const Yaxis: Story = () => {
  const [yAxis, setYAxis] = useState<YAxisConfigLike>({
    name: 'TVL',
    column: 'tvl',
    scale: true
  })
  return (
    <div className="w-full p-8">
      <YaxisControls
        yAxis={yAxis}
        setYAxis={setYAxis}
        defaultOpen
        columnSelect={
          <select
            className="border-main focus:border-primary-500 focus:ring-3 focus:ring-primary-600/30 h-8 rounded-r-md border bg-white px-2 focus:outline-none"
            value={yAxis.column ?? 'tvl'}
            onChange={(event) =>
              setYAxis({ ...yAxis, column: event.target.value })
            }
          >
            <option value="tvl">TVL</option>
            <option value="volume">Volume</option>
            <option value="transactions">Transactions</option>
            <option value="fees">Fees</option>
          </select>
        }
      />
      <pre className="text-text-foreground-secondary mt-4 text-xs">
        {JSON.stringify(yAxis, null, 2)}
      </pre>
    </div>
  )
}
Yaxis.meta = {
  description: 'Y-axis name / column / min-max / stack / show-zero'
}

export const Xaxis: Story = () => {
  const [xAxis, setXAxis] = useState<XAxisConfigLike>({
    name: 'Time',
    column: 'ts',
    type: 'category'
  })
  return (
    <div className="w-full p-8">
      <XAxisControls
        xAxis={xAxis}
        setXAxis={(v) => setXAxis(v || {})}
        defaultOpen
        supportSetType
        supportSort
        columnIsNonTime
        columnSelect={
          <select
            className="border-main focus:border-primary-500 focus:ring-3 focus:ring-primary-600/30 h-8 rounded-r-md border bg-white px-2 focus:outline-none"
            value={xAxis.column ?? 'ts'}
            onChange={(event) =>
              setXAxis({ ...xAxis, column: event.target.value })
            }
          >
            <option value="ts">Timestamp</option>
            <option value="block">Block</option>
            <option value="txHash">Tx Hash</option>
            <option value="category">Category</option>
          </select>
        }
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
