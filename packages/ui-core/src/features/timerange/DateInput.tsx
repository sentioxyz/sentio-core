import { DateTimeValue, relativeTimeToString } from '../../utils/time'
import { useEffect, useState } from 'react'
import dayjs from 'dayjs'
import duration from 'dayjs/plugin/duration'
import relativeTime from 'dayjs/plugin/relativeTime'

dayjs.extend(duration)
dayjs.extend(relativeTime)

export const DATE_FORMAT = 'MMM D, YYYY'

interface Props {
  value?: DateTimeValue
  onChange: (value: string) => void
}

const options = [
  ['now', 'now'],
  ['1 minute ago', 'now-1m'],
  ['5 minutes ago', 'now-5m'],
  ['1 hour ago', 'now-1h'],
  ['2 hours ago', 'now-2h'],
  ['12 hours ago', 'now-12h'],
  ['1 day ago', 'now-1d'],
  ['5 days ago', 'now-5d'],
  ['1 week ago', 'now-1w'],
  ['2 weeks ago', 'now-2w'],
  ['1 month ago', 'now-1M'],
  ['1 year ago', 'now-1y'],
  ['2 years ago', 'now-2y']
]

const dateToString = (value?: DateTimeValue) => {
  return dayjs.isDayjs(value)
    ? value.format(DATE_FORMAT)
    : relativeTimeToString(value)
}

export function DateInput({ value, onChange }: Props) {
  const [focused, setFocused] = useState(false)
  const [active, setActive] = useState(false)
  const [inputText, setInputText] = useState(dateToString(value))

  const syncInputText = () => {
    if (value) {
      setInputText(dateToString(value))
    }
  }

  useEffect(() => {
    if (!focused) {
      syncInputText()
    }
  }, [value, focused])

  const onMouseDown = () => {
    setActive(true)
  }

  const onMouseUp = () => {
    setActive(false)
  }

  const onFocus = () => {
    setFocused(true)
  }

  const onBlur = () => {
    if (active) {
      return
    }
    setFocused(false)
    setActive(false)
    syncInputText()
  }

  const onSelect = (val: string) => {
    setInputText(val)
    onChange(val)
    setActive(false)
  }

  const _onChange = (text: string) => {
    setInputText(text)
    onChange(text)
  }

  const filteredOptions = options.filter(
    ([label, val]) => label.includes(inputText) || val.includes(inputText)
  )

  return (
    <div className="relative" onMouseDown={onMouseDown} onMouseUp={onMouseUp}>
      <input
        className="focus:border-primary-600 focus:ring-3 focus:ring-primary-600/30 border-border-color hover:border-primary-600 mr-2 w-full rounded-md px-2 text-xs sm:w-[100px]"
        type="text"
        value={focused || !value ? inputText : dateToString(value)}
        onChange={(e) => _onChange(e.target.value)}
        onFocus={onFocus}
        onBlur={onBlur}
      />
      {focused && filteredOptions.length > 1 && (
        <div className="bg-default-bg shadow-xs border-main absolute z-10 max-h-60 w-full translate-y-2 overflow-y-auto rounded-md border sm:w-fit">
          {filteredOptions.map(([label, val]) => (
            <div
              className="hover:bg-primary-600 flex justify-between whitespace-nowrap px-3 py-2 hover:text-white"
              key={val}
              onClick={() => onSelect(val)}
            >
              {label}
              <span className="text-text-foreground border-light bg-default-bg ml-8 rounded-sm border px-1">
                {val}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
