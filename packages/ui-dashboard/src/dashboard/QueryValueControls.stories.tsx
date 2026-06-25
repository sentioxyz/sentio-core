import type { Story } from '@ladle/react'
import { useState } from 'react'
import { QueryValueControls } from './QueryValueControls'
import type { QueryValueConfigLike } from '../types/chart'

export const Basic: Story = () => {
  const [config, setConfig] = useState<QueryValueConfigLike>({
    calculation: 'LAST'
  })
  return (
    <div className="w-[56rem] p-8">
      <QueryValueControls
        config={config}
        defaultOpen
        onChange={setConfig}
        renderColorSelect={(value, onChange) => (
          // Stand-in for the app's worker-bound ColorSelect.
          <select
            className="px-3"
            value={value?.themeType ?? 'Gray'}
            onChange={(e) => onChange({ value: { themeType: e.target.value } })}
          >
            {['Gray', 'Red', 'Green', 'Blue'].map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        )}
      />
      <pre className="mt-4 text-xs">{JSON.stringify(config, null, 2)}</pre>
    </div>
  )
}
