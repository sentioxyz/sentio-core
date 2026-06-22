import type { Story } from '@ladle/react'
import { useState } from 'react'
import { MarkerControls } from './MarkerControls'
import type { MarkerLike } from '../../types'

export const Basic: Story = () => {
  const [markers, setMarkers] = useState<MarkerLike[]>([
    { type: 'LINE', value: 100, color: '#ff0000', label: 'threshold' }
  ])
  return (
    <div className="w-[48rem] p-8">
      <MarkerControls markers={markers} onChange={setMarkers} />
      <pre className="text-text-foreground-secondary mt-4 text-xs">
        {JSON.stringify(markers, null, 2)}
      </pre>
    </div>
  )
}
Basic.meta = {
  description: 'Horizontal/vertical line markers (value/color/label)'
}
