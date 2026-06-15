import '../../styles.css'
import type { Story } from '@ladle/react'
import { PopoverButton } from './PopoverButton'

export const Basic: Story = () => (
  <div className="p-16">
    <PopoverButton
      className="border-main hover:border-primary-600 rounded-md border px-3 py-1.5 text-sm"
      content={
        <div className="w-48 p-3 text-sm">
          Popover content anchored to the trigger.
        </div>
      }
    >
      Open popover
    </PopoverButton>
  </div>
)
Basic.meta = { description: 'Popover anchored to a trigger button' }

export const WithArrowAndRenderProp: Story = () => (
  <div className="p-16">
    <PopoverButton
      arrow
      placement="bottom-start"
      className="border-main hover:border-primary-600 rounded-md border px-3 py-1.5 text-sm"
      content={({ close }) => (
        <div className="w-48 p-3 text-sm">
          <p>Render-prop content with access to close().</p>
          <button className="text-primary-600 mt-2" onClick={close}>
            Close
          </button>
        </div>
      )}
    >
      Open with arrow
    </PopoverButton>
  </div>
)
WithArrowAndRenderProp.meta = {
  description: 'Arrow indicator + render-prop content using close()'
}
