import type { Story } from '@ladle/react'
import { useState } from 'react'
import { FunctionsPanel } from './FunctionsPanel'
import type { FunctionDef } from './functions'

export const Basic: Story = () => {
  const [picked, setPicked] = useState<string>()
  return (
    <div className="border-light h-72 w-[28rem] border">
      <FunctionsPanel onClick={(f: FunctionDef) => setPicked(f.name)} />
      <p className="text-text-foreground-secondary p-2 text-sm">
        Picked: {picked || '—'}
      </p>
    </div>
  )
}
Basic.meta = {
  description:
    'Categorized function picker (hover a category, click a function)'
}
