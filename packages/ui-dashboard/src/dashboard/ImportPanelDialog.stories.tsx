import type { Story } from '@ladle/react'
import { useState } from 'react'
import { ImportPanelDialog } from './ImportPanelDialog'

export const Default: Story = () => {
  const [show, setShow] = useState(true)
  return (
    <ImportPanelDialog
      show={show}
      onClose={() => setShow(false)}
      onSubmit={async (p) => {
        alert(JSON.stringify(p, null, 2))
        setShow(false)
      }}
    />
  )
}
