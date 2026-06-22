import type { ReactNode } from 'react'
import { classNames } from '@sentio/ui-core'

// Inline addon label sitting flush against a select/input across the option
// panels. The base carries the invariant chrome; `className` carries the
// per-use border-side / rounded / padding variant.
export const AddonLabel = ({
  className,
  children
}: {
  className?: string
  children: ReactNode
}) => (
  <span
    className={classNames(
      'sm:text-icontent border-main inline-flex items-center whitespace-nowrap bg-gray-50',
      className
    )}
  >
    {children}
  </span>
)
