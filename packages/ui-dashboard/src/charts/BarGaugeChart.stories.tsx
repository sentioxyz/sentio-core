import type { Story } from '@ladle/react'
import { BarGaugeChart } from './BarGaugeChart'
import type { ChartConfigLike } from '../types'

const series = [
  { name: 'Ethereum', data: [[0, 4200]] },
  { name: 'Arbitrum', data: [[0, 1800]] },
  { name: 'Base', data: [[0, 950]] },
  { name: 'Optimism', data: [[0, 430]] }
]
const fmt = (v: number) => '$' + v.toLocaleString()

export const Horizontal: Story = () => {
  const config: ChartConfigLike = {
    barGauge: {
      direction: 'HORIZONTAL',
      sort: { sortBy: 'ByValue', orderDesc: true }
    },
    valueConfig: { showValueLabel: true }
  }
  return (
    <div className="h-80 w-[40rem]">
      <BarGaugeChart
        series={series}
        valueFormatter={fmt}
        config={config}
        title="TVL by chain"
      />
    </div>
  )
}
Horizontal.meta = { description: 'Horizontal bars, sorted by value' }

export const Vertical: Story = () => {
  const config: ChartConfigLike = {
    barGauge: { direction: 'VERTICAL', sort: { sortBy: 'ByName' } },
    valueConfig: { showValueLabel: true }
  }
  return (
    <div className="h-80 w-[40rem]">
      <BarGaugeChart series={series} valueFormatter={fmt} config={config} />
    </div>
  )
}
Vertical.meta = { description: 'Vertical bars, sorted by name' }
