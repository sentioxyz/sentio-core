'use client'

import { type CSSProperties, type FC, type PropsWithChildren, useState } from 'react'
import copy from 'copy-to-clipboard'
import { cn } from '@/lib/utils'
import './CopyButton.css'

interface Props {
  text?: string | Function
  size?: number
  ml?: number
  mr?: number
  hover?: boolean
  placement?: 'left' | 'right'
  className?: string
}

export const CopyButton: FC<PropsWithChildren<Props>> = ({
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
  const copyToClipboard = (val: string) => {
    copy(val)
    setCopied(true)
    setTimeout(() => {
      setCopied(false)
    }, 2000)
  }

  const onCopy = (e: React.MouseEvent<HTMLElement, MouseEvent>) => {
    const target = e.target as HTMLElement
    if (target.nodeName.toLowerCase() === 'a' && target.getAttribute('href')) {
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
  }

  const handleEventProxy = (e: React.MouseEvent<HTMLElement, MouseEvent>) => {
    onCopy(e)
  }

  const isPureComponent = !children

  const iconContainerStyle: CSSProperties = {
    minWidth: `${size}px`,
    maxWidth: `${size}px`,
    minHeight: `${size}px`,
    maxHeight: `${size}px`,
    marginLeft: `${ml}px`,
    marginRight: `${mr}px`
  }

  const containerStyle: CSSProperties = {
    display: !isPureComponent ? 'inline-block' : 'contents'
  }

  return (
    <div
      className={cn(
        'copyButton space-x-1',
        !isPureComponent && hover ? 'hoverShowIcon' : '',
        !isPureComponent ? 'pointer' : '',
        className
      )}
      style={containerStyle}
      onClick={handleEventProxy}
    >
      {placement === 'right' && children}
      <div
        className={cn('iconContainer', isPureComponent ? className : '', copied ? 'copied' : '')}
        style={iconContainerStyle}
      >
        <i className="fa-regular fa-copy iconCopy text-sm"></i>
        <i className="fa-regular fa-circle-check iconSuccess text-sm text-green-600"></i>
      </div>
      {placement === 'left' && children}
    </div>
  )
}
