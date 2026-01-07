import '../styles.css'
import type { Story } from '@ladle/react'
import { Group, List, Panels, Panel } from './StyledTabs'
import React from 'react'

export const BasicTabs: Story = () => {
  return (
    <div className="p-8">
      <Group>
        <List tabs={['Tab 1', 'Tab 2', 'Tab 3']} />
        <Panels>
          <Panel>
            <div className="p-4">
              <h3 className="mb-2 text-lg font-semibold">Panel 1 Content</h3>
              <p className="text-gray-600">
                This is the content for the first tab.
              </p>
            </div>
          </Panel>
          <Panel>
            <div className="p-4">
              <h3 className="mb-2 text-lg font-semibold">Panel 2 Content</h3>
              <p className="text-gray-600">
                This is the content for the second tab.
              </p>
            </div>
          </Panel>
          <Panel>
            <div className="p-4">
              <h3 className="mb-2 text-lg font-semibold">Panel 3 Content</h3>
              <p className="text-gray-600">
                This is the content for the third tab.
              </p>
            </div>
          </Panel>
        </Panels>
      </Group>
    </div>
  )
}

BasicTabs.meta = {
  description: 'Basic styled tabs with simple text labels'
}

export const TabsWithIcons: Story = () => {
  return (
    <div className="p-8">
      <Group>
        <List
          tabs={[
            <span className="flex items-center gap-2">
              <svg
                className="h-4 w-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"
                />
              </svg>
              Home
            </span>,
            <span className="flex items-center gap-2">
              <svg
                className="h-4 w-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
                />
              </svg>
              Profile
            </span>,
            <span className="flex items-center gap-2">
              <svg
                className="h-4 w-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
                />
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                />
              </svg>
              Settings
            </span>
          ]}
        />
        <Panels>
          <Panel>
            <div className="p-4">
              <h3 className="mb-2 text-lg font-semibold">Home</h3>
              <p className="text-gray-600">Welcome to the home page.</p>
            </div>
          </Panel>
          <Panel>
            <div className="p-4">
              <h3 className="mb-2 text-lg font-semibold">Profile</h3>
              <p className="text-gray-600">
                Manage your profile settings here.
              </p>
            </div>
          </Panel>
          <Panel>
            <div className="p-4">
              <h3 className="mb-2 text-lg font-semibold">Settings</h3>
              <p className="text-gray-600">
                Configure your application settings.
              </p>
            </div>
          </Panel>
        </Panels>
      </Group>
    </div>
  )
}

TabsWithIcons.meta = {
  description: 'Tabs with icon and text labels'
}

export const DisabledTabs: Story = () => {
  return (
    <div className="p-8">
      <Group>
        <List
          tabs={['Available', 'Disabled', 'Available', 'Disabled']}
          disabledTabs={[1, 3]}
        />
        <Panels>
          <Panel>
            <div className="p-4">
              <h3 className="mb-2 text-lg font-semibold">Tab 1</h3>
              <p className="text-gray-600">This tab is available.</p>
            </div>
          </Panel>
          <Panel>
            <div className="p-4">
              <p className="text-gray-600">This tab is disabled.</p>
            </div>
          </Panel>
          <Panel>
            <div className="p-4">
              <h3 className="mb-2 text-lg font-semibold">Tab 3</h3>
              <p className="text-gray-600">This tab is available.</p>
            </div>
          </Panel>
          <Panel>
            <div className="p-4">
              <p className="text-gray-600">This tab is disabled.</p>
            </div>
          </Panel>
        </Panels>
      </Group>
    </div>
  )
}

DisabledTabs.meta = {
  description: 'Tabs with some disabled options using disabledTabs prop'
}

export const NoBorderTabs: Story = () => {
  return (
    <div className="bg-gray-50 p-8">
      <Group>
        <List tabs={['First', 'Second', 'Third']} noBorder />
        <Panels>
          <Panel>
            <div className="rounded bg-white p-4 shadow">
              <h3 className="mb-2 text-lg font-semibold">First Panel</h3>
              <p className="text-gray-600">No border on the tab list.</p>
            </div>
          </Panel>
          <Panel>
            <div className="rounded bg-white p-4 shadow">
              <h3 className="mb-2 text-lg font-semibold">Second Panel</h3>
              <p className="text-gray-600">Content for second panel.</p>
            </div>
          </Panel>
          <Panel>
            <div className="rounded bg-white p-4 shadow">
              <h3 className="mb-2 text-lg font-semibold">Third Panel</h3>
              <p className="text-gray-600">Content for third panel.</p>
            </div>
          </Panel>
        </Panels>
      </Group>
    </div>
  )
}

NoBorderTabs.meta = {
  description: 'Tabs without bottom border using noBorder prop'
}

export const ControlledTabs: Story = () => {
  const [selectedIndex, setSelectedIndex] = React.useState(0)

  return (
    <div className="p-8">
      <div className="mb-4 space-x-2">
        <button
          onClick={() => setSelectedIndex(0)}
          className="rounded bg-blue-500 px-3 py-1 text-sm text-white hover:bg-blue-600"
        >
          Go to Tab 1
        </button>
        <button
          onClick={() => setSelectedIndex(1)}
          className="rounded bg-blue-500 px-3 py-1 text-sm text-white hover:bg-blue-600"
        >
          Go to Tab 2
        </button>
        <button
          onClick={() => setSelectedIndex(2)}
          className="rounded bg-blue-500 px-3 py-1 text-sm text-white hover:bg-blue-600"
        >
          Go to Tab 3
        </button>
      </div>

      <Group selectedIndex={selectedIndex} onChange={setSelectedIndex}>
        <List tabs={['Tab 1', 'Tab 2', 'Tab 3']} />
        <Panels>
          <Panel>
            <div className="p-4">
              <h3 className="mb-2 text-lg font-semibold">Panel 1</h3>
              <p className="text-gray-600">
                Controlled tab - Current index: {selectedIndex}
              </p>
            </div>
          </Panel>
          <Panel>
            <div className="p-4">
              <h3 className="mb-2 text-lg font-semibold">Panel 2</h3>
              <p className="text-gray-600">
                Controlled tab - Current index: {selectedIndex}
              </p>
            </div>
          </Panel>
          <Panel>
            <div className="p-4">
              <h3 className="mb-2 text-lg font-semibold">Panel 3</h3>
              <p className="text-gray-600">
                Controlled tab - Current index: {selectedIndex}
              </p>
            </div>
          </Panel>
        </Panels>
      </Group>
    </div>
  )
}

ControlledTabs.meta = {
  description: 'Controlled tabs with external state management'
}
