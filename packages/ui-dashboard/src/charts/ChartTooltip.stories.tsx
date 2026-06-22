import type { Story } from '@ladle/react'
import { ChartTooltip } from './ChartTooltip'
import { ScatterChartTooltip } from './ScatterChartTooltip'

const t0 = Date.UTC(2024, 0, 1, 12, 0, 0)
const fmt = (v: number) => v.toLocaleString()

// echarts tooltip params: one entry per series, value = [time, number].
const series = [
  {
    seriesId: 'a',
    seriesName: 'Volume',
    value: [t0, 1234.5],
    marker:
      '<span style="display:inline-block;margin-right:4px;border-radius:10px;width:10px;height:10px;background-color:#5470f0;"/>'
  },
  {
    seriesId: 'b',
    seriesName: 'TVL',
    value: [t0, 980],
    marker:
      '<span style="display:inline-block;margin-right:4px;border-radius:10px;width:10px;height:10px;background-color:#91cc75;"/>'
  }
]
const compare = [
  {
    seriesId: 'a_compare',
    seriesName: 'Volume',
    value: [t0, 1000],
    marker:
      '<span style="display:inline-block;margin-right:4px;border-radius:10px;width:10px;height:10px;border:1px solid #5470f0;"/>'
  },
  {
    seriesId: 'b_compare',
    seriesName: 'TVL',
    value: [t0, 1100],
    marker:
      '<span style="display:inline-block;margin-right:4px;border-radius:10px;width:10px;height:10px;border:1px solid #91cc75;"/>'
  }
]

const Frame = ({ children }: { children: React.ReactNode }) => (
  <div className="bg-default-bg w-80 rounded-md border p-2">{children}</div>
)

export const Basic: Story = () => (
  <Frame>
    <ChartTooltip data={series} numberFormatter={fmt} showTotal />
  </Frame>
)
Basic.meta = { description: 'Series rows + total' }

export const WithCompare: Story = () => (
  <Frame>
    <ChartTooltip
      data={[...series, ...compare]}
      numberFormatter={fmt}
      showTotal
      compareTimeDuration={{ value: 7, unit: 'd' }}
    />
  </Frame>
)
WithCompare.meta = { description: 'Compare-period rows + % diff' }

export const Scatter: Story = () => (
  <Frame>
    <ScatterChartTooltip
      data={[
        {
          seriesId: 'a',
          seriesName: 'Pool',
          value: [t0, 42, 7],
          marker:
            '<span style="display:inline-block;margin-right:4px;border-radius:10px;width:10px;height:10px;background-color:#5470f0;"/>'
        }
      ]}
      numberFormatter={fmt}
      sizeTitle="Liquidity"
    />
  </Frame>
)
Scatter.meta = { description: 'Scatter point (x/y + size dimension)' }
