import React from 'react'

interface Props {
  className?: string
  children: React.ReactNode
}

export function LineNumber({ className, children }: Props) {
  return (
    <div className={`absolute h-5 w-5 rounded-full ${className}`}>
      <span className="overflow absolute left-0 top-0 w-5 text-center leading-5">
        {children}
      </span>
    </div>
  )
}
