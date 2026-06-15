import { useEffect, useRef, useState } from 'react'
import { LuClock } from 'react-icons/lu'
import { classNames } from '../../utils/classnames'

const inputCls = 'border-0 w-5 py-0 px-0.5 text-xs focus:ring-0'

export interface TimeInputProps {
  value: string
  disabled: boolean
  onChange: (value: string) => void
}

export function TimeInput({ value, disabled, onChange }: TimeInputProps) {
  const [hour, setHour] = useState(value.split(':')[0] || '00')
  const [minute, setMinute] = useState(value.split(':')[1] || '00')
  const refHour = useRef<HTMLInputElement>(null)
  const refMinute = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const [h, m] = value.split(':')
    if (+h !== +hour || !hour) {
      setHour(h)
    }
    if (+m !== +minute || !minute) {
      setMinute(m)
    }
  }, [value])

  useEffect(() => {
    const next = `${hour}:${minute}`
    // Only report genuine user edits. When `hour`/`minute` were just synced
    // from the `value` prop (above), echoing them back through onChange creates
    // a feedback loop with the parent — which, for non-idempotent transforms
    // like timezone shifts with fractional offsets, oscillates indefinitely.
    if (next !== value) {
      onChange(next)
    }
  }, [hour, minute])

  const onChangeHour = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { value } = e.currentTarget
    if (value.length < 3 && +value < 24) {
      setHour(value)
    }
    if (+value > 2) {
      setTimeout(() => {
        refMinute.current?.focus()
      })
    }
  }

  const onBlurHour = () => {
    if (hour.length < 2) {
      setHour('0' + hour)
    }
  }

  const onKeyDownHour = (e: React.KeyboardEvent<HTMLInputElement>) => {
    switch (e.key) {
      case 'ArrowRight':
        if (e.currentTarget.selectionEnd === e.currentTarget.value.length) {
          refMinute.current?.focus()
        }
        return
      case 'ArrowUp':
        if (+hour < 23) {
          setHour((+hour + 1).toString().padStart(2, '0'))
        }
        e.preventDefault()
        return
      case 'ArrowDown':
        if (+hour > 0) {
          setHour((+hour - 1).toString().padStart(2, '0'))
        }
        e.preventDefault()
        return
    }
  }

  const onChangeMinute = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { value } = e.currentTarget
    if (value.length < 3 && +value < 60) {
      setMinute(value)
    }
  }

  const onBlurMinute = () => {
    if (minute?.length < 2) {
      setMinute('0' + minute)
    }
  }

  const onKeyDownMinute = (e: React.KeyboardEvent<HTMLInputElement>) => {
    switch (e.key) {
      case 'ArrowLeft':
        if (e.currentTarget.selectionStart === 0) {
          refHour.current?.focus()
        }
        return
      case 'ArrowUp':
        if (+minute < 59) {
          setMinute((+minute + 1).toString().padStart(2, '0'))
        }
        e.preventDefault()
        return
      case 'ArrowDown':
        if (+minute > 0) {
          setMinute((+minute - 1).toString().padStart(2, '0'))
        }
        e.preventDefault()
        return
    }
  }

  const selectInput = (e: React.FocusEvent<HTMLInputElement>) => {
    // Capture the element now; the synthetic event's currentTarget is cleared
    // before the deferred callback runs.
    const el = e.target as HTMLInputElement
    setTimeout(() => {
      el.select()
    })
  }

  return (
    <div
      className={classNames(
        'hover:border-primary-600 focus-within:border-primary-600 inline-flex items-center gap-0.5 rounded-md border px-2 py-1.5 leading-4',
        disabled && 'opacity-30'
      )}
    >
      <input
        className={inputCls}
        type="text"
        value={disabled ? '--' : hour || '00'}
        ref={refHour}
        onChange={onChangeHour}
        onFocus={selectInput}
        onBlur={onBlurHour}
        onKeyDown={onKeyDownHour}
      />
      :
      <input
        className={inputCls}
        type="text"
        value={disabled ? '--' : minute || '00'}
        ref={refMinute}
        onChange={onChangeMinute}
        onFocus={selectInput}
        onBlur={onBlurMinute}
        onKeyDown={onKeyDownMinute}
      />
      <LuClock className="text-text-foreground-secondary ml-2 h-3.5 w-3.5" />
    </div>
  )
}
