import '../styles.css'
import type { Story } from '@ladle/react'
import { HelpIcon } from './HelpIcon'
import { ErrorIcon } from './ErrorIcon'

export const Help: Story = () => (
  <div className="flex items-center p-8">
    <span className="text-text-foreground text-sm">Aggregate</span>
    <HelpIcon text="Choose how matching series are combined into one." />
  </div>
)
Help.meta = {
  description: 'Inline help icon with a hover tooltip'
}

export const Error: Story = () => (
  <div className="flex items-center p-8 text-red-600">
    <span className="text-sm">Invalid expression</span>
    <ErrorIcon text="Formula references an unknown variable." />
  </div>
)
Error.meta = {
  description: 'Inline error icon with a hover tooltip'
}
