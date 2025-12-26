import '../styles.css'
import { BaseDialog } from './BaseDialog'
import { useState } from 'react'
import NewButton from '../NewButton'

// Ladle works by rendering exported React components from *.stories.* files.
// Export simple React components (no Storybook APIs) so Ladle can pick them up.

export const Default = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        title="Default Dialog"
        open={open}
        onClose={() => setOpen(false)}
        onCancel={() => setOpen(false)}
        onOk={() => {
          console.log('OK clicked')
          setOpen(false)
        }}
      >
        <div className="px-4 py-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            This is a default dialog with title, content, and action buttons.
          </p>
        </div>
      </BaseDialog>
    </>
  )
}

export const WithoutTitle = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        open={open}
        onClose={() => setOpen(false)}
        onCancel={() => setOpen(false)}
        onOk={() => setOpen(false)}
      >
        <div className="px-4 py-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Dialog without title
          </p>
        </div>
      </BaseDialog>
    </>
  )
}

export const WithoutBorders = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        title="No Borders"
        titleBorder={false}
        footerBorder={false}
        open={open}
        onClose={() => setOpen(false)}
        onCancel={() => setOpen(false)}
        onOk={() => setOpen(false)}
      >
        <div className="px-4 py-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Dialog without title and footer borders
          </p>
        </div>
      </BaseDialog>
    </>
  )
}

export const CustomButtonText = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        title="Custom Buttons"
        open={open}
        onClose={() => setOpen(false)}
        onCancel={() => setOpen(false)}
        cancelText="No"
        onOk={() => setOpen(false)}
        okText="Yes"
      >
        <div className="px-4 py-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Are you sure you want to proceed?
          </p>
        </div>
      </BaseDialog>
    </>
  )
}

export const WithErrorMessage = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        title="Dialog with Error"
        open={open}
        onClose={() => setOpen(false)}
        onCancel={() => setOpen(false)}
        onOk={() => setOpen(false)}
        errorMessages="This is an error message"
      >
        <div className="px-4 py-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Dialog with error message displayed in the footer
          </p>
        </div>
      </BaseDialog>
    </>
  )
}

export const WithExtraButtons = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        title="Extra Buttons"
        open={open}
        onClose={() => setOpen(false)}
        onCancel={() => setOpen(false)}
        onOk={() => setOpen(false)}
        extraButtons={
          <NewButton
            role="tertiary"
            onClick={() => console.log('Extra action')}
          >
            Extra Action
          </NewButton>
        }
      >
        <div className="px-4 py-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Dialog with additional action buttons
          </p>
        </div>
      </BaseDialog>
    </>
  )
}

export const OnlyOkButton = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        title="Information"
        open={open}
        onClose={() => setOpen(false)}
        onOk={() => setOpen(false)}
        okText="Got it"
      >
        <div className="px-4 py-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            This is an information dialog with only an OK button
          </p>
        </div>
      </BaseDialog>
    </>
  )
}

export const CustomButtonProps = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        title="Custom Button Props"
        open={open}
        onClose={() => setOpen(false)}
        onCancel={() => setOpen(false)}
        cancelProps={{ disabled: false }}
        onOk={() => setOpen(false)}
        okProps={{ processing: true }}
        okText="Processing..."
      >
        <div className="px-4 py-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Dialog with custom button properties (processing state)
          </p>
        </div>
      </BaseDialog>
    </>
  )
}

export const CustomFooter = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        title="Custom Footer"
        open={open}
        onClose={() => setOpen(false)}
        footer={
          <div className="border-border-color flex justify-center border-t pt-4">
            <NewButton role="link" onClick={() => setOpen(false)}>
              Close
            </NewButton>
          </div>
        }
      >
        <div className="px-4 py-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Dialog with completely custom footer
          </p>
        </div>
      </BaseDialog>
    </>
  )
}

export const LightMask = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        title="Light Mask Dialog"
        open={open}
        onClose={() => setOpen(false)}
        onCancel={() => setOpen(false)}
        onOk={() => setOpen(false)}
        mask="light"
      >
        <div className="px-4 py-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Dialog with lighter background mask
          </p>
        </div>
      </BaseDialog>
    </>
  )
}

export const CustomPanelStyle = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        title="Custom Panel Style"
        open={open}
        onClose={() => setOpen(false)}
        onCancel={() => setOpen(false)}
        onOk={() => setOpen(false)}
        panelClassName="sm:max-w-lg"
      >
        <div className="px-4 py-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Dialog with custom panel size (smaller max width)
          </p>
        </div>
      </BaseDialog>
    </>
  )
}

export const LargeContent = () => {
  const [open, setOpen] = useState(true)

  return (
    <>
      <NewButton role="primary" onClick={() => setOpen(true)}>
        Open Dialog
      </NewButton>
      <BaseDialog
        title="Dialog with Large Content"
        open={open}
        onClose={() => setOpen(false)}
        onCancel={() => setOpen(false)}
        onOk={() => setOpen(false)}
      >
        <div className="max-h-96 overflow-y-auto px-4 py-4">
          <p className="mb-4 text-sm text-gray-600 dark:text-gray-400">
            This dialog contains a lot of content to demonstrate scrolling
            behavior.
          </p>
          {Array.from({ length: 20 }).map((_, i) => (
            <p
              key={i}
              className="mb-2 text-sm text-gray-600 dark:text-gray-400"
            >
              Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do
              eiusmod tempor incididunt ut labore et dolore magna aliqua.
            </p>
          ))}
        </div>
      </BaseDialog>
    </>
  )
}
