import type { Story } from '@ladle/react'
import { useState } from 'react'
import { ShareDashboardDialog } from './ShareDashboardDialog'
import type { SharingConfigLike } from '../types/dashboard'

export const Basic: Story = () => {
  const [open, setOpen] = useState(true)
  const [config, setConfig] = useState<SharingConfigLike>({ isReadonly: true })
  return (
    <div className="p-8">
      <button
        className="border-main rounded-md border px-3 py-1 text-sm"
        onClick={() => setOpen(true)}
      >
        Open dialog
      </button>
      <ShareDashboardDialog
        open={open}
        initData={{ id: 'abc123', config }}
        onClose={() => setOpen(false)}
        onUnshare={() => setOpen(false)}
        onConfigChange={setConfig}
      />
    </div>
  )
}
