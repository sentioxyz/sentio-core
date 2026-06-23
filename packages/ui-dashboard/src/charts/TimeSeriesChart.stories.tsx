import type { Story } from '@ladle/react'
import { TimeSeriesChart } from './TimeSeriesChart'
import type { ChartConfigLike, ChartTypeLike, SeriesLike } from '../types'

// Build N points/series of mock time series, one point per hour.
const t0 = Date.UTC(2024, 0, 1, 0, 0, 0)
function mockSeries(
  names: string[],
  points = 48,
  type: 'line' | 'bar' = 'line'
): SeriesLike<Date>[] {
  return names.map((name, si) => ({
    id: name,
    name,
    type,
    showSymbol: false,
    data: Array.from({ length: points }, (_, i) => {
      const base = 100 * (si + 1)
      const v = base + base * 0.4 * Math.sin(i / 4 + si) + i * (si + 1)
      return [new Date(t0 + i * 3600_000), Math.round(v)] as [Date, number]
    })
  }))
}

const fmt = (v: number) => v.toLocaleString()
const series = mockSeries(['Ethereum', 'Arbitrum', 'Base'])
const start = new Date(t0)
const end = new Date(t0 + 47 * 3600_000)

function Frame({ children }: { children: React.ReactNode }) {
  return <div className="h-[26rem] w-full p-4">{children}</div>
}

export const Line: Story = () => {
  const config: ChartConfigLike = {
    xAxis: { type: 'category' },
    lineConfig: { style: 'Solid' }
  }
  return (
    <Frame>
      <TimeSeriesChart
        series={series}
        legend={series.map((s) => s.name)}
        numberFormatter={fmt}
        chartType={'LINE' as ChartTypeLike}
        config={config}
        startTime={start as any}
        endTime={end as any}
      />
    </Frame>
  )
}
Line.meta = { description: 'Line chart — injected series + numberFormatter' }

export const Area: Story = () => (
  <Frame>
    <TimeSeriesChart
      series={mockSeries(['Volume', 'Fees'])}
      legend={['Volume', 'Fees']}
      numberFormatter={fmt}
      chartType={'AREA' as ChartTypeLike}
      config={{ xAxis: { type: 'category' }, yAxis: { stacked: 'samesign' } }}
      startTime={start as any}
      endTime={end as any}
    />
  </Frame>
)
Area.meta = { description: 'Stacked area variant' }

export const Bar: Story = () => (
  <Frame>
    <TimeSeriesChart
      series={mockSeries(['TVL'], 24, 'bar')}
      legend={['TVL']}
      numberFormatter={fmt}
      chartType={'BAR' as ChartTypeLike}
      config={{ xAxis: { type: 'category' } }}
      startTime={start as any}
      endTime={end as any}
    />
  </Frame>
)
Bar.meta = { description: 'Bar variant' }
