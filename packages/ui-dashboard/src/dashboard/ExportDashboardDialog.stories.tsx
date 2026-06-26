import type { Story } from '@ladle/react'
import { useState } from 'react'
import { ExportDashboardDialog } from './ExportDashboardDialog'

const sampleJson = JSON.stringify(
  {
    name: 'My Dashboard',
    panels: [
      { name: 'Panel 1', chart: { type: 'LINE', datasourceType: 'METRICS' } }
    ]
  },
  null,
  2
)

export const Default: Story = () => {
  const [open, setOpen] = useState(false)
  return (
    <div className="p-8">
      <button
        className="bg-primary-600 hover:bg-primary-700 rounded px-4 py-2 text-sm text-white"
        onClick={() => setOpen(true)}
      >
        Open Export Dialog
      </button>
      <ExportDashboardDialog
        open={open}
        onClose={() => setOpen(false)}
        dashboardId="dashboard-123"
        json={sampleJson}
      />
    </div>
  )
}
