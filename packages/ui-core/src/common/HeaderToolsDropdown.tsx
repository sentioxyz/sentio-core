import { LuChevronDown, LuChevronUp } from 'react-icons/lu'
import { cx as classNames } from 'class-variance-authority'
import { ReactNode } from 'react'

interface HeaderToolsToggleButtonProps {
  isOpen: boolean
  onClick: () => void
  className?: string
}

export const HeaderToolsToggleButton = ({
  isOpen,
  onClick,
  className
}: HeaderToolsToggleButtonProps) => {
  return (
    <button
      onClick={onClick}
      className={classNames(
        'flex items-center justify-center rounded-md p-1 transition-colors',
        'dark:hover:bg-sentio-gray-100 hover:bg-gray-200',
        'text-text-foreground',
        className
      )}
      aria-label="Toggle tools"
    >
      {isOpen ? (
        <LuChevronUp className="h-4 w-4 transition-transform" />
      ) : (
        <LuChevronDown className="h-4 w-4 transition-transform" />
      )}
    </button>
  )
}

interface HeaderToolsContentProps {
  isOpen: boolean
  children: ReactNode
  className?: string
}

export const HeaderToolsContent = ({
  isOpen,
  children,
  className
}: HeaderToolsContentProps) => {
  if (!isOpen) {
    return null
  }

  return (
    <div className={classNames('w-full overflow-hidden', className)}>
      {children}
    </div>
  )
}
