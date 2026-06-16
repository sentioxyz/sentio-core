import { Fragment, useCallback, useEffect, useRef, useState } from 'react'
import { Menu, Transition } from '@headlessui/react'
import { classNames } from '../../utils/classnames'
import {
  LuCircleCheck,
  LuRefreshCw as RefreshIcon,
  LuChevronDown
} from 'react-icons/lu'

interface Props {
  onClick: () => void
  /** Current auto-refresh interval in ms (0 = off). Controlled by the consumer. */
  autoRefresh: number
  /** Called when the user picks an interval; the consumer persists it. */
  onAutoRefreshChange: (value: number) => void
}

const items = [
  {
    name: 'Stop auto refresh',
    value: 0
  },
  {
    name: 'Refresh every 10s',
    value: 10000
  },
  {
    name: 'Refresh every minute',
    value: 60000
  }
]

export function AutoRefreshButton({
  onClick,
  autoRefresh,
  onAutoRefreshChange
}: Props) {
  const [timeLeft, setTimeLeft] = useState(0)
  const textRef = useRef<HTMLSpanElement>(null)

  useEffect(() => {
    if (autoRefresh > 0) {
      const interval = setInterval(() => {
        const left = timeLeft - 1
        if (left <= 0) {
          setTimeLeft(autoRefresh / 1000)
          onClick()
        } else {
          setTimeLeft(left)
        }
      }, 1000)
      return () => clearInterval(interval)
    }
  }, [autoRefresh, timeLeft])

  const setRefresh = (value: number) => {
    onAutoRefreshChange(value)
    setTimeLeft(value / 1000)
  }

  useEffect(() => {
    if (autoRefresh && textRef.current) {
      textRef.current.innerText = `Refresh in ${timeLeft}s`
    }
  }, [autoRefresh, timeLeft])

  const foreceRefresh = useCallback(() => {
    onClick()
    setTimeLeft(autoRefresh / 1000)
  }, [onClick, autoRefresh])

  return (
    <div className="inline-flex rounded-md">
      <button
        onClick={foreceRefresh}
        className={classNames(
          'group h-[30px]',
          'hover:border-primary-600 active:bg-primary-700 hover:bg-primary-600 border-main inline-flex items-center border hover:text-white',
          'px-2.5',
          'text-ilabel',
          'font-ilabel',
          'gap-2',
          'py-1'
        )}
      >
        <RefreshIcon className="h-4.5 w-4.5" aria-hidden="true" />
        <span ref={textRef} className="hidden sm:inline">
          Refresh
        </span>
      </button>
      <Menu as="div" className="relative -ml-px block h-[30px]">
        {({ open }) => (
          <>
            <Menu.Button
              className={classNames(
                'relative inline-flex h-[30px] items-center rounded-r-md',
                'px-2 py-2 focus:z-10',
                'border-main border',
                open
                  ? 'bg-primary-600 border-primary-600 text-white'
                  : 'hover:bg-primary-600 hover:border-primary-600 text-text-foreground-secondary border-gray-300 hover:text-white'
              )}
            >
              <span className="sr-only">Open options</span>
              <LuChevronDown className="h-4.5 w-4.5" aria-hidden="true" />
            </Menu.Button>
            <Transition
              as={Fragment}
              enter="transition ease-out duration-100"
              enterFrom="transform opacity-0 scale-95"
              enterTo="transform opacity-100 scale-100"
              leave="transition ease-in duration-75"
              leaveFrom="transform opacity-100 scale-100"
              leaveTo="transform opacity-0 scale-95"
            >
              <Menu.Items className="focus:outline-hidden border-main bg-default-bg absolute right-0 z-10 -mr-1 mt-2 w-56 origin-top-right rounded-md border">
                <div className="py-1">
                  {items.map((item) => (
                    <Menu.Item key={item.value}>
                      {({ active }) => (
                        <button
                          onClick={() => setRefresh(item.value)}
                          className={classNames(
                            'flex w-full justify-between px-4 py-2 text-sm',
                            autoRefresh === item.value
                              ? 'bg-primary-600 text-white'
                              : active
                                ? 'text-primary-600 bg-primary-50'
                                : 'text-foreground'
                          )}
                        >
                          {item.name}
                          {autoRefresh === item.value ? (
                            <LuCircleCheck className="inline-block h-5 w-5" />
                          ) : null}
                        </button>
                      )}
                    </Menu.Item>
                  ))}
                </div>
              </Menu.Items>
            </Transition>
          </>
        )}
      </Menu>
    </div>
  )
}
