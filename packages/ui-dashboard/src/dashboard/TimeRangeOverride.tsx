import { defaults } from 'lodash'
import { produce } from 'immer'
import { Switch, TimeRangePicker } from '@sentio/ui-core'
import type { DateTimeValue } from '@sentio/ui-core'
import type { TimeRangeOverrideLike } from '../types/chart'
import { fromTimeLike, toTimeLike } from '../charts/time-utils'

interface Props {
  config?: TimeRangeOverrideLike
  onChange: (config: TimeRangeOverrideLike) => void
  // Global dashboard time range, injected by the app (it owns the data hook /
  // local-storage default). When override is OFF the picker drives these.
  globalStartTime?: DateTimeValue
  globalEndTime?: DateTimeValue
  globalTz?: string
  onSetGlobalTimeRange: (
    start?: DateTimeValue,
    end?: DateTimeValue,
    tz?: string
  ) => void
}

export const defaultConfig: TimeRangeOverrideLike = {
  enabled: false
}

export function TimeRangeOverride({
  config,
  onChange,
  globalStartTime,
  globalEndTime,
  globalTz,
  onSetGlobalTimeRange
}: Props) {
  config = defaults(config || {}, defaultConfig)

  const setEnabled = (enabled: boolean) => {
    config &&
      onChange(
        produce(config, (draft) => {
          draft.enabled = enabled
          if (enabled) {
            draft.timeRange = {
              start: toTimeLike(globalStartTime),
              end: toTimeLike(globalEndTime),
              step: draft.timeRange?.step,
              interval: draft.timeRange?.interval,
              timezone: draft.timeRange?.timezone
            }
          }
        })
      )
  }

  function onTimeRangeChange(
    start?: DateTimeValue,
    end?: DateTimeValue,
    tz?: string
  ) {
    if (config?.enabled) {
      onChange(
        produce(config, (draft) => {
          draft.timeRange = {
            start: toTimeLike(start),
            end: toTimeLike(end),
            timezone: tz,
            step: draft.timeRange?.step,
            interval: draft.timeRange?.interval
          }
        })
      )
    } else {
      onSetGlobalTimeRange(start, end, tz)
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-4 p-2">
      <div className="flex items-center">
        <Switch
          checked={config.enabled || false}
          onChange={setEnabled}
          label="Override Global Time"
        />
      </div>

      <TimeRangePicker
        startTime={
          config.enabled
            ? fromTimeLike(config.timeRange?.start)
            : globalStartTime
        }
        endTime={
          config.enabled ? fromTimeLike(config.timeRange?.end) : globalEndTime
        }
        tz={config.enabled ? config.timeRange?.timezone || globalTz : globalTz}
        onChange={onTimeRangeChange}
      />
    </div>
  )
}
