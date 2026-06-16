import { useState } from 'react'
import { BaseDialog } from '../../common/dialog/BaseDialog'
import { Checkbox } from '../../common/Checkbox'
import { formatTimeRange, formatTimeZone } from './utils'
import {
  DateTimeValue,
  dateTimeToString,
  parseDateTime
} from '../../utils/time'

/** A project's persisted default time range (string-encoded start/end + tz). */
export interface DefaultTimerangeValue {
  start?: string
  end?: string
  timezone?: string
}

interface Props {
  start?: DateTimeValue
  end?: DateTimeValue
  tz?: string
  /** The current persisted default time range, for display and as the merge base. */
  currentDefault?: DefaultTimerangeValue
  /**
   * Persist the chosen default time range. The consumer owns the actual save
   * (and any success/error notification); rejecting aborts confirm/close.
   */
  onSaveDefault: (defaultTimerange: DefaultTimerangeValue) => Promise<void>
  onConfirm: (start?: DateTimeValue, end?: DateTimeValue, tz?: string) => void
  onClose: () => void
}

export const DefaultTimeConfirmDialog = ({
  start,
  end,
  tz,
  currentDefault,
  onSaveDefault,
  onConfirm,
  onClose
}: Props) => {
  const [rangeChecked, setRangeChecked] = useState(true)
  const [zoneChecked, setZoneChecked] = useState(true)

  const onOk = async () => {
    const defaultTimerange: DefaultTimerangeValue = { ...currentDefault }
    if (rangeChecked) {
      defaultTimerange.start = dateTimeToString(start)
      defaultTimerange.end = dateTimeToString(end)
    }
    if (zoneChecked) {
      defaultTimerange.timezone = tz
    }
    await onSaveDefault(defaultTimerange)
    onConfirm(
      parseDateTime(defaultTimerange.start) || undefined,
      parseDateTime(defaultTimerange.end) || undefined,
      defaultTimerange.timezone
    )
    onClose()
  }

  const currentRange =
    currentDefault?.start && currentDefault?.end
      ? formatTimeRange(
          parseDateTime(currentDefault.start),
          parseDateTime(currentDefault.end)
        )
      : 'Default value'
  return (
    <BaseDialog
      open={true}
      title="Update project default time"
      okText="Update"
      onOk={onOk}
      onCancel={onClose}
      onClose={onClose}
    >
      <div className="px-4 py-8">
        <div className="text-ilabel mb-2 font-medium">Default time range</div>
        <Checkbox
          checked={rangeChecked}
          onChange={setRangeChecked}
          labelNode={
            <div className="text-xs">
              <span className="text-text-foreground-secondary">
                {currentRange}
              </span>
              <span className="mx-4">{'->'}</span>
              <span className="font-medium">{formatTimeRange(start, end)}</span>
            </div>
          }
        />
        <div className="text-ilabel mb-2 mt-5 font-medium">
          Default time zone
        </div>
        <Checkbox
          checked={zoneChecked}
          onChange={setZoneChecked}
          labelNode={
            <div className="text-xs">
              <span className="text-text-foreground-secondary">
                {currentDefault?.timezone || 'Browser Time'}
              </span>
              {currentDefault?.timezone && (
                <span className="dark:text-text-foreground text-text-foreground-secondary ml-1.5 rounded-sm border bg-gray-50 px-1 text-[11px]">
                  {formatTimeZone(currentDefault.timezone)}
                </span>
              )}
              <span className="mx-4">{'->'}</span>
              <span className="font-medium">{tz}</span>
              {tz ? (
                <span className="text-text-foreground ml-1.5 rounded-sm border bg-gray-50 px-1 text-[11px]">
                  {formatTimeZone(tz)}
                </span>
              ) : (
                'Browser Time'
              )}
            </div>
          }
        />
      </div>
    </BaseDialog>
  )
}
