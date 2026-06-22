import { useState } from 'react'
import { useResizeDetector } from 'react-resize-detector'
import { NewButtonGroup as ButtonGroup } from '@sentio/ui-core'
import type { ChartTypeLike } from '../types'
import BarGuageIcon from './icons/BarGuageIcon'
import QueryValueIcon from './icons/QueryValueIcon'
import TableIcon from './icons/TableIcon'
import AreaIcon from './icons/AreaIcon'
import BarIcon from './icons/BarIcon'
import LineIcon from './icons/LineIcon'
import PieIcon from './icons/PieIcon'
import ScatterIcon from './icons/ScatterIcon'

const visuals: {
  label: string
  value: ChartTypeLike
  icon: React.ReactNode
}[] = [
  {
    label: 'Lines',
    value: 'LINE',
    icon: <LineIcon className="mr-1 h-4 w-4" />
  },
  { label: 'Bars', value: 'BAR', icon: <BarIcon className="mr-1 h-4 w-4" /> },
  {
    label: 'Areas',
    value: 'AREA',
    icon: <AreaIcon className="mr-1 h-4 w-4" />
  },
  {
    label: 'Bar Gauge',
    value: 'BAR_GAUGE',
    icon: <BarGuageIcon className="mr-1 h-4 w-4" />
  },
  {
    label: 'Scatter',
    value: 'SCATTER',
    icon: <ScatterIcon className="mr-1 h-4 w-4" />
  },
  {
    label: 'Query Value',
    value: 'QUERY_VALUE',
    icon: <QueryValueIcon className="mr-1 h-4 w-4" />
  },
  {
    label: 'Table',
    value: 'TABLE',
    icon: <TableIcon className="mr-1 h-4 w-4" />
  },
  { label: 'Pie', value: 'PIE', icon: <PieIcon className="mr-1 h-4 w-4" /> }
]

type Props = {
  value: ChartTypeLike
  onChange: (value: ChartTypeLike) => void
  small?: boolean
}

export const ChartTypeButtonGroup = ({
  value,
  onChange,
  small = false
}: Props) => {
  const [hideLabel, setHideLabel] = useState(small)
  const { ref } = useResizeDetector<HTMLDivElement>({
    onResize: ({ width }) => {
      if (width) {
        setHideLabel(width < 800)
      }
    },
    refreshMode: 'throttle',
    refreshRate: 100
  })
  return (
    <div className="w-full flex-1" ref={ref}>
      <ButtonGroup
        buttons={visuals}
        value={value}
        onChange={onChange}
        small={small}
        hideLabel={hideLabel}
      />
    </div>
  )
}
