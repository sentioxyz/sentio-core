import { produce } from 'immer'
import { LuMinus, LuPlus } from 'react-icons/lu'
import { Button, DisclosurePanel } from '@sentio/ui-core'
import type { MarkerLike } from '../../types'

interface Props {
  markers?: MarkerLike[]
  onChange: (v: MarkerLike[]) => void
}

const labelCls =
  'inline-flex items-center border border-r-0 sm:text-icontent border-main  bg-gray-50 px-2 rounded-l-md'
const inputCls =
  'border focus:border-primary-500 rounded-r-md sm:text-icontent border-main w-28 hover:border-primary-600 focus:ring-3 focus:ring-primary-600/30'

function MarkerInput({
  marker,
  label,
  onChange,
  onRemove
}: {
  marker: MarkerLike
  label: string
  onChange: (v: MarkerLike) => void
  onRemove: () => void
}) {
  const _onChange = (field: string, value: any) => {
    onChange(
      produce(marker, (draft) => {
        ;(draft as Record<string, any>)[field] = value
      })
    )
  }
  return (
    <div className="flex items-center gap-[10px]">
      <label className="inline-flex h-8">
        <span className={labelCls}>
          <span className="pr-2">{label}</span>
          <select
            className="sm:text-ilabel border-main text-text-foreground inline-flex h-full items-center border border-b-0 border-t-0 bg-gray-50 p-0 pl-4 pr-7 focus:border-transparent focus:ring-inset"
            value={marker.type}
            onChange={(e) => _onChange('type', e.target.value)}
          >
            <option value={'LINE'}>horizontal line</option>
            <option value={'LINEX'}>vertical line</option>
          </select>
          <span className="pl-2">at</span>
        </span>
        {marker.type === 'LINEX' ? (
          <input
            className={inputCls}
            type="text"
            value={marker.valueX}
            placeholder="YYYY-MM-DD"
            onChange={(e) => _onChange('valueX', e.target.value)}
          />
        ) : (
          <input
            className={inputCls}
            type="text"
            value={marker.value}
            onChange={(e) => _onChange('value', e.target.value)}
          />
        )}
      </label>
      <label className="inline-flex h-8">
        <span className={labelCls}>Color</span>
        <div className="relative">
          <div className="absolute inset-0 flex w-8 items-center justify-center">
            <div className="h-4 w-4" style={{ background: marker.color }} />
          </div>
          <input
            className={inputCls + ' pl-8'}
            type="text"
            value={marker.color}
            onChange={(e) => _onChange('color', e.target.value)}
          />
        </div>
      </label>
      <label className="inline-flex h-8">
        <span className={labelCls}>Label</span>
        <input
          className={inputCls}
          type="text"
          value={marker.label}
          onChange={(e) => _onChange('label', e.target.value)}
        />
      </label>
      <button
        type="button"
        className="ml-2 flex h-4 w-4 cursor-pointer items-center justify-center rounded-full bg-gray-800 hover:bg-red-600"
        aria-label="Remove marker"
        onClick={onRemove}
      >
        <LuMinus
          className="dark:text-default-bg h-3 w-3 text-white"
          aria-hidden="true"
        />
      </button>
    </div>
  )
}

const DEFAULT_MARKER: MarkerLike = { value: 0, color: '#ff0000', label: '' }

export function MarkerControls({ markers, onChange }: Props) {
  const _markers = markers?.length ? markers : []

  const onAdd = () => {
    onChange(
      produce(_markers, (draft) => {
        draft.push(DEFAULT_MARKER)
      })
    )
  }

  const onRemove = (index: number) => {
    onChange(
      produce(_markers, (draft) => {
        draft.splice(index, 1)
      })
    )
  }

  const _onChange = (index: number, marker: MarkerLike) => {
    onChange(
      produce(_markers, (draft) => {
        draft[index] = marker
      })
    )
  }

  return (
    <DisclosurePanel
      title="Markers"
      containerClassName="w-full bg-default-bg"
      defaultOpen={true}
    >
      <div className="space-y-2">
        {_markers.map((marker, index) => (
          <MarkerInput
            marker={marker}
            key={index}
            label={String.fromCharCode(65 + (index % 26))}
            onChange={(v) => _onChange(index, v)}
            onRemove={() => onRemove(index)}
          />
        ))}
        <div>
          <Button type="button" onClick={onAdd}>
            <LuPlus className="h-4 w-4" aria-hidden="true" />
            Add Marker
          </Button>
        </div>
      </div>
    </DisclosurePanel>
  )
}
