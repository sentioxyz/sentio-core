import type { Story } from '@ladle/react'
import { useState } from 'react'
import '../../styles.css'
import { Switch } from './Switch'

export const BasicSwitch: Story = () => {
  const [checked, setChecked] = useState(false)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Basic Switch</h3>
      <Switch
        checked={checked}
        onChange={setChecked}
        label="Enable notifications"
        srText="Enable notifications"
      />
      <p className="mt-4 text-sm text-gray-600">
        Enabled: {checked ? 'Yes' : 'No'}
      </p>
    </div>
  )
}

BasicSwitch.meta = {
  description: 'Basic switch with label'
}

export const WithoutLabel: Story = () => {
  const [checked, setChecked] = useState(true)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Switch Without Label</h3>
      <Switch
        checked={checked}
        onChange={setChecked}
        srText="Toggle feature"
      />
      <p className="mt-4 text-sm text-gray-600">
        Enabled: {checked ? 'Yes' : 'No'}
      </p>
    </div>
  )
}

WithoutLabel.meta = {
  description: 'Switch without visible label text'
}

export const Sizes: Story = () => {
  const [small, setSmall] = useState(false)
  const [regular, setRegular] = useState(true)
  const [large, setLarge] = useState(false)

  return (
    <div className="space-y-4 p-8">
      <h3 className="mb-4 text-lg font-semibold">Switch Sizes</h3>
      <Switch
        checked={small}
        onChange={setSmall}
        size="sm"
        label="Small"
        srText="Toggle small switch"
      />
      <Switch
        checked={regular}
        onChange={setRegular}
        label="Default"
        srText="Toggle default switch"
      />
      <Switch
        checked={large}
        onChange={setLarge}
        size="lg"
        label="Large"
        srText="Toggle large switch"
      />
    </div>
  )
}

Sizes.meta = {
  description: 'Switch size variants'
}

export const Disabled: Story = () => {
  return (
    <div className="space-y-4 p-8">
      <h3 className="mb-4 text-lg font-semibold">Disabled States</h3>
      <Switch
        checked={false}
        onChange={() => {}}
        label="Disabled off"
        srText="Disabled off"
        disabled
      />
      <Switch
        checked={true}
        onChange={() => {}}
        label="Disabled on"
        srText="Disabled on"
        disabled
      />
    </div>
  )
}

Disabled.meta = {
  description: 'Disabled switch in checked and unchecked states'
}

export const ControlledSwitch: Story = () => {
  const [checked, setChecked] = useState(false)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Controlled Switch</h3>
      <div className="space-y-4">
        <Switch
          checked={checked}
          onChange={setChecked}
          label="Controlled switch"
          srText="Controlled switch"
        />
        <div className="space-x-2">
          <button
            onClick={() => setChecked(true)}
            className="rounded-sm bg-green-500 px-3 py-1 text-sm text-white hover:bg-green-600"
          >
            Turn on
          </button>
          <button
            onClick={() => setChecked(false)}
            className="rounded-sm bg-red-500 px-3 py-1 text-sm text-white hover:bg-red-600"
          >
            Turn off
          </button>
          <button
            onClick={() => setChecked(!checked)}
            className="rounded-sm bg-blue-500 px-3 py-1 text-sm text-white hover:bg-blue-600"
          >
            Toggle
          </button>
        </div>
      </div>
    </div>
  )
}

ControlledSwitch.meta = {
  description: 'Switch with external controls'
}
