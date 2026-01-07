import { ReactNode, useMemo } from 'react'
import { classNames } from '../utils/classnames'

interface ProgressBarProps {
  progress: number // value between 0 and 1
  segments?: Record<number, string> // segment color stops as a percentage
  gradient?: boolean // whether to use a gradient color or not
  upperTicks?: Record<number, ReactNode>
  lowerTicks?: Record<number, ReactNode>
  roundedFull?: boolean
}

const defaultSegments = {
  0.25: 'from-cyan-600 to-cyan-500',
  0.5: 'from-cyan-500 to-orange-600',
  0.75: 'from-orange-600 to-red-600',
  1: 'from-red-600 to-red-700'
}

export const ProgressBar = ({
  progress,
  segments = defaultSegments,
  gradient,
  upperTicks,
  lowerTicks,
  roundedFull
}: ProgressBarProps) => {
  const elements = useMemo(() => {
    const result: ReactNode[] = []
    const colors = Object.entries(segments).sort(
      (a, b) => parseFloat(a[0]) - parseFloat(b[0])
    )
    if (gradient) {
      let pos = 0
      colors.forEach(([stop, color], idx) => {
        const width = (parseFloat(stop) - pos) * 100
        result.push(
          <div
            key={stop}
            className={classNames(
              `absolute top-0 h-4 bg-gradient-to-r ${color}`,
              idx === 0 && 'rounded-l-full',
              idx === colors.length - 1 && 'rounded-r-full'
            )}
            style={{ left: `${pos * 100}%`, width: `${width}%` }}
          />
        )
        pos = parseFloat(stop)
      })
    } else {
      let pos = 0
      for (const [stop, color] of colors) {
        const width = (parseFloat(stop) - pos) * 100
        result.push(
          <div
            className={`absolute h-4 bg-${color} top-0 left-[${pos}] w-[${width}]`}
          />
        )
        pos = parseFloat(stop) * 100
      }
    }
    return result
  }, [segments, gradient])

  const upperTicksElements = useMemo(() => {
    if (!upperTicks) return null
    return Object.entries(upperTicks).map(([p, label]) => {
      const pos = parseFloat(p)
      return (
        <div
          key={pos}
          className="absolute top-0 border-l border-gray-500 text-xs text-gray-500 hover:z-[1]"
          style={{ left: `${pos * 100}%` }}
        >
          <div
            className={classNames(
              'absolute w-fit -translate-y-full whitespace-nowrap text-gray-500',
              pos < 0.05
                ? '-translate-x-1/4'
                : pos > 0.95
                  ? '-translate-x-3/4'
                  : '-translate-x-1/2'
            )}
          >
            {label}
          </div>
          <div className="absolute h-3 w-2 translate-y-1 border-l border-gray-400 border-opacity-50"></div>
        </div>
      )
    })
  }, [upperTicks])

  const lowerTicksElements = useMemo(() => {
    if (!lowerTicks) return null
    return Object.entries(lowerTicks).map(([p, label]) => {
      const pos = parseFloat(p)
      return (
        <div
          key={pos}
          className="relative bottom-0 text-xs hover:z-[1]"
          style={{ left: `${pos * 100}%` }}
        >
          <div className="absolute top-0 h-3 w-2 border-l border-gray-400 border-opacity-50"></div>
          <div
            className={classNames(
              'absolute translate-y-full text-gray-500',
              pos < 0.05
                ? '-translate-x-1/4'
                : pos > 0.95
                  ? '-translate-x-3/4'
                  : '-translate-x-1/2'
            )}
          >
            {label}
          </div>
        </div>
      )
    })
  }, [lowerTicks])

  return (
    <div className="w-full">
      <div className="relative h-4 w-full">{upperTicksElements}</div>
      <div className="relative h-4 w-full">
        {elements}
        <div
          className={classNames(
            progress === 0 ? 'rounded-l-full' : '',
            `dark:bg-sentio-gray-400 absolute right-0 top-0 h-4 rounded-r-full bg-gray-300`
          )}
          style={{ left: `${progress * 100}%` }}
        ></div>
      </div>
      <div className="relative h-4 w-full">{lowerTicksElements}</div>
    </div>
  )
}
