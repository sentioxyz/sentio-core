import { memo, ReactNode, useMemo, useRef } from 'react'
import { PopupMenuButton } from '@sentio/ui-core'
import type { IMenuItem } from '@sentio/ui-core'
import { ExportChartMenu } from './ExportChartMenu'
import { ExtraSettingMenu, ReadonlyExtraSettingMenu } from './ExtraSettingMenu'
import { AiOutlineApi } from 'react-icons/ai'
import {
  LuCamera,
  LuClipboard,
  LuDownload,
  LuExpand,
  LuLayers2,
  LuLink2,
  LuPencil,
  LuRefreshCw,
  LuSettings,
  LuSquareArrowOutUpRight,
  LuTrash2
} from 'react-icons/lu'

interface Props {
  onMenuSelect: (selectKey: string) => void
  allowEdit?: boolean
  allowViewPanel?: boolean
  allowExport?: boolean
  allowFullScreen?: boolean
  allowEmbed?: boolean
  /** Whether "Export as API" (curl) is available. Injected by the consumer (tier/user-aware). */
  canExportCurl?: boolean
  /** Hint shown when curl export is disabled. Injected by the consumer. */
  exportCurlHint?: ReactNode
}

const StaticButtons: Record<string, IMenuItem> = {
  fullscreen: {
    label: 'Full screen',
    icon: <LuExpand className="mr-2 w-4" />,
    key: 'fullscreen'
  },
  refresh: {
    label: 'Refresh panel',
    icon: <LuRefreshCw className="mr-2 w-4" />,
    key: 'refresh'
  },
  edit: {
    label: 'Edit panel',
    icon: <LuPencil className="mr-2 w-4" />,
    key: 'edit'
  },
  clone: {
    label: 'Clone',
    icon: <LuLayers2 className="mr-2 w-4" />,
    key: 'clone'
  },
  copy: {
    label: 'Copy configuration',
    icon: <LuClipboard className="mr-2 w-4" />,
    key: 'copy'
  },
  delete: {
    label: 'Delete',
    key: 'delete',
    status: 'danger',
    icon: <LuTrash2 className="mr-2 w-4" />
  }
}

const DashboardButtons = ({
  allowEdit,
  onMenuSelect,
  allowExport = true,
  allowFullScreen = true,
  allowViewPanel = true,
  allowEmbed = false,
  canExportCurl = false,
  exportCurlHint
}: Props) => {
  const menuRef = useRef<HTMLDivElement>(null)
  const exportButton = allowExport ? (
    <ExportChartMenu
      allowEdit={allowEdit}
      onSelect={onMenuSelect}
      allowEmbed={allowEmbed}
      canExportCurl={canExportCurl}
      exportCurlHint={exportCurlHint}
    />
  ) : null
  const fullScreenButton = allowFullScreen ? (
    <button
      type="button"
      className="text-text-foreground-secondary hover:text-primary-600 w-6 px-1"
      aria-label="Full screen"
      onClick={() => {
        onMenuSelect('fullscreen')
      }}
    >
      <LuExpand />
    </button>
  ) : null

  const items: IMenuItem[][] = useMemo(() => {
    const ret: IMenuItem[][] = []
    const curlItem: IMenuItem = {
      label: 'Export as API',
      icon: <AiOutlineApi className="mr-2 h-4 w-4" />,
      key: 'curl'
    }
    if (!canExportCurl) {
      curlItem.disabled = true
      curlItem.disabledHint = exportCurlHint
    }
    const lastGroup: IMenuItem[] = [curlItem]
    if (allowEmbed) {
      lastGroup.push({
        label: 'Embed Iframe',
        icon: <LuLink2 className="mr-2 h-4 w-4" />,
        key: 'embed'
      })
    }
    const exportGroup: IMenuItem = {
      label: 'Export',
      icon: <LuSquareArrowOutUpRight className="mr-2 w-4" />,
      key: 'export',
      items: [
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
        lastGroup
      ]
    }
    if (allowFullScreen || allowExport) {
      ret.push([
        ...(allowFullScreen ? [StaticButtons.fullscreen] : []),
        ...(allowExport ? [exportGroup] : [])
      ])
    }
    const editButtons: IMenuItem[] = [StaticButtons.refresh]
    if (allowViewPanel) {
      editButtons.push(StaticButtons.edit)
    }
    ret.push(editButtons)
    if (allowEdit) {
      ret.push(
        [StaticButtons.clone, StaticButtons.copy],
        [StaticButtons.delete]
      )
    } else if (allowViewPanel) {
      ret.push([StaticButtons.copy])
    }
    return ret
  }, [
    allowEdit,
    allowExport,
    allowFullScreen,
    allowViewPanel,
    allowEmbed,
    canExportCurl,
    exportCurlHint
  ])

  return (
    <>
      <div className="hidden group-[.xs]:flex" ref={menuRef}>
        <PopupMenuButton
          onSelect={onMenuSelect}
          items={items}
          ariaLabel="dropdown menu"
          buttonIcon={<LuSettings className="w-4" />}
        />
      </div>
      <div className="flex group-[.xs]:hidden">
        {fullScreenButton}
        {exportButton}
        {allowViewPanel && (
          <button
            type="button"
            className="text-text-foreground-secondary hover:text-primary-600 w-6 px-1"
            aria-label="Edit panel"
            onClick={() => {
              onMenuSelect('edit')
            }}
          >
            <LuPencil />
          </button>
        )}
        {allowEdit ? (
          <ExtraSettingMenu onSelect={onMenuSelect} />
        ) : allowViewPanel ? (
          <ReadonlyExtraSettingMenu onSelect={onMenuSelect} />
        ) : null}
      </div>
    </>
  )
}

export const DashboardButtonsMemo = memo(DashboardButtons)
