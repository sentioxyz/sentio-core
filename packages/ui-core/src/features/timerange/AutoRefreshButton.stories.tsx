import '../../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { AutoRefreshButton } from './AutoRefreshButton'

export const Basic: Story = () => {
  const [autoRefresh, setAutoRefresh] = useState(0)
  const [count, setCount] = useState(0)
  return (
    <div className="p-8">
      <AutoRefreshButton
        autoRefresh={autoRefresh}
        onAutoRefreshChange={setAutoRefresh}
        onClick={() => setCount((c) => c + 1)}
      />
      <p className="text-text-foreground-secondary mt-4 text-sm">
        interval: {autoRefresh}ms · refreshed {count}×
      </p>
    </div>
  )
}
Basic.meta = {
  description: 'Refresh button with controlled auto-refresh interval'
}
