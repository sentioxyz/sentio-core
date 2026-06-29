import type { Story } from '@ladle/react'
import { useState } from 'react'
import { ImportDashboardDialog } from './ImportDashboardDialog'

export const Default: Story = () => {
  const [open, setOpen] = useState(false)
  return (
    <div className="p-8">
      <button
        className="bg-primary-600 hover:bg-primary-700 rounded px-4 py-2 text-sm text-white"
        onClick={() => setOpen(true)}
      >
        Open Import Dialog
      </button>
      <ImportDashboardDialog
        open={open}
        onClose={() => setOpen(false)}
        onImport={async (json) => {
          // eslint-disable-next-line no-console
          console.log('import', json)
        }}
      />
    </div>
  )
}
