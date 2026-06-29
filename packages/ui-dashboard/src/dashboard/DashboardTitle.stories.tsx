import type { Story } from '@ladle/react'
import { DashboardTitle } from './DashboardTitle'

const dashboards = [
  { id: 'a', name: 'Overview' },
  { id: 'b', name: 'Revenue' },
  { id: 'c', name: 'Alpha Metrics' }
]

export const Default: Story = () => (
  <div className="p-8">
    <DashboardTitle
      dashboard={dashboards[1]}
      dashboards={dashboards}
      allowEdit
      // eslint-disable-next-line no-console
      onSelectDashboard={(id) => console.log('select', id)}
      // eslint-disable-next-line no-console
      onNewDashboard={() => console.log('new dashboard')}
    />
  </div>
)
