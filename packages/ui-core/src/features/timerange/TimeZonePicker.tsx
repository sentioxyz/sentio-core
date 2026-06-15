import { useMemo } from 'react'
import { ComboInput } from '../../common/input/ComboInput'
import { DEFAULT_TZ } from '../../utils/time'
import { classNames } from '../../utils/classnames'

interface Props {
  value: string
  onChange: (value?: string) => void
}

// Intl.supportedValuesOf is ES2022; ui-core targets ES2020, so access it through
// a narrow cast rather than widening the lib.
const intlSupportedValuesOf = (key: string): string[] =>
  (
    Intl as unknown as { supportedValuesOf: (k: string) => string[] }
  ).supportedValuesOf(key)

export function TimeZonePicker({ value, onChange }: Props) {
  const { timeZones, timeZoneOffset } = useMemo(() => {
    const timeZones = [
      ' ',
      'UTC',
      ...intlSupportedValuesOf('timeZone').filter((x) => x != 'UTC')
    ]
    const now = new Date()
    const timeZoneOffset: Record<string, string> = {}
    timeZones.forEach((timeZone) => {
      const dateString = Intl.DateTimeFormat([], {
        timeZone: timeZone.trim() || DEFAULT_TZ,
        timeZoneName: 'longOffset'
      }).format(now)
      const offset = dateString.split(' ')[1].replace('GMT', 'UTC')
      timeZoneOffset[timeZone] = offset
    })
    return {
      timeZones,
      timeZoneOffset
    }
  }, [])

  return (
    <div className="grid grid-cols-1 items-center gap-y-2 sm:flex sm:gap-y-0">
      <div className="flex shrink-0 items-center">
        <span className="text-ilabel mr-2.5 font-medium">Time Zone</span>
      </div>
      <ComboInput
        className="text-icontent focus:ring-primary-600/30 focus:border-primary-500 border-border-color sm:w-54 block h-8 w-full rounded-md"
        inputClassName="rounded-md"
        value={''}
        options={timeZones}
        displayFn={(z) => (
          <div
            className={classNames(
              'group flex items-center justify-between gap-3'
            )}
          >
            {z == 'UTC' && <div className="absolute inset-0 border-b" />}
            <div className="inline-flex items-center">
              {z.trim() ? z : 'Browser Time'}
              {z == DEFAULT_TZ && (
                <span className="group-hover:bg-primary text-primary-600 ml-1 inline-block rounded-sm bg-blue-100 px-1 py-0.5 text-[11px] dark:bg-blue-800">
                  Browser time
                </span>
              )}
            </div>
            <span className="text-text-foreground rounded-sm border bg-gray-50 px-1 text-[11px]">
              {timeZoneOffset[z]}
            </span>
          </div>
        )}
        onChange={(v) => onChange(v?.trim() || undefined)}
        placeholder={
          <div className="flex items-center justify-between">
            <span
              className="text-text-foreground truncate text-xs"
              title={value}
            >
              {value.trim() || 'Browser Time'}
            </span>
            <span className="text-text-foreground rounded-sm border bg-gray-50 px-1 py-0.5 text-[11px] leading-[14px]">
              {timeZoneOffset[value]}
            </span>
          </div>
        }
      />
    </div>
  )
}
