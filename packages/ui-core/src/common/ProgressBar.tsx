import { ReactNode, useMemo } from 'react'
import { classNames } from '../utils/classnames'

/**
 * Theme-aware reference to slot `i` of the Sentio "classic" categorical
 * palette. The actual hex values live in `theme-variables.css` as
 * `--sentio-classic-0..8` (one set under `body { }`, another under
 * `body.dark { }`), so the returned CSS color swaps with the theme.
 */
export const classic = (i: number) =>
  `rgb(var(--sentio-classic-${((i % 9) + 9) % 9}))`

export type SegmentColor =
  | string // Tailwind gradient utility, e.g. 'from-cyan-600 to-cyan-500'
  | { from: string; to: string } // Any CSS color (hex, rgb, var(...))

interface ProgressBarProps {
  progress: number // value between 0 and 1
  segments?: Record<number, SegmentColor> // segment color stops as a percentage
  gradient?: boolean // whether to use a gradient color or not
  upperTicks?: Record<number, ReactNode>
  lowerTicks?: Record<number, ReactNode>
  roundedFull?: boolean
}

// Default ramp uses the Sentio classic palette (cool → warm), so it reads
// naturally for "low/safe → high/danger" usage indicators in both themes.
//   idx 6 (green)  → idx 7 (cyan)   → idx 5 (yellow) → idx 4 (orange) → idx 3 (pink)
const defaultSegments: Record<number, SegmentColor> = {
  0.25: { from: classic(6), to: classic(7) },
  0.5: { from: classic(7), to: classic(5) },
  0.75: { from: classic(5), to: classic(4) },
  1: { from: classic(4), to: classic(3) }
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
        const isObject = typeof color !== 'string'
        const style: React.CSSProperties = {
          left: `${pos * 100}%`,
          width: `${width}%`
        }
        if (isObject) {
          style.backgroundImage = `linear-gradient(to right, ${color.from}, ${color.to})`
        }
        result.push(
          <div
            key={stop}
            className={classNames(
              'absolute top-0 h-4',
              !isObject && `bg-linear-to-r ${color as string}`,
              idx === 0 && 'rounded-l-full',
              idx === colors.length - 1 && 'rounded-r-full'
            )}
            style={style}
          />
        )
        pos = parseFloat(stop)
      })
    } else {
      let pos = 0
      for (const [stop, color] of colors) {
        const width = (parseFloat(stop) - pos) * 100
        const isObject = typeof color !== 'string'
        const style: React.CSSProperties = {
          left: `${pos * 100}%`,
          width: `${width}%`
        }
        if (isObject) {
          // Solid fill: take the `from` end of the segment.
          style.backgroundColor = color.from
        }
        result.push(
          <div
            key={stop}
            className={classNames(
              'absolute top-0 h-4',
              !isObject && `bg-${color as string}`
            )}
            style={style}
          />
        )
        pos = parseFloat(stop)
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
          className="absolute top-0 border-l border-dark text-xs text-text-foreground-secondary hover:z-1"
          style={{ left: `${pos * 100}%` }}
        >
          <div
            className={classNames(
              'absolute w-fit -translate-y-full whitespace-nowrap text-text-foreground-secondary',
              pos < 0.05
                ? '-translate-x-1/4'
                : pos > 0.95
                  ? '-translate-x-3/4'
                  : '-translate-x-1/2'
            )}
          >
            {label}
          </div>
          <div className="absolute h-3 w-2 translate-y-1 border-l border-dark border-opacity-50"></div>
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
          className="relative bottom-0 text-xs hover:z-1"
          style={{ left: `${pos * 100}%` }}
        >
          <div className="absolute top-0 h-3 w-2 border-l border-dark border-opacity-50"></div>
          <div
            className={classNames(
              'absolute translate-y-full text-text-foreground-secondary',
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
            `absolute right-0 top-0 h-4 rounded-r-full bg-default-bg border border-main`
          )}
          style={{ left: `${progress * 100}%` }}
        ></div>
      </div>
      <div className="relative h-4 w-full">{lowerTicksElements}</div>
    </div>
  )
}
