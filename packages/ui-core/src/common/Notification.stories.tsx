import '../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { Notification } from './Notification'

export const SuccessNotification: Story = () => {
  const [show, setShow] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setShow(true)}
        className="rounded bg-green-500 px-4 py-2 text-white hover:bg-green-600"
      >
        Show Success Notification
      </button>
      <Notification
        show={show}
        setShow={setShow}
        title="Successfully saved!"
        message="Your changes have been saved successfully."
        type="success"
      />
    </div>
  )
}

SuccessNotification.meta = {
  description: 'Success notification with green icon'
}

export const ErrorNotification: Story = () => {
  const [show, setShow] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setShow(true)}
        className="rounded bg-red-500 px-4 py-2 text-white hover:bg-red-600"
      >
        Show Error Notification
      </button>
      <Notification
        show={show}
        setShow={setShow}
        title="Error occurred"
        message="Something went wrong. Please try again."
        type="error"
      />
    </div>
  )
}

ErrorNotification.meta = {
  description: 'Error notification with red icon'
}

export const WarningNotification: Story = () => {
  const [show, setShow] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setShow(true)}
        className="rounded bg-yellow-500 px-4 py-2 text-white hover:bg-yellow-600"
      >
        Show Warning Notification
      </button>
      <Notification
        show={show}
        setShow={setShow}
        title="Warning"
        message="Your session will expire in 5 minutes."
        type="warning"
      />
    </div>
  )
}

WarningNotification.meta = {
  description: 'Warning notification with yellow icon'
}

export const InfoNotification: Story = () => {
  const [show, setShow] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setShow(true)}
        className="rounded bg-blue-500 px-4 py-2 text-white hover:bg-blue-600"
      >
        Show Info Notification
      </button>
      <Notification
        show={show}
        setShow={setShow}
        title="Information"
        message="A new version is available. Update recommended."
        type="info"
      />
    </div>
  )
}

InfoNotification.meta = {
  description: 'Info notification with blue icon'
}

export const WithButtons: Story = () => {
  const [show, setShow] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setShow(true)}
        className="rounded bg-purple-500 px-4 py-2 text-white hover:bg-purple-600"
      >
        Show Notification with Actions
      </button>
      <Notification
        show={show}
        setShow={setShow}
        title="New message"
        message="You have a new message from John Doe."
        type="info"
        buttons={() => (
          <div className="space-x-2">
            <button
              onClick={() => {
                alert('Viewing message...')
                setShow(false)
              }}
              className="rounded bg-blue-500 px-3 py-1 text-sm text-white hover:bg-blue-600"
            >
              View
            </button>
            <button
              onClick={() => setShow(false)}
              className="rounded bg-gray-300 px-3 py-1 text-sm text-gray-700 hover:bg-gray-400"
            >
              Dismiss
            </button>
          </div>
        )}
      />
    </div>
  )
}

WithButtons.meta = {
  description: 'Notification with custom action buttons'
}

export const AutoDismiss: Story = () => {
  const [show, setShow] = useState(false)

  const showNotification = () => {
    setShow(true)
    setTimeout(() => {
      setShow(false)
    }, 3000)
  }

  return (
    <div className="p-8">
      <button
        onClick={showNotification}
        className="rounded bg-indigo-500 px-4 py-2 text-white hover:bg-indigo-600"
      >
        Show Auto-Dismiss Notification (3s)
      </button>
      <p className="mt-2 text-sm text-gray-600">
        This notification will automatically dismiss after 3 seconds
      </p>
      <Notification
        show={show}
        setShow={setShow}
        title="Auto-dismiss"
        message="This notification will close automatically."
        type="success"
      />
    </div>
  )
}

AutoDismiss.meta = {
  description: 'Notification that auto-dismisses after a timeout'
}

export const MultipleNotifications: Story = () => {
  const [showSuccess, setShowSuccess] = useState(false)
  const [showError, setShowError] = useState(false)

  return (
    <div className="p-8">
      <div className="space-x-2">
        <button
          onClick={() => setShowSuccess(true)}
          className="rounded bg-green-500 px-4 py-2 text-white hover:bg-green-600"
        >
          Success
        </button>
        <button
          onClick={() => setShowError(true)}
          className="rounded bg-red-500 px-4 py-2 text-white hover:bg-red-600"
        >
          Error
        </button>
      </div>
      <p className="mt-2 text-sm text-gray-600">
        Note: Multiple notifications will stack on top of each other
      </p>
      <Notification
        show={showSuccess}
        setShow={setShowSuccess}
        title="Success"
        message="Operation completed successfully."
        type="success"
      />
      <Notification
        show={showError}
        setShow={setShowError}
        title="Error"
        message="Operation failed. Please try again."
        type="error"
      />
    </div>
  )
}

MultipleNotifications.meta = {
  description: 'Multiple notifications displayed simultaneously'
}

export const LongMessage: Story = () => {
  const [show, setShow] = useState(false)

  return (
    <div className="p-8">
      <button
        onClick={() => setShow(true)}
        className="rounded bg-blue-500 px-4 py-2 text-white hover:bg-blue-600"
      >
        Show Long Message
      </button>
      <Notification
        show={show}
        setShow={setShow}
        title="Update available"
        message="A new version of the application is available. This update includes important security fixes, performance improvements, and new features. We recommend updating as soon as possible to ensure the best experience."
        type="info"
      />
    </div>
  )
}

LongMessage.meta = {
  description: 'Notification with a longer message that wraps'
}
