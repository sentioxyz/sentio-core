import type { Story } from '@ladle/react'
import { PieChart } from './PieChart'
import type { ChartConfigLike } from '../types'

// Shape the app passes after running the worker compute: name + [x, value].
const series = [
  { name: 'Ethereum', data: [[0, 4200]] },
  { name: 'Arbitrum', data: [[0, 1800]] },
  { name: 'Base', data: [[0, 950]] },
  { name: 'Optimism', data: [[0, 430]] }
]
const fmt = (v: number) => '$' + v.toLocaleString()
const config: ChartConfigLike = {
  pieConfig: { pieType: 'Donut', showValue: true, showPercent: true }
}

export const Donut: Story = () => (
  <div className="h-80 w-[40rem]">
    <PieChart
      series={series}
      valueFormatter={fmt}
      config={config}
      title="TVL by chain"
    />
  </div>
)
Donut.meta = {
  description: 'Presentational pie/donut — series + formatter injected'
}
