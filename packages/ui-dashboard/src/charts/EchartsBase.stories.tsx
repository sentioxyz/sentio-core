import type { Story } from '@ladle/react'
import { ReactEChartsBase, type EChartsOption } from './EchartsBase'

const days = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

const lineOption: EChartsOption = {
  legend: { data: ['Requests', 'Errors'] },
  tooltip: { trigger: 'axis' },
  grid: { left: 40, right: 16, top: 32, bottom: 24 },
  xAxis: { type: 'category', data: days },
  yAxis: { type: 'value' },
  series: [
    {
      id: 'requests',
      name: 'Requests',
      type: 'line',
      smooth: true,
      data: [120, 200, 150, 80, 70, 110, 130]
    },
    {
      id: 'errors',
      name: 'Errors',
      type: 'line',
      smooth: true,
      data: [5, 12, 8, 3, 2, 6, 4]
    }
  ]
}

const barOption: EChartsOption = {
  legend: { data: ['Volume'] },
  tooltip: { trigger: 'axis' },
  grid: { left: 40, right: 16, top: 32, bottom: 24 },
  xAxis: { type: 'category', data: days },
  yAxis: { type: 'value' },
  series: [
    {
      id: 'volume',
      name: 'Volume',
      type: 'bar',
      data: [220, 182, 191, 234, 290, 330, 310]
    }
  ]
}

export const Line: Story = () => (
  <div className="w-[42rem] p-8">
    <ReactEChartsBase option={lineOption} style={{ height: 280 }} />
  </div>
)
Line.meta = {
  description:
    'ReactEChartsBase rendering a themed line chart (with ChartLegend)'
}

export const Bar: Story = () => (
  <div className="w-[42rem] p-8">
    <ReactEChartsBase option={barOption} style={{ height: 280 }} />
  </div>
)
Bar.meta = { description: 'Bar chart variant' }
