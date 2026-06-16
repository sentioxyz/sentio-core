import { ReactNode } from 'react'
import { PopoverTooltip } from './DivTooltip'
import { LuCircleHelp } from 'react-icons/lu'

interface Props {
  text: ReactNode
  className?: string
}

export function HelpIcon({ text, className }: Props) {
  const icon = (
    <LuCircleHelp className="text-text-foreground-secondary ml-1 h-4 w-4" />
  )
  return <PopoverTooltip icon={icon} text={text} className={className} />
}
