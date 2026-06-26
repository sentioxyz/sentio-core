import { MouseEvent, ReactNode, useMemo } from 'react'
import { produce } from 'immer'
import { PopupMenuButton } from '@sentio/ui-core'
import type { IMenuItem } from '@sentio/ui-core'
import { AiOutlineApi } from 'react-icons/ai'
import {
  LuLink2,
  LuSquareArrowOutUpRight,
  LuDownload,
  LuCamera
} from 'react-icons/lu'

const DefaultMenuItems: IMenuItem[][] = [
  [
    {
      label: 'Take a snapshot',
      icon: <LuCamera className="mr-2 w-4" />,
      key: 'snapshot'
    }
  ],
  [
    {
      key: 'png',
      label: 'Export as PNG',
      icon: <LuSquareArrowOutUpRight className="mr-2 w-4" />
    },
    {
      key: 'svg',
      label: 'Export as SVG',
      icon: <LuDownload className="mr-2 w-4" />
    },
    {
      key: 'csv',
      label: 'Export as CSV',
      icon: <LuDownload className="mr-2 w-4" />
    }
  ],
  [
    {
      label: 'Export as API',
      icon: <AiOutlineApi className="mr-2 h-4 w-4" />,
      key: 'curl'
    }
  ]
]

interface Props {
  allowEdit?: boolean
  onSelect: (key: string, event: MouseEvent) => void
  allowEmbed?: boolean
  /** Whether the "Export as API" (curl) action is available. Injected by the consumer (tier/user-aware). */
  canExportCurl?: boolean
  /** Hint shown when curl export is disabled. Injected by the consumer. */
  exportCurlHint?: ReactNode
}

export function ExportChartMenu({
  allowEdit,
  onSelect,
  allowEmbed,
  canExportCurl,
  exportCurlHint
}: Props) {
  const menuItems = useMemo(() => {
    return produce(DefaultMenuItems, (draft) => {
      draft[0][0].disabled = !allowEdit
      const last = draft[draft.length - 1]
      last[0].disabled = !canExportCurl || !allowEdit
      last[0].disabledHint = exportCurlHint
      if (allowEmbed) {
        last.push({
          label: 'Embed Iframe',
          icon: <LuLink2 className="mr-2 h-4 w-4" />,
          key: 'embed'
        })
      }
    })
  }, [allowEdit, canExportCurl, exportCurlHint, allowEmbed])

  return (
    <PopupMenuButton
      items={menuItems}
      onSelect={onSelect}
      ariaLabel={'export'}
      buttonIcon={<LuSquareArrowOutUpRight className="w-4" />}
      buttonClassName="cursor-pointer"
    />
  )
}
