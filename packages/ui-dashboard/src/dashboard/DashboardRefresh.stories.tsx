import type { Story } from '@ladle/react'
import { DashboardRefresh } from './DashboardRefresh'

export const Fresh: Story = () => (
  <DashboardRefresh
    stats={{ computedAt: new Date().toISOString() }}
    onRefresh={async () => {}}
  />
)

export const Stale: Story = () => (
  <DashboardRefresh
    stats={{ computedAt: new Date(Date.now() - 3 * 3600 * 1000).toISOString() }}
    onRefresh={async () => {}}
  />
)

export const Refreshing: Story = () => (
  <DashboardRefresh
    stats={{ computedAt: new Date().toISOString(), isRefreshing: true }}
    onRefresh={async () => {}}
  />
)
