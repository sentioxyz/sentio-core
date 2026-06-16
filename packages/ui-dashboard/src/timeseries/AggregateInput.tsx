import { useMemo } from 'react'
import { capitalize } from 'lodash'
import { produce } from 'immer'
import { NewMultipleSelect } from '@sentio/ui-core'
import type {
  AggregateOpsLike,
  MetricInfoLike,
  QueryLike
} from '../types/metrics'
import { SystemLabels } from './labels'

const AggregateAggregateOps: AggregateOpsLike[] = [
  'AVG',
  'SUM',
  'MIN',
  'MAX',
  'COUNT'
]

interface Props {
  metric?: MetricInfoLike
  value: QueryLike
  onChange: (value: QueryLike) => void
}

type Label = { label: string; value: string }

export function AggregateInput({ metric, value, onChange }: Props) {
  const { labels, selectedLabels } = useMemo(() => {
    const labels: Label[] = []
    for (const sl of SystemLabels) {
      labels.push({ label: sl.name, value: sl.field })
    }
    Object.keys(metric?.labels || {}).forEach((l) => {
      labels.push({ label: l, value: l })
    })
    const selectedLabels: Label[] = []
    for (const l of value?.aggregate?.grouping || []) {
      const label = labels.find((lb) => lb.value === l)
      if (label) {
        selectedLabels.push(label)
      }
    }

    return { labels, selectedLabels }
  }, [metric, value])

  const onSelectLabel = (labels: Label[]) => {
    onChange(
      produce(value, (draft) => {
        draft.aggregate = draft.aggregate || {}
        draft.aggregate.grouping = labels.map((l) => l.value)
      })
    )
  }

  const onSelectFunc = (f: string) => {
    onChange(
      produce(value, (draft) => {
        if (f == 'none') {
          delete draft.aggregate
        } else {
          const aggr = draft.aggregate || {}
          aggr.op = f as AggregateOpsLike
          draft.aggregate = aggr
        }
      })
    )
  }

  return (
    <div className="min-h-8 flex grow items-center ">
      <select
        value={value.aggregate?.op || ''}
        className="sm:text-ilabel border-main text-text-foreground inline-flex h-full items-center rounded-l-md border border-r-0 bg-gray-50 p-0 pl-4 pr-7 focus:border-0 focus:ring-inset"
        onChange={(e) => onSelectFunc(e.target.value)}
        aria-label="aggregate"
      >
        <option key="" value={'none'}>
          No aggregate
        </option>
        {AggregateAggregateOps.map((key) => {
          return (
            <option key={key} value={key}>
              {capitalize(key)} by
            </option>
          )
        })}
      </select>
      <NewMultipleSelect<Label>
        disabled={!value.aggregate}
        className="border-main flex h-full grow overflow-hidden rounded-r-md border"
        options={labels || []}
        value={selectedLabels}
        onChange={onSelectLabel}
        displayFn={(l) => l.label}
        unSelectedText="(everything)"
        optionsClassName="min-w-[200px]"
      />
    </div>
  )
}
