import type { Story } from '@ladle/react'
import { useState } from 'react'
import { CurlDialog, ExportType } from './CurlDialog'

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
      <CurlDialog
        open={open}
        onClose={() => setOpen(false)}
        apiHost="https://app.sentio.xyz"
        apiUrl="/api/v1/analytics/projectOwner/projectSlug/sql/execute"
        payload={{
          projectOwner: 'acme',
          projectSlug: 'demo',
          query: 'SELECT 1',
          size: 100
        }}
        defaultType={ExportType.CURL}
      />
    </div>
  )
}
