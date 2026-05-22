import type { Story } from '@ladle/react'
import { useState } from 'react'
import { LuEllipsis, LuSettings, LuChevronDown } from 'react-icons/lu'
import '../../styles.css'
import { PopupMenuButton } from './PopupMenuButton'
import { IMenuItem } from './types'

const basicItems: IMenuItem[][] = [
  [
    { key: 'edit', label: 'Edit' },
    { key: 'duplicate', label: 'Duplicate' },
    { key: 'rename', label: 'Rename' }
  ],
  [{ key: 'delete', label: 'Delete' }]
]

const itemsWithIcons: IMenuItem[][] = [
  [
    {
      key: 'settings',
      label: 'Settings',
      icon: <LuSettings className="h-4 w-4" />
    },
    {
      key: 'export',
      label: 'Export',
      icon: <span className="h-4 w-4 text-xs">⬆</span>
    },
    {
      key: 'archive',
      label: 'Archive',
      icon: <span className="h-4 w-4 text-xs">📦</span>,
      disabled: true,
      disabledHint: 'Upgrade plan to archive'
    }
  ]
]

export const Default: Story = () => {
  const [selected, setSelected] = useState<string>('')
  return (
    <div className="p-16 flex justify-center">
      <div className="space-y-4">
        <PopupMenuButton
          buttonIcon={<LuEllipsis className="h-5 w-5" />}
          items={basicItems}
          onSelect={(key) => setSelected(key)}
          ariaLabel="More options"
        />
        {selected && (
          <p className="text-sm text-text-foreground-secondary">
            Selected: {selected}
          </p>
        )}
      </div>
    </div>
  )
}

Default.meta = { description: 'Basic popup menu with grouped items' }

export const WithGroupLabels: Story = () => {
  const [selected, setSelected] = useState<string>('')
  const items: IMenuItem[][] = [
    [
      { key: 'view', label: 'View' },
      { key: 'edit', label: 'Edit' }
    ],
    [
      { key: 'share', label: 'Share' },
      { key: 'export', label: 'Export' }
    ],
    [{ key: 'delete', label: 'Delete' }]
  ]

  return (
    <div className="p-16 flex justify-center">
      <div className="space-y-4">
        <PopupMenuButton
          buttonIcon={<LuEllipsis className="h-5 w-5" />}
          items={items}
          groupLabels={['Actions', 'Share', 'Danger Zone']}
          onSelect={(key) => setSelected(key)}
          ariaLabel="Options"
        />
        {selected && (
          <p className="text-sm text-text-foreground-secondary">
            Selected: {selected}
          </p>
        )}
      </div>
    </div>
  )
}

WithGroupLabels.meta = { description: 'Menu with group section labels' }

export const WithIcons: Story = () => {
  const [selected, setSelected] = useState<string>('')
  return (
    <div className="p-16 flex justify-center">
      <PopupMenuButton
        buttonIcon={<LuSettings className="h-5 w-5" />}
        items={itemsWithIcons}
        onSelect={(key) => setSelected(key)}
        ariaLabel="Settings"
      />
      {selected && (
        <p className="mt-4 text-sm text-text-foreground-secondary">
          Selected: {selected}
        </p>
      )}
    </div>
  )
}

WithIcons.meta = { description: 'Menu items with icons and disabled state' }

export const WithHeaderAndFooter: Story = () => {
  const [selected, setSelected] = useState<string>('')
  return (
    <div className="p-16 flex justify-center">
      <PopupMenuButton
        buttonIcon={<LuEllipsis className="h-5 w-5" />}
        items={basicItems}
        header={
          <div className="px-4 py-2 border-b border-light text-sm font-medium">
            My Project
          </div>
        }
        footer={
          <div className="px-4 py-2 border-light text-xs text-text-foreground-secondary">
            v1.2.0
          </div>
        }
        onSelect={(key) => setSelected(key)}
        ariaLabel="Project options"
      />
      {selected && (
        <p className="mt-4 text-sm text-text-foreground-secondary">
          Selected: {selected}
        </p>
      )}
    </div>
  )
}

