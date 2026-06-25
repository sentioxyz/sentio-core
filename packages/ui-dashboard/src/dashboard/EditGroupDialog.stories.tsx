import type { Story } from '@ladle/react'
import { useState } from 'react'
import { EditGroupDialog } from './EditGroupDialog'
import type { GroupStyleLike } from '../types/enums'

export const Basic: Story = () => {
  const [open, setOpen] = useState(true)
  const [style, setStyle] = useState<GroupStyleLike>('EMPHASIS')
  const [color, setColor] = useState('green')
  return (
    <div className="p-8">
      <button
        className="border-main rounded-md border px-3 py-1 text-sm"
        onClick={() => setOpen(true)}
      >
        Open dialog
      </button>
      <EditGroupDialog
        open={open}
        title="My Group"
        style={style}
        highlightColor={color}
        onClose={() => setOpen(false)}
        onSave={(next) => {
          setStyle(next.style)
          setColor(next.highlightColor)
          setOpen(false)
        }}
      />
    </div>
  )
}
