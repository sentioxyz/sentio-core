import { LuTrash2, LuSettings, LuLayers2, LuClipboard } from 'react-icons/lu'
import { PopupMenuButton } from '@sentio/ui-core'
import type { IMenuItem } from '@sentio/ui-core'
import { MouseEvent } from 'react'

const menuItems = [
  [
    {
      label: 'Clone',
      icon: <LuLayers2 className="mr-2 w-4" />,
      key: 'clone'
    },
    {
      label: 'Copy configuration',
      icon: <LuClipboard className="mr-2 w-4" />,
      key: 'copy'
    }
  ],
  [
    {
      label: 'Delete',
      key: 'delete',
      status: 'danger',
      icon: <LuTrash2 className="mr-2 w-4" />
    }
  ]
] as IMenuItem[][]

interface Props {
  onSelect: (key: string, event: MouseEvent) => void
}

export function ExtraSettingMenu({ onSelect }: Props) {
  return (
    <PopupMenuButton
      items={menuItems}
      onSelect={onSelect}
      ariaLabel={'settings'}
      buttonIcon={<LuSettings className="w-4" />}
      buttonClassName="cursor-pointer"
    />
  )
}

export function ReadonlyExtraSettingMenu({ onSelect }: Props) {
  return (
    <PopupMenuButton
      items={[
        [
          {
            label: 'Copy configuration',
            icon: <LuClipboard className="mr-2 w-4" />,
            key: 'copy'
          }
        ]
      ]}
      onSelect={onSelect}
      ariaLabel={'settings'}
      buttonIcon={<LuSettings className="w-4" />}
      buttonClassName="cursor-pointer"
    />
  )
}
