import '../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import SlideOver from './SlideOver'

export const BasicSlideOver: Story = () => {
  const [open, setOpen] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setOpen(true)}
        className="rounded bg-blue-500 px-4 py-2 text-white hover:bg-blue-600"
      >
        Open SlideOver
      </button>

      <SlideOver
        title="Basic SlideOver"
        open={open}
        onClose={() => setOpen(false)}
      >
        <div className="p-6">
          <h2 className="mb-4 text-lg font-semibold">Content Area</h2>
          <p className="mb-4 text-gray-600">
            This is a slide-over panel that appears from the right side of the
            screen.
          </p>
          <p className="mb-4 text-gray-600">
            You can close it by clicking the X button, clicking outside (if
            triggerClose is 'all'), or pressing the Escape key.
          </p>
          <div className="space-y-2">
            <div className="rounded border border-gray-200 p-3">Item 1</div>
            <div className="rounded border border-gray-200 p-3">Item 2</div>
            <div className="rounded border border-gray-200 p-3">Item 3</div>
          </div>
        </div>
      </SlideOver>
    </div>
  )
}

BasicSlideOver.meta = {
  description: 'A basic slide-over panel that slides in from the right'
}

export const DifferentSizes: Story = () => {
  const [size, setSize] = useState<
    '2xl' | '3xl' | '4xl' | '5xl' | '6xl' | '7xl' | 'full'
  >('2xl')
  const [open, setOpen] = useState(false)

  return (
    <div className="p-8">
      <div className="mb-4 space-x-2">
        {(['2xl', '3xl', '4xl', '5xl', '6xl', '7xl', 'full'] as const).map(
          (s) => (
            <button
              key={s}
              onClick={() => {
                setSize(s)
                setOpen(true)
              }}
              className="rounded bg-blue-500 px-3 py-2 text-sm text-white hover:bg-blue-600"
            >
              Open {s}
            </button>
          )
        )}
      </div>

      <SlideOver
        title={`SlideOver (${size})`}
        open={open}
        onClose={() => setOpen(false)}
        size={size}
      >
        <div className="p-6">
          <h2 className="mb-4 text-lg font-semibold">Size: {size}</h2>
          <p className="text-gray-600">
            This slide-over has a max-width of {size}. Try different sizes to
            see the difference.
          </p>
        </div>
      </SlideOver>
    </div>
  )
}

DifferentSizes.meta = {
  description: 'SlideOver panels with different size options'
}

export const WithHeaderAddon: Story = () => {
  const [open, setOpen] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setOpen(true)}
        className="rounded bg-blue-500 px-4 py-2 text-white hover:bg-blue-600"
      >
        Open with Header Addon
      </button>

      <SlideOver
        title="SlideOver with Actions"
        open={open}
        onClose={() => setOpen(false)}
        headAddon={
          <button
            className="rounded bg-green-500 px-3 py-1 text-sm text-white hover:bg-green-600"
            onClick={() => alert('Action clicked!')}
          >
            Save
          </button>
        }
      >
        <div className="p-6">
          <h2 className="mb-4 text-lg font-semibold">Form Content</h2>
          <div className="space-y-4">
            <div>
              <label className="mb-1 block text-sm font-medium">Name</label>
              <input
                type="text"
                className="w-full rounded border border-gray-300 px-3 py-2"
                placeholder="Enter name"
              />
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium">Email</label>
              <input
                type="email"
                className="w-full rounded border border-gray-300 px-3 py-2"
                placeholder="Enter email"
              />
            </div>
          </div>
        </div>
      </SlideOver>
    </div>
  )
}

WithHeaderAddon.meta = {
  description: 'SlideOver with additional header actions'
}

export const ButtonCloseOnly: Story = () => {
  const [open, setOpen] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setOpen(true)}
        className="rounded bg-blue-500 px-4 py-2 text-white hover:bg-blue-600"
      >
        Open (Button Close Only)
      </button>

      <p className="mt-2 text-sm text-gray-600">
        This SlideOver only closes via the X button or Escape key. Clicking
        outside won't close it.
      </p>

      <SlideOver
        title="Button Close Only"
        open={open}
        onClose={() => setOpen(false)}
        triggerClose="button"
      >
        <div className="p-6">
          <h2 className="mb-4 text-lg font-semibold">Modal-like Behavior</h2>
          <p className="text-gray-600">
            This slide-over won't close when you click outside of it. You must
            use the X button or press Escape.
          </p>
        </div>
      </SlideOver>
    </div>
  )
}

ButtonCloseOnly.meta = {
  description: 'SlideOver that only closes via button or Escape key'
}

export const NoAnimation: Story = () => {
  const [open, setOpen] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setOpen(true)}
        className="rounded bg-blue-500 px-4 py-2 text-white hover:bg-blue-600"
      >
        Open (No Animation)
      </button>

      <SlideOver
        title="No Animation"
        open={open}
        onClose={() => setOpen(false)}
        noAnimation={true}
      >
        <div className="p-6">
          <h2 className="mb-4 text-lg font-semibold">Instant Display</h2>
          <p className="text-gray-600">
            This slide-over appears instantly without any animation transition.
          </p>
        </div>
      </SlideOver>
    </div>
  )
}

NoAnimation.meta = {
  description: 'SlideOver without animation transition'
}
