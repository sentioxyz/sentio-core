import type { Story } from '@ladle/react'
import { useState } from 'react'
import { AddPanel } from './AddPanel'
import { AddPanelSlideover } from './AddPanelSlideover'

const routerQuery = { owner: 'acme', slug: 'demo', id: 'dash1' }

export const Button: Story = () => (
  <div className="p-8">
    <AddPanel
      allowEdit
      saving={false}
      routerQuery={routerQuery}
      onNewPanel={(c) => console.log('new panel', c)}
      onImportPanel={() => console.log('import')}
    />
  </div>
)

export const Slideover: Story = () => {
  const [open, setOpen] = useState(true)
  return (
    <div className="p-8">
      <button
        className="border-main rounded-md border px-3 py-1 text-sm"
        onClick={() => setOpen(true)}
      >
        Open
      </button>
      <AddPanelSlideover
        open={open}
        onClose={() => setOpen(false)}
        routerQuery={routerQuery}
        onSelect={(t) => console.log('select', t)}
        allowImport
      />
    </div>
  )
}
