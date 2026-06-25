import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import updateLocale from 'dayjs/plugin/updateLocale'
import { classNames } from '@sentio/ui-core'
import { useCallback, useEffect, useRef, useState } from 'react'
import { IoMdRefresh } from 'react-icons/io'
import dayjsEn from 'dayjs/locale/en'
import type { ComputeStatsLike } from '../types/dashboard'

dayjs.extend(relativeTime)
dayjs.extend(updateLocale)

dayjs.locale('en.short', {
  ...dayjsEn,
  relativeTime: {
    // relative time format strings, keep %s %d as the same
    future: 'in %s',
    past: '%s ago',
    s: '<1s',
    m: '1min',
    mm: '%dmin',
    h: '1h',
    hh: '%dh',
    d: '1d',
    dd: '%dd',
    M: '1m',
    MM: '%dm',
    y: '1y',
    yy: '%dy'
  }
})
dayjs.locale('en')

interface Props {
  stats?: ComputeStatsLike
  onRefresh: () => Promise<void>
}

enum COLORS {
  WARNNING = 'text-[#D98200] border-[#D98200] border',
  NORMAL = 'text-[#4CAF1D] border-[#4CAF1D] border'
}

export const DashboardRefresh = ({ stats, onRefresh }: Props) => {
  const timeRef = useRef<HTMLSpanElement>(null)
  const [fetching, setFetching] = useState(false)
  const [currentColor, setCurrentColor] = useState<COLORS>(COLORS.NORMAL)
  useEffect(() => {
    const updateFn = () => {
      if (!stats || !stats.computedAt) return
      const computedAt = dayjs(stats.computedAt).locale('en.short')
      timeRef.current!.textContent = computedAt.fromNow(true)
      if (computedAt.isBefore(dayjs().subtract(1, 'hour'))) {
        setCurrentColor(COLORS.WARNNING)
      } else {
        setCurrentColor(COLORS.NORMAL)
      }
    }
    updateFn()
    const interval = setInterval(() => {
      updateFn()
    }, 1000)
    return () => {
      clearInterval(interval)
    }
  }, [stats])

  const onClick = useCallback(() => {
    setFetching((prevState) => {
      if (prevState) return prevState
      onRefresh().finally(() => {
        setFetching(false)
      })
      return true
    })
  }, [onRefresh])

  useEffect(() => {
    if (typeof window == 'object') {
      window.addEventListener('refresh_all', onClick)
      return () => {
        window.removeEventListener('refresh_all', onClick)
      }
    }
  }, [])

  const showReload = fetching || stats?.isRefreshing
  return (
    <div
      className={classNames(
        'group/refresh relative ml-1 flex items-center gap-1 rounded-sm py-px pl-1 text-xs transition-all',
        currentColor,
        showReload ? 'pr-5' : 'pr-1.5 hover:pr-5'
      )}
    >
      <span className="cursor-default text-[10px]" ref={timeRef} />
      <button
        onClick={onClick}
        className={classNames(
          'absolute right-1',
          showReload ? 'block' : 'hidden group-hover/refresh:block'
        )}
      >
        <IoMdRefresh
          className={classNames(
            'h-3.5 w-3.5',
            showReload ? 'animate-spin' : ''
          )}
        />
      </button>
    </div>
  )
}
