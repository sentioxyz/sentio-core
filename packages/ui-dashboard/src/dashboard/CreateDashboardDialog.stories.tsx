import type { Story } from '@ladle/react'
import { useState } from 'react'
import { CreateDashboardDialog } from './CreateDashboardDialog'

export const Default: Story = () => {
  const [open, setOpen] = useState(false)
  return (
    <div className="p-8">
      <button
        className="bg-primary-600 hover:bg-primary-700 rounded px-4 py-2 text-sm text-white"
        onClick={() => setOpen(true)}
      >
        Open Create Dialog
      </button>
      <CreateDashboardDialog
        open={open}
        onClose={() => setOpen(false)}
        defaultName="Alice's Dashboard Mon, Jun 29, 10:30:00 am"
        projectId="project-123"
        ownerId="user-1"
        showExternal
        onCreate={async (data) => {
          // eslint-disable-next-line no-console
          console.log('create', data)
        }}
      />
    </div>
  )
}
