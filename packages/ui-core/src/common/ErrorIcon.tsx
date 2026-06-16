import { LuCircleAlert } from 'react-icons/lu'
import { PopoverTooltip } from './DivTooltip'

interface Props {
  text: string
  className?: string
}

export function ErrorIcon({ text, className }: Props) {
  const icon = <LuCircleAlert className="ml-1 h-5 w-5" />

  return <PopoverTooltip text={text} icon={icon} className={className} />
}
