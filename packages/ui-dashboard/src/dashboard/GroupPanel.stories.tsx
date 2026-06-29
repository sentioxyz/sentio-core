import type { Story } from '@ladle/react'
import { GroupPanel } from './GroupPanel'
import type { PanelLike } from '../types'

const childPanels: PanelLike[] = [
  { id: 'p1', name: 'Panel One' },
  { id: 'p2', name: 'Panel Two' }
]

const groupPanel: PanelLike = {
  id: 'g1',
  chart: {
    type: 'GROUP',
    group: {
      title: 'My Group',
      style: 'EMPHASIS',
      highlightColor: 'green',
      childLayouts: {
        responsiveLayouts: {
          md: {
            layouts: [
              { i: 'p1', x: 0, y: 0, w: 6, h: 4 },
              { i: 'p2', x: 6, y: 0, w: 6, h: 4 }
            ]
          }
        }
      }
    }
  }
}

export const Default: Story = () => (
  <div className="p-8" style={{ height: 480 }}>
    <GroupPanel
      dashboard={{ id: 'd1' }}
      panel={groupPanel}
      childPanels={childPanels}
      allowEdit
      outerCols={12}
      renderChild={(child) => (
        <div className="bg-hover text-text-foreground flex h-full w-full items-center justify-center rounded text-sm">
          {child.name}
        </div>
      )}
    />
  </div>
)
