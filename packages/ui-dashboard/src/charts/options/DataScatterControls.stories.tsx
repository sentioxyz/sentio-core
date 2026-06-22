import type { Story } from '@ladle/react'
import { useState } from 'react'
import { DataControls } from './DataControls'
import { ScatterControls } from './ScatterControls'
import type {
  ChartConfigLike,
  DataConfigLike,
  ScatterConfigLike
} from '../../types'

export const Data: Story = () => {
  const [cfg, setCfg] = useState<ChartConfigLike>({
    dataConfig: { seriesLimit: 50 }
  })
  return (
    <div className="w-[40rem] p-8">
      <DataControls
        chartConfig={cfg}
        defaultSeriesLimit={20}
        onChange={(dataConfig: DataConfigLike) =>
          setCfg({ ...cfg, dataConfig })
        }
        defaultOpen
      />
      <pre className="text-text-foreground-secondary mt-4 text-xs">
        {JSON.stringify(cfg.dataConfig, null, 2)}
      </pre>
    </div>
  )
}
Data.meta = {
  description: 'Max series limit (per-chart-type default injected)'
}

// Stub pickers stand in for the app-injected sql column picker / color picker.
const stubSelect = ({
  value,
  onChange
}: {
  value?: string
  onChange: (v: string) => void
}) => (
  <input
    className="border-main text-icontent h-full w-40 rounded-r-md border px-2"
    value={value || ''}
    placeholder="column"
    onChange={(e) => onChange(e.target.value)}
  />
)

export const Scatter: Story = () => {
  const [config, setConfig] = useState<ScatterConfigLike>({
    minSize: 5,
    maxSize: 30
  })
  return (
    <div className="w-[52rem] p-8">
      <ScatterControls
        config={config}
        onChange={setConfig}
        defaultOpen
        columnSelect={stubSelect}
        colorPicker={({ value, onChange }) => stubSelect({ value, onChange })}
      />
      <pre className="text-text-foreground-secondary mt-4 text-xs">
        {JSON.stringify(config, null, 2)}
      </pre>
    </div>
  )
}
Scatter.meta = {
  description: 'Scatter size/color (column + color pickers injected by app)'
}
