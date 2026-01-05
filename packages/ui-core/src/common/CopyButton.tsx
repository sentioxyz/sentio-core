'use client'

import {
  type CSSProperties,
  type FC,
  type ReactNode,
  useState,
  useCallback,
  useRef,
  type SVGProps,
  useEffect
} from 'react'
import copy from 'copy-to-clipboard'
import { cx as classNames } from 'class-variance-authority'

export const CopyIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    fill="none"
    viewBox="0 0 24 24"
    strokeWidth="2"
    stroke="currentColor"
    {...props}
  >
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      d="M16.5 8.25V6a2.25 2.25 0 0 0-2.25-2.25H6A2.25 2.25 0 0 0 3.75 6v8.25A2.25 2.25 0 0 0 6 16.5h2.25m8.25-8.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-7.5A2.25 2.25 0 0 1 8.25 18v-1.5m8.25-8.25h-6a2.25 2.25 0 0 0-2.25 2.25v6"
    />
  </svg>
)

export const CopySuccessIcon = (props: SVGProps<SVGSVGElement>) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    fill="rgb(var(--cyan-600))"
    {...props}
  >
    <path
      fillRule="evenodd"
      d="M2.25 12c0-5.385 4.365-9.75 9.75-9.75s9.75 4.365 9.75 9.75-4.365 9.75-9.75 9.75S2.25 17.385 2.25 12Zm13.36-1.814a.75.75 0 1 0-1.22-.872l-3.236 4.53L9.53 12.22a.75.75 0 0 0-1.06 1.06l2.25 2.25a.75.75 0 0 0 1.14-.094l3.75-5.25Z"
      clipRule="evenodd"
    />
  </svg>
)

interface Props {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-function-type
  text?: string | Function
  size?: number
  ml?: number
  mr?: number
  hover?: boolean
  placement?: 'left' | 'right'
  className?: string
  children?: ReactNode
}

export const CopyButton: FC<Props> = ({
  text = '',
  size = 16,
  ml,
  mr,
  placement = 'right',
  hover,
  children,
  className
}) => {
  const [copied, setCopied] = useState(false)
  const [isHovered, setIsHovered] = useState(false)
  const [isMobile, setIsMobile] = useState(false)
  const timeoutRef = useRef<NodeJS.Timeout | null>(null)
  const iconContainerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 640)
    }
    checkMobile()
    window.addEventListener('resize', checkMobile)
    return () => window.removeEventListener('resize', checkMobile)
  }, [])

  const copyToClipboard = useCallback((val: string) => {
    copy(val)
    setCopied(true)

    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current)
    }

    timeoutRef.current = setTimeout(() => {
      setCopied(false)
      timeoutRef.current = null
    }, 2000)
  }, [])

  const onCopy = useCallback(
    (e: React.MouseEvent<HTMLElement, MouseEvent>) => {
      const target = e.target as HTMLElement
      if (
        target.nodeName.toLowerCase() === 'a' &&
        target.getAttribute('href')
      ) {
        return
      }
      e.stopPropagation()
      e.preventDefault()
      if (copied) return
      if (typeof text === 'function') {
        const val = text() as string | Promise<string>
        if (val instanceof Promise) {
          val
            .then((res: string) => {
              copyToClipboard(res)
            })
            .catch((error) => {
              console.error(error)
            })
        } else {
          copyToClipboard(val)
        }
      } else {
        copyToClipboard(text)
      }
    },
    [copied, text, copyToClipboard]
  )

  const handleEventProxy = useCallback(
    (e: React.MouseEvent<HTMLElement, MouseEvent>) => {
      if (children) {
        onCopy(e)
      }
    },
    [children, onCopy]
  )

  const isPureComponent = !children

  const iconContainerStyle: CSSProperties = {
    minWidth: `${size}px`,
    maxWidth: `${size}px`,
    minHeight: `${size}px`,
    maxHeight: `${size}px`,
    marginLeft: ml !== undefined ? `${ml}px` : undefined,
    marginRight: mr !== undefined ? `${mr}px` : undefined,
    visibility:
      !isPureComponent && hover
        ? isMobile
          ? 'visible'
          : isHovered
            ? 'visible'
            : 'hidden'
        : 'visible'
  }

  const containerStyle: CSSProperties = {
    display: !isPureComponent ? 'inline-block' : 'contents'
  }

  const svgStyle: CSSProperties = {
    margin: 0
  }

  const iconCopyStyle: CSSProperties = {
    transform: copied ? 'translateY(-100%)' : 'translateY(0)'
  }

  const iconSuccessStyle: CSSProperties = {
    transform: copied ? 'translateY(-100%)' : 'translateY(100%)'
  }

  return (
    <div
      className={classNames(
        'inline-block min-w-fit overflow-hidden whitespace-nowrap',
        'space-x-1',
        className
      )}
      style={containerStyle}
      onClick={handleEventProxy}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      {placement === 'right' && children}
      <div
        ref={iconContainerRef}
        className={classNames(
          'icon-container inline-block overflow-hidden align-sub',
          isPureComponent ? className : '',
          copied ? 'copied' : ''
        )}
        style={iconContainerStyle}
      >
        <CopyIcon
          className="icon-copy block cursor-pointer transition-all"
          width={size}
          height={size}
          style={{ ...svgStyle, ...iconCopyStyle }}
          // @ts-expect-error allow unknown prop
          onClick={onCopy}
        />
        <CopySuccessIcon
          className="icon-success block transition-all"
          width={size}
          height={size}
          style={{ ...svgStyle, ...iconSuccessStyle }}
        />
      </div>
      {placement === 'left' && children}
    </div>
  )
}
