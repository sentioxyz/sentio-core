import type { Story } from '@ladle/react'
import { useState } from 'react'
import { ImportPanelDialog } from './ImportPanelDialog'

export const Default: Story = () => {
  const [show, setShow] = useState(false)
  return (
    <div className="p-8">
      <button
        className="bg-primary-600 hover:bg-primary-700 rounded px-4 py-2 text-sm text-white"
        onClick={() => setShow(true)}
      >
        Open Import Dialog
      </button>
      <ImportPanelDialog
        show={show}
        onClose={() => setShow(false)}
        onSubmit={async (p) => {
          alert(JSON.stringify(p, null, 2))
          setShow(false)
        }}
      />
    </div>
  )
}
