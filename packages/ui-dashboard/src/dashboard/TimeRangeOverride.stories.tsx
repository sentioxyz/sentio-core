import type { Story } from '@ladle/react'
import { useState } from 'react'
import dayjs from 'dayjs'
import { TimeRangeOverride } from './TimeRangeOverride'
import type { TimeRangeOverrideLike } from '../types/chart'

export const Basic: Story = () => {
  const [config, setConfig] = useState<TimeRangeOverrideLike>({
    enabled: false
  })
  const [global, setGlobal] = useState({
    start: dayjs().subtract(1, 'hour'),
    end: dayjs()
  })
  return (
    <div className="w-[40rem] p-8">
      <TimeRangeOverride
        config={config}
        onChange={setConfig}
        globalStartTime={global.start}
        globalEndTime={global.end}
        globalTz="UTC"
        onSetGlobalTimeRange={(start, end) => {
          if (dayjs.isDayjs(start) && dayjs.isDayjs(end))
            setGlobal({ start, end })
        }}
      />
      <pre className="mt-4 text-xs">{JSON.stringify(config, null, 2)}</pre>
    </div>
  )
}
