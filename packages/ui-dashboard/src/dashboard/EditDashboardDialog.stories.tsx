import type { Story } from '@ladle/react'
import { useState } from 'react'
import { EditDashboardDialog } from './EditDashboardDialog'
import type { DashboardLike } from '../types/dashboard'

const dashboard: DashboardLike = { id: 'd1', name: 'My Dashboard' }

export const Basic: Story = () => {
  const [open, setOpen] = useState(true)
  return (
    <div className="p-8">
      <button
        className="border-main rounded-md border px-3 py-1 text-sm"
        onClick={() => setOpen(true)}
      >
        Open dialog
      </button>
      <EditDashboardDialog
        open={open}
        dashboard={dashboard}
        onClose={() => setOpen(false)}
        onUpdate={async (data) => {
          // eslint-disable-next-line no-console
          console.log('update', data)
        }}
      />
    </div>
  )
}
