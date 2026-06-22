import type { Story } from '@ladle/react'
import { QueryValueChart } from './QueryValueChart'

export const Basic: Story = () => (
  <div className="h-48 w-80 rounded-md border">
    <QueryValueChart series={[]} valueText="$4.2M" textColor="#2563EB" />
  </div>
)
Basic.meta = {
  description: 'Single big number — worker-resolved value/colors injected'
}

export const WithBackground: Story = () => (
  <div className="h-48 w-80 rounded-md border">
    <QueryValueChart
      series={[]}
      valueText="98.6%"
      textColor="#FFFFFF"
      backgroundColor="#10B981"
    />
  </div>
)
WithBackground.meta = { description: 'Filled-background theme variant' }
