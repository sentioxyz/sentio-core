import { cx as classNames } from 'class-variance-authority'
import { ReactNode } from 'react'

export enum StatusRole {
  Success = 'success',
  Warning = 'warning',
  Error = 'error',
  Disabled = 'disabled',
  Info = 'info'
}

const VersionStateColors: { [key: string]: string } = {
  [StatusRole.Success]: 'bg-cyan-600/10 text-cyan-600',
  [StatusRole.Warning]: 'bg-orange-600/10 text-orange-600',
  [StatusRole.Error]: 'bg-red-600/10 text-red-600',
  [StatusRole.Disabled]: 'bg-gray-600/10 text-gray-600',
  [StatusRole.Info]: 'bg-gray-300/10 text-gray-300'
}

interface Props {
  status: string | ReactNode
  className?: string
  colorClasses?: string
  roundClasses?: string
  bubble?: boolean // if contains a prefix bubble
  role?: string | StatusRole
}

export function StatusBadge({
  status,
  className,
  colorClasses: _colorClasses,
  roundClasses,
  bubble,
  role
}: Props) {
  const colorClasses =
    _colorClasses || VersionStateColors[role || StatusRole.Info]

  return (
    <span
      className={classNames(
        'text-ilabel inline-flex cursor-default items-center px-2 py-0.5 font-medium',
        colorClasses,
        roundClasses ? roundClasses : 'rounded-full',
        className
      )}
      data-test-status={status}
    >
      {bubble && <div className="mr-1.5 h-1.5 w-1.5 rounded-full bg-current" />}
      {status}
    </span>
  )
}

export default StatusBadge
