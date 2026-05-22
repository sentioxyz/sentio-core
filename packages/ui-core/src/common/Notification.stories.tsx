import '../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { Notification } from './Notification'

export const AllNotifications: Story = () => {
  const [showSuccess, setShowSuccess] = useState(false)
  const [showError, setShowError] = useState(false)
  const [showWarning, setShowWarning] = useState(false)
  const [showInfo, setShowInfo] = useState(false)
  const [showWithButtons, setShowWithButtons] = useState(false)
  const [showAutoDismiss, setShowAutoDismiss] = useState(false)
  const [showLongMessage, setShowLongMessage] = useState(false)

  const triggerAutoDismiss = () => {
    setShowAutoDismiss(true)
    setTimeout(() => {
      setShowAutoDismiss(false)
    }, 3000)
  }

  return (
    <div className="p-8">
      <div className="grid grid-cols-4 gap-4">
        <button
          onClick={() => setShowSuccess(true)}
          className="rounded-sm bg-green-500 px-4 py-2 text-white hover:bg-green-600"
        >
          Show Success Notification
        </button>
        <button
          onClick={() => setShowError(true)}
          className="rounded-sm bg-red-500 px-4 py-2 text-white hover:bg-red-600"
        >
          Show Error Notification
        </button>
        <button
          onClick={() => setShowWarning(true)}
          className="rounded-sm bg-yellow-500 px-4 py-2 text-white hover:bg-yellow-600"
        >
          Show Warning Notification
        </button>
        <button
          onClick={() => setShowInfo(true)}
          className="rounded-sm bg-blue-500 px-4 py-2 text-white hover:bg-blue-600"
        >
          Show Info Notification
        </button>
        <button
          onClick={() => setShowWithButtons(true)}
          className="rounded-sm bg-purple-500 px-4 py-2 text-white hover:bg-purple-600"
        >
          Show Notification with Actions
        </button>
        <button
          onClick={triggerAutoDismiss}
          className="rounded-sm bg-indigo-500 px-4 py-2 text-white hover:bg-indigo-600"
        >
          Show Auto-Dismiss Notification (3s)
        </button>
        <button
          onClick={() => setShowLongMessage(true)}
          className="rounded-sm bg-blue-500 px-4 py-2 text-white hover:bg-blue-600"
        >
          Show Long Message
        </button>
      </div>
      
      <Notification
        show={showSuccess}
        setShow={setShowSuccess}
        title="Successfully saved!"
        message="Your changes have been saved successfully."
        type="success"
      />
      <Notification
        show={showError}
        setShow={setShowError}
        title="Error occurred"
        message="Something went wrong. Please try again."
        type="error"
      />
      <Notification
        show={showWarning}
        setShow={setShowWarning}
        title="Warning"
        message="Your session will expire in 5 minutes."
        type="warning"
      />
      <Notification
        show={showInfo}
        setShow={setShowInfo}
        title="Information"
        message="A new version is available. Update recommended."
        type="info"
      />
      <Notification
        show={showWithButtons}
        setShow={setShowWithButtons}
        title="New message"
        message="You have a new message from John Doe."
        type="info"
        buttons={() => (
          <div className="space-x-2">
            <button
              onClick={() => {
                alert('Viewing message...')
                setShowWithButtons(false)
              }}
              className="rounded-sm bg-blue-500 px-3 py-1 text-sm text-white hover:bg-blue-600"
            >
              View
            </button>
            <button
              onClick={() => setShowWithButtons(false)}
              className="rounded-sm bg-gray-300 px-3 py-1 text-sm text-text-foreground-secondary hover:bg-gray-400"
            >
              Dismiss
            </button>
          </div>
        )}
      />
      <Notification
        show={showAutoDismiss}
        setShow={setShowAutoDismiss}
        title="Auto-dismiss"
        message="This notification will close automatically."
        type="success"
      />
      <Notification
        show={showLongMessage}
        setShow={setShowLongMessage}
        title="Update available"
        message="A new version of the application is available. This update includes important security fixes, performance improvements, and new features. We recommend updating as soon as possible to ensure the best experience."
        type="info"
      />
    </div>
  )
}

AllNotifications.meta = {
  description: 'All notification types in a single story'
}
