import '../../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { DateInput } from './DateInput'
import { PresetPicker } from './PresetPicker'
import { TimeInput } from './TimeInput'
import { TimeRangeLabel } from './TimeRangeLabel'
import {
  ago,
  now,
  parseDateTime,
  dateTimeToString,
  DateTimeValue,
  fromNumber
} from '../../utils/time'

export const DateInputDefault: Story = () => {
  const [value, setValue] = useState<DateTimeValue>(ago(5, 'minutes'))

  return (
    <div className="max-w-sm space-y-4 p-8">
      <h3 className="text-lg font-semibold">Date Input</h3>
      <DateInput
        value={value}
        onChange={(text: string) => {
          const parsed = parseDateTime(text)
          if (parsed) {
            setValue(parsed)
          }
        }}
      />
      <div className="border-border-color bg-default-bg text-text-foreground-secondary rounded-md border p-3 text-sm">
        Current value:{' '}
        <span className="text-text-foreground font-medium">
          {dateTimeToString(value)}
        </span>
      </div>
    </div>
  )
}

DateInputDefault.meta = {
  description:
    'Editable date input with support for relative time values like now-5m.'
}

export const TimeInputDefault: Story = () => {
  const [value, setValue] = useState('08:30')

  return (
    <div className="max-w-sm space-y-4 p-8">
      <h3 className="text-lg font-semibold">Time Input</h3>
      <div className="flex items-center gap-4">
        <TimeInput value={value} disabled={false} onChange={setValue} />
        <span className="text-text-foreground-secondary text-sm">
          Selected: {value}
        </span>
      </div>
    </div>
  )
}

TimeInputDefault.meta = {
  description:
    'Editable hour/minute picker with arrow key support and smart focus behavior.'
}

export const TimeInputDisabled: Story = () => {
  return (
    <div className="max-w-sm space-y-4 p-8">
      <h3 className="text-lg font-semibold">Disabled Time Input</h3>
      <TimeInput value="00:00" disabled={true} onChange={() => {}} />
    </div>
  )
}

TimeInputDisabled.meta = {
  description: 'Read-only disabled state for the time picker.'
}

export const PresetPickerDefault: Story = () => {
  const [range, setRange] = useState<[DateTimeValue, DateTimeValue]>([
    ago(15, 'minutes'),
    now
  ])

  return (
    <div className="border-border-color relative max-w-xl overflow-hidden rounded-xl border p-8 shadow-sm">
      <h3 className="mb-4 text-lg font-semibold">Preset Picker</h3>
      <div className="mb-4 grid gap-4 sm:grid-cols-[220px_minmax(0,1fr)]">
        <div className="border-border-color bg-default-bg relative h-[400px] rounded-lg border p-4">
          <PresetPicker
            onSelect={(start: DateTimeValue, end: DateTimeValue) =>
              setRange([start, end])
            }
          />
        </div>
        <div className="border-border-color rounded-lg border p-4">
          <p className="text-text-foreground mb-2 text-sm font-medium">
            Selected range
          </p>
          <div className="text-text-foreground-secondary space-y-2 text-sm">
            <div>From: {dateTimeToString(range[0])}</div>
            <div>To: {dateTimeToString(range[1])}</div>
          </div>
        </div>
      </div>
    </div>
  )
}

PresetPickerDefault.meta = {
  description:
    'Quick range selection panel that updates the active time range when a preset is clicked.'
}

export const TimeRangeLabelDefault: Story = () => {
  return (
    <div className="max-w-sm space-y-3 p-8">
      <h3 className="text-lg font-semibold">Time Range Label</h3>
      <div className="space-y-2">
        <TimeRangeLabel startTime={ago(15, 'minutes')} endTime={now} />
        <TimeRangeLabel startTime={ago(1, 'days')} endTime={now} />
        <TimeRangeLabel
          startTime={fromNumber(Date.now() - 3600 * 1000)}
          endTime={fromNumber(Date.now())}
        />
      </div>
    </div>
  )
}

TimeRangeLabelDefault.meta = {
  description:
    'Compact label showing a human-friendly time range for relative or absolute values.'
}

export const TimerangePlayground: Story = () => {
  const [dateValue, setDateValue] = useState<DateTimeValue>(ago(1, 'hours'))
  const [timeValue, setTimeValue] = useState('14:00')
  const [range, setRange] = useState<[DateTimeValue, DateTimeValue]>([
    ago(15, 'minutes'),
    now
  ])

  return (
    <div className="space-y-6 p-8">
      <h3 className="text-lg font-semibold">Timerange Playground</h3>
      <div className="grid gap-6 lg:grid-cols-2">
        <div className="border-border-color bg-default-bg rounded-lg border p-4">
          <p className="text-text-foreground mb-3 text-sm font-medium">
            Date input
          </p>
          <DateInput
            value={dateValue}
            onChange={(text: string) => {
              const parsed = parseDateTime(text)
              if (parsed) {
                setDateValue(parsed)
              }
            }}
          />
          <p className="text-text-foreground-secondary mt-3 text-sm">
            Current: {dateTimeToString(dateValue)}
          </p>
        </div>
        <div className="border-border-color bg-default-bg rounded-lg border p-4">
          <p className="text-text-foreground mb-3 text-sm font-medium">
            Time input
          </p>
          <TimeInput
            value={timeValue}
            disabled={false}
            onChange={setTimeValue}
          />
          <p className="text-text-foreground-secondary mt-3 text-sm">
            Selected: {timeValue}
          </p>
        </div>
      </div>
      <div className="border-border-color bg-default-bg rounded-lg border p-4">
        <p className="text-text-foreground mb-3 text-sm font-medium">
          Preset picker
        </p>
        <div className="border-border-color relative min-h-[240px] overflow-hidden rounded-lg border">
          <PresetPicker
            onSelect={(start: DateTimeValue, end: DateTimeValue) =>
              setRange([start, end])
            }
          />
        </div>
        <div className="mt-4">
          <TimeRangeLabel startTime={range[0]} endTime={range[1]} />
        </div>
      </div>
    </div>
  )
}

TimerangePlayground.meta = {
  description:
    'Interactive demo combining date input, time input, preset picker and time range label.'
}