WithHeaderAndFooter.meta = { description: 'Menu with custom header and footer' }

export const WithSelectedKey: Story = () => {
  const [selectedKey, setSelectedKey] = useState('duplicate')
  const items: IMenuItem[][] = [
    [
      { key: 'edit', label: 'Edit' },
      { key: 'duplicate', label: 'Duplicate' },
      { key: 'rename', label: 'Rename' }
    ]
  ]
  return (
    <div className="p-16 flex justify-center">
      <div className="space-y-4">
        <PopupMenuButton
          buttonIcon={<LuEllipsis className="h-5 w-5" />}
          items={items}
          selectedKey={selectedKey}
          onSelect={(key) => setSelectedKey(key)}
          ariaLabel="Options"
        />
        <p className="text-sm text-text-foreground-secondary">
          Selected key: {selectedKey}
        </p>
      </div>
    </div>
  )
}

WithSelectedKey.meta = { description: 'Menu with a pre-selected item highlighted' }

export const CustomButtonIcon: Story = () => {
  const [selected, setSelected] = useState<string>('')
  return (
    <div className="p-16 flex gap-8">
      <div className="space-y-2">
        <p className="text-xs text-text-foreground-secondary">Static icon</p>
        <PopupMenuButton
          buttonIcon={
            <button className="flex items-center gap-1 rounded px-3 py-1 text-sm border border-main hover:bg-gray-50">
              Actions <LuChevronDown className="h-4 w-4" />
            </button>
          }
          items={basicItems}
          onSelect={(key) => setSelected(key)}
          portal={false}
        />
      </div>
      <div className="space-y-2">
        <p className="text-xs text-text-foreground-secondary">Dynamic icon (changes when open)</p>
        <PopupMenuButton
          buttonIcon={(isOpen) => (
            <button className="flex items-center gap-1 rounded px-3 py-1 text-sm border border-main hover:bg-gray-50">
              Actions
              <LuChevronDown
                className={`h-4 w-4 transition-transform ${isOpen ? 'rotate-180' : ''}`}
              />
            </button>
          )}
          items={basicItems}
          onSelect={(key) => setSelected(key)}
          portal={false}
        />
      </div>
      {selected && (
        <p className="self-end text-sm text-text-foreground-secondary">
          Selected: {selected}
        </p>
      )}
    </div>
  )
}

CustomButtonIcon.meta = { description: 'Custom button trigger — static and dynamic icon function' }

export const CustomWidth: Story = () => (
  <div className="p-16 flex justify-center">
    <PopupMenuButton
      buttonIcon={<LuEllipsis className="h-5 w-5" />}
      items={basicItems}
      width={300}
      ariaLabel="Wide menu"
    />
  </div>
)

CustomWidth.meta = { description: 'Menu with a fixed custom width' }

export const CustomRenderItem: Story = () => {
  const [selected, setSelected] = useState<string>('')
  const items: IMenuItem[][] = [
    [
      { key: 'danger', label: 'Delete project', data: { danger: true } },
      { key: 'safe', label: 'Duplicate project', data: { danger: false } }
    ]
  ]
  return (
    <div className="p-16 flex justify-center">
      <div className="space-y-4">
        <PopupMenuButton
          buttonIcon={<LuEllipsis className="h-5 w-5" />}
          items={items}
          onSelect={(key) => setSelected(key)}
          renderItem={(item) => (
            <button
              key={item.key}
              className={`block w-full px-4 py-2 text-left text-sm hover:bg-gray-100 ${item.data?.danger ? 'text-red-600' : ''}`}
              onClick={(e) => {
                // onSelect is not called via renderItem, handle inline
                setSelected(item.key)
              }}
            >
              {item.label}
            </button>
          )}
          ariaLabel="Custom rendered menu"
        />
        {selected && (
          <p className="text-sm text-text-foreground-secondary">
            Selected: {selected}
          </p>
        )}
      </div>
    </div>
  )
}

CustomRenderItem.meta = { description: 'Menu with fully custom item renderer' }
