import '../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { ConfirmDialog } from './ConfirmDialog'

export const DangerConfirm: Story = () => {
  const [open, setOpen] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setOpen(true)}
        className="rounded bg-red-500 px-4 py-2 text-white hover:bg-red-600"
      >
        Delete Item
      </button>

      <ConfirmDialog
        title="Delete item?"
        message="Are you sure you want to delete this item? This action cannot be undone."
        open={open}
        onClose={setOpen}
        onConfirm={async () => {
          await new Promise((resolve) => setTimeout(resolve, 1000))
          alert('Item deleted!')
        }}
        type="danger"
        buttonLabel="Delete"
      />
    </div>
  )
}

DangerConfirm.meta = {
  description: 'Danger confirmation dialog with red icon and delete action'
}

export const QuestionConfirm: Story = () => {
  const [open, setOpen] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setOpen(true)}
        className="rounded bg-blue-500 px-4 py-2 text-white hover:bg-blue-600"
      >
        Proceed
      </button>

      <ConfirmDialog
        title="Continue with this action?"
        message="This will make changes to your account settings."
        open={open}
        onClose={setOpen}
        onConfirm={async () => {
          await new Promise((resolve) => setTimeout(resolve, 800))
          alert('Action confirmed!')
        }}
        type="question"
        buttonLabel="Proceed"
      />
    </div>
  )
}

QuestionConfirm.meta = {
  description: 'Question confirmation dialog with blue icon'
}

export const WithChildren: Story = () => {
  const [open, setOpen] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setOpen(true)}
        className="rounded bg-orange-500 px-4 py-2 text-white hover:bg-orange-600"
      >
        Remove User
      </button>

      <ConfirmDialog
        title="Remove user from project?"
        open={open}
        onClose={setOpen}
        onConfirm={async () => {
          await new Promise((resolve) => setTimeout(resolve, 1000))
          alert('User removed!')
        }}
        type="danger"
        buttonLabel="Remove"
      >
        <div className="mt-2">
          <p className="text-sm text-gray-500">
            This will remove{' '}
            <strong className="font-semibold">john@example.com</strong> from the
            project.
          </p>
          <p className="mt-2 text-sm text-gray-500">
            They will lose access to:
          </p>
          <ul className="ml-2 mt-1 list-inside list-disc text-sm text-gray-500">
            <li>Project dashboard</li>
            <li>Analytics data</li>
            <li>Team collaboration</li>
          </ul>
        </div>
      </ConfirmDialog>
    </div>
  )
}

WithChildren.meta = {
  description: 'Confirmation dialog with custom children content'
}

export const DisabledConfirm: Story = () => {
  const [open, setOpen] = useState(false)
  const [agreed, setAgreed] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => {
          setOpen(true)
          setAgreed(false)
        }}
        className="rounded bg-blue-500 px-4 py-2 text-white hover:bg-blue-600"
      >
        Accept Terms
      </button>

      <ConfirmDialog
        title="Accept Terms and Conditions"
        open={open}
        onClose={setOpen}
        onConfirm={async () => {
          await new Promise((resolve) => setTimeout(resolve, 800))
          alert('Terms accepted!')
        }}
        type="question"
        buttonLabel="Accept"
        disabled={!agreed}
      >
        <div className="mt-2">
          <p className="mb-3 text-sm text-gray-500">
            Please read and accept the terms and conditions to proceed.
          </p>
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={agreed}
              onChange={(e) => setAgreed(e.target.checked)}
              className="rounded"
            />
            <span className="text-sm text-gray-700">
              I have read and agree to the terms
            </span>
          </label>
        </div>
      </ConfirmDialog>
    </div>
  )
}

DisabledConfirm.meta = {
  description:
    'Confirmation dialog with disabled confirm button until checkbox is checked'
}

export const CustomButtons: Story = () => {
  const [open, setOpen] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setOpen(true)}
        className="rounded bg-purple-500 px-4 py-2 text-white hover:bg-purple-600"
      >
        Open Custom Dialog
      </button>

      <ConfirmDialog
        title="Choose an option"
        message="You can provide custom buttons instead of default confirm/cancel."
        open={open}
        onClose={setOpen}
        onConfirm={() => {}}
        type="question"
        buttons={
          <div className="flex w-full gap-2">
            <button
              onClick={() => {
                alert('Option A selected')
                setOpen(false)
              }}
              className="flex-1 rounded bg-green-500 px-4 py-2 text-white hover:bg-green-600"
            >
              Option A
            </button>
            <button
              onClick={() => {
                alert('Option B selected')
                setOpen(false)
              }}
              className="flex-1 rounded bg-blue-500 px-4 py-2 text-white hover:bg-blue-600"
            >
              Option B
            </button>
            <button
              onClick={() => setOpen(false)}
              className="flex-1 rounded bg-gray-300 px-4 py-2 text-gray-700 hover:bg-gray-400"
            >
              Cancel
            </button>
          </div>
        }
      />
    </div>
  )
}

CustomButtons.meta = {
  description: 'Confirmation dialog with custom button layout'
}
