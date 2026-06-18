import type { Story } from '@ladle/react'
import { useState } from 'react'
import { RefreshContext, RefreshButton } from './RefreshContext'

export const Basic: Story = () => {
  const [refreshing, setRefreshing] = useState(false)
  return (
    <div className="p-8">
      <RefreshContext.Provider
        value={{
          isRefreshing: refreshing,
          refresh: () => {
            setRefreshing(true)
            setTimeout(() => setRefreshing(false), 1000)
          }
        }}
      >
        <RefreshButton />
      </RefreshContext.Provider>
    </div>
  )
}
Basic.meta = {
  description:
    'RefreshButton — only renders when a refresh callback is provided via context'
}
