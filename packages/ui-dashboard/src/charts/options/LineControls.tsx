import { produce } from 'immer'
import {
  DisclosurePanel,
  NewButtonGroup as ButtonGroup,
  Checkbox
} from '@sentio/ui-core'
import type { LineConfigLike, LineStyleLike } from '../../types'

const lineStyles: { label: string; value: LineStyleLike }[] = [
  { label: 'Solid', value: 'Solid' },
  { label: 'Dotted', value: 'Dotted' }
]

interface Props {
  config?: LineConfigLike
  defaultOpen?: boolean
  onChange: (config: LineConfigLike) => void
}

export const LineControls = ({ config, defaultOpen, onChange }: Props) => {
  const setStyle = (style: LineStyleLike) => {
    onChange(
      produce(config || {}, (draft) => {
        draft.style = style
      })
    )
  }
  const setSmooth = (smooth: boolean) => {
    onChange(
      produce(config || {}, (draft) => {
        draft.smooth = smooth
      })
    )
  }
  return (
    <DisclosurePanel
      title="Line style"
      containerClassName="w-full bg-default-bg"
    >
      <div className="flex items-center gap-4">
        <ButtonGroup
          buttons={lineStyles}
          value={config?.style || 'Solid'}
          theme="light"
          onChange={setStyle}
        />
        <Checkbox
          label="Smooth Curves"
          checked={config?.smooth}
          onChange={setSmooth}
        />
      </div>
    </DisclosurePanel>
  )
}
