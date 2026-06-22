import type { Story } from '@ladle/react'
import { useState } from 'react'
import { LineControls } from './LineControls'
import { LabelControls } from './LabelControls'
import { PieChartControls } from './PieChartControls'
import { BarGaugeControls } from './BarGaugeControls'
import { ValueControls } from './ValueControls'
import type {
  LineConfigLike,
  LabelConfigLike,
  PieConfigLike,
  BarGaugeConfigLike,
  ValueConfigLike
} from '../../types'

function Frame({
  children,
  value
}: {
  children: React.ReactNode
  value: unknown
}) {
  return (
    <div className="w-full p-8">
      {children}
      <pre className="text-text-foreground-secondary mt-4 text-xs">
        {JSON.stringify(value, null, 2)}
      </pre>
    </div>
  )
}

export const Line: Story = () => {
  const [config, setConfig] = useState<LineConfigLike>({
    style: 'Solid',
    smooth: false
  })
  return (
    <Frame value={config}>
      <LineControls config={config} onChange={setConfig} />
    </Frame>
  )
}
Line.meta = { description: 'Line style options' }

export const Label: Story = () => {
  const [config, setConfig] = useState<LabelConfigLike>({
    alias: '{{contract}}',
    columns: []
  })
  return (
    <Frame value={config}>
      <LabelControls config={config} setConfig={setConfig} defaultOpen />
    </Frame>
  )
}
Label.meta = { description: 'Series label alias/template' }

export const Pie: Story = () => {
  const [config, setConfig] = useState<PieConfigLike>({
    pieType: 'Pie',
    calculation: 'LAST',
    showValue: true
  })
  return (
    <Frame value={config}>
      <PieChartControls config={config} onChange={setConfig} defaultOpen />
    </Frame>
  )
}
Pie.meta = { description: 'Pie/donut options' }

export const BarGauge: Story = () => {
  const [config, setConfig] = useState<BarGaugeConfigLike>({
    direction: 'HORIZONTAL',
    calculation: 'LAST'
  })
  return (
    <Frame value={config}>
      <BarGaugeControls config={config} onChange={setConfig} defaultOpen />
    </Frame>
  )
}
BarGauge.meta = { description: 'Bar-gauge direction/calculation/sort options' }

export const Value: Story = () => {
  const [config, setConfig] = useState<ValueConfigLike>({
    valueFormatter: 'NumberFormatter',
    style: 'Standard',
    maxFractionDigits: 2
  })
  return (
    <Frame value={config}>
      <ValueControls
        config={config}
        onChange={setConfig}
        defaultOpen
        showPrefix
        showSuffix
      />
    </Frame>
  )
}
Value.meta = {
  description: 'Value formatter (number/date/string mapping) + prefix/suffix'
}
