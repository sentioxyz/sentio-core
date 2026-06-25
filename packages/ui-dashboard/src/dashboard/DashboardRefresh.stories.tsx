import type { Story } from '@ladle/react'
import { DashboardRefresh } from './DashboardRefresh'

export const Default: Story = () => (
  <div className="flex w-full items-center gap-4">
    <DashboardRefresh
      stats={{ computedAt: new Date().toISOString() }}
      onRefresh={async () => {}}
    />
    <DashboardRefresh
      stats={{
        computedAt: new Date(Date.now() - 3 * 3600 * 1000).toISOString()
      }}
      onRefresh={async () => {}}
    />
    <DashboardRefresh
      stats={{ computedAt: new Date().toISOString(), isRefreshing: true }}
      onRefresh={async () => {}}
    />
  </div>
)
