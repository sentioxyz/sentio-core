import type { Story } from '@ladle/react'
import LineIcon from './LineIcon'
import AreaIcon from './AreaIcon'
import BarIcon from './BarIcon'
import BarGuageIcon from './BarGuageIcon'
import PieIcon from './PieIcon'
import QueryValueIcon from './QueryValueIcon'
import ScatterIcon from './ScatterIcon'
import TableIcon from './TableIcon'

const icons: [string, (props: { className?: string }) => JSX.Element][] = [
  ['Line', LineIcon],
  ['Area', AreaIcon],
  ['Bar', BarIcon],
  ['BarGuage', BarGuageIcon],
  ['Pie', PieIcon],
  ['QueryValue', QueryValueIcon],
  ['Scatter', ScatterIcon],
  ['Table', TableIcon]
]

export const All: Story = () => (
  <div className="text-text-foreground grid grid-cols-4 gap-6 p-8">
    {icons.map(([name, Icon]) => (
      <div key={name} className="flex flex-col items-center gap-2">
        <Icon className="h-6 w-6" />
        <span className="text-text-foreground-secondary text-xs">{name}</span>
      </div>
    ))}
  </div>
)
All.meta = { description: 'Chart-type icons (use currentColor)' }
