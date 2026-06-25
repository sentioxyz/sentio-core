import type { Story } from '@ladle/react'
import { useState } from 'react'
import { SeriesControls } from './SeriesControls'
import type { ChartConfigLike, SeriesConfigLike } from '../types/chart'

export const Basic: Story = () => {
  const [config, setConfig] = useState<ChartConfigLike>({})
  return (
    <div className="w-[40rem] p-8">
      <SeriesControls
        config={config}
        chartType="LINE"
        series={['transfer_volume', 'active_users', 'tvl']}
        enabled
        setSeriesConfig={(seriesConfig: SeriesConfigLike) =>
          setConfig((c) => ({ ...c, seriesConfig }))
        }
      />
      <pre className="mt-4 text-xs">
        {JSON.stringify(config.seriesConfig ?? {}, null, 2)}
      </pre>
    </div>
  )
}

export const Disabled: Story = () => (
  <div className="p-8">
    <SeriesControls
      config={{}}
      chartType="LINE"
      series={['a', 'b']}
      enabled={false}
      setSeriesConfig={() => {}}
    />
    <span className="text-xs">(flag off → renders nothing)</span>
  </div>
)
