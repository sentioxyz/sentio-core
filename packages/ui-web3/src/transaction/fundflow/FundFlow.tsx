import { Empty } from '@sentio/ui-core'
import { memo, ReactNode } from 'react'
import { VizGraph } from './GraphvizGraph'

interface Props {
  nodes?: any[]
  edges?: any[]
  zoomable?: boolean
  renderGraph?: (graphString: string, zoomable?: boolean) => ReactNode
}

export const FundFlow = memo(function Fundflow({
  nodes = [],
  edges = [],
  zoomable,
  renderGraph
}: Props) {
  if (nodes.length === 0) {
    return (
      <div className="h-full">
        <div className="absolute bottom-0 left-0 right-0 top-0 z-[1] pt-32">
          <Empty title="This transaction has no fund flow" />
        </div>
      </div>
    )
  }

  const graphString = `
digraph {
  fontname="Courier New"
  class="flow-chart"
  bgcolor="transparent"
  nodesep=0.4
  rankdir="LR"
  ratio=auto
  node [shape=rect style="filled"]
  ${nodes.join('\n')}
  ${edges.join('\n')}
}
`

  // If custom render function is provided, use it
  if (renderGraph) {
    return <>{renderGraph(graphString, zoomable)}</>
  }

  // Default to VizGraph renderer
  /**
   * Graphviz cannot support Roboto font, use "Courier New" font instead to get monospace width.
   * Override the font to Roboto and 12px in the style.
   */
  return <VizGraph zoomable={zoomable} graphString={graphString} />
})
