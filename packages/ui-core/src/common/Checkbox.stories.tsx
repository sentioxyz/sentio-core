import '../styles.css'
import type { Story } from '@ladle/react'
import { useState } from 'react'
import { Checkbox } from './Checkbox'

export const BasicCheckbox: Story = () => {
  const [checked, setChecked] = useState(false)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Basic Checkbox</h3>
      <Checkbox
        checked={checked}
        onChange={setChecked}
        label="Accept terms and conditions"
      />
      <p className="mt-4 text-sm text-gray-600">
        Checked: {checked ? 'Yes' : 'No'}
      </p>
    </div>
  )
}

BasicCheckbox.meta = {
  description: 'Basic checkbox with label'
}

export const WithoutLabel: Story = () => {
  const [checked, setChecked] = useState(true)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Checkbox Without Label</h3>
      <Checkbox checked={checked} onChange={setChecked} />
      <p className="mt-4 text-sm text-gray-600">
        Checked: {checked ? 'Yes' : 'No'}
      </p>
    </div>
  )
}

WithoutLabel.meta = {
  description: 'Checkbox without label text'
}

export const WithLabelNode: Story = () => {
  const [checked, setChecked] = useState(false)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">
        Checkbox with Custom Label Node
      </h3>
      <Checkbox
        checked={checked}
        onChange={setChecked}
        labelNode={
          <span>
            I agree to the{' '}
            <a href="#" className="text-blue-500 hover:underline">
              Terms of Service
            </a>{' '}
            and{' '}
            <a href="#" className="text-blue-500 hover:underline">
              Privacy Policy
            </a>
          </span>
        }
      />
    </div>
  )
}

WithLabelNode.meta = {
  description: 'Checkbox with custom React node as label'
}

export const Disabled: Story = () => {
  return (
    <div className="space-y-4 p-8">
      <h3 className="mb-4 text-lg font-semibold">Disabled States</h3>
      <Checkbox
        checked={false}
        onChange={() => {}}
        label="Disabled unchecked"
        disabled
      />
      <Checkbox
        checked={true}
        onChange={() => {}}
        label="Disabled checked"
        disabled
      />
    </div>
  )
}

Disabled.meta = {
  description: 'Disabled checkbox in checked and unchecked states'
}

export const MultipleCheckboxes: Story = () => {
  const [options, setOptions] = useState({
    email: true,
    sms: false,
    push: true
  })

  const handleChange = (key: keyof typeof options) => (value: boolean) => {
    setOptions({ ...options, [key]: value })
  }

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Notification Preferences</h3>
      <div className="space-y-3">
        <Checkbox
          checked={options.email}
          onChange={handleChange('email')}
          label="Email notifications"
        />
        <Checkbox
          checked={options.sms}
          onChange={handleChange('sms')}
          label="SMS notifications"
        />
        <Checkbox
          checked={options.push}
          onChange={handleChange('push')}
          label="Push notifications"
        />
      </div>
      <div className="mt-4 rounded bg-gray-100 p-3">
        <p className="text-sm font-medium">Selected:</p>
        <ul className="list-inside list-disc text-sm text-gray-600">
          {options.email && <li>Email</li>}
          {options.sms && <li>SMS</li>}
          {options.push && <li>Push</li>}
        </ul>
      </div>
    </div>
  )
}

MultipleCheckboxes.meta = {
  description: 'Multiple checkboxes for selecting options'
}

export const WithCustomStyling: Story = () => {
  const [checked, setChecked] = useState(false)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Custom Styled Checkbox</h3>
      <Checkbox
        checked={checked}
        onChange={setChecked}
        label="Premium feature"
        className="rounded border border-gray-300 p-3 hover:bg-gray-50"
        labelClassName="text-lg font-bold text-purple-600"
      />
    </div>
  )
}

WithCustomStyling.meta = {
  description: 'Checkbox with custom className and labelClassName'
}

export const WithIdAndName: Story = () => {
  const [checked, setChecked] = useState(false)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Checkbox with ID and Name</h3>
      <Checkbox
        checked={checked}
        onChange={setChecked}
        label="Subscribe to newsletter"
        id="newsletter-checkbox"
        name="newsletter"
      />
      <p className="mt-2 text-xs text-gray-500">
        Inspect the checkbox element to see id="newsletter-checkbox" and
        name="newsletter"
      </p>
    </div>
  )
}

WithIdAndName.meta = {
  description: 'Checkbox with id and name attributes for form integration'
}

export const ControlledCheckbox: Story = () => {
  const [checked, setChecked] = useState(false)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Controlled Checkbox</h3>
      <div className="space-y-4">
        <Checkbox
          checked={checked}
          onChange={setChecked}
          label="Controlled checkbox"
        />
        <div className="space-x-2">
          <button
            onClick={() => setChecked(true)}
            className="rounded bg-green-500 px-3 py-1 text-sm text-white hover:bg-green-600"
          >
            Check
          </button>
          <button
            onClick={() => setChecked(false)}
            className="rounded bg-red-500 px-3 py-1 text-sm text-white hover:bg-red-600"
          >
            Uncheck
          </button>
          <button
            onClick={() => setChecked(!checked)}
            className="rounded bg-blue-500 px-3 py-1 text-sm text-white hover:bg-blue-600"
          >
            Toggle
          </button>
        </div>
      </div>
    </div>
  )
}

ControlledCheckbox.meta = {
  description: 'Checkbox with external controls'
}
