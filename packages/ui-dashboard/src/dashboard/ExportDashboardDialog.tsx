import { BaseDialog, Button, CopyButton, useDarkMode } from '@sentio/ui-core'
import MonacoEditor from '@monaco-editor/react'
import type { BeforeMount } from '@monaco-editor/react'
import { LuDownload } from 'react-icons/lu'

interface Props {
  open: boolean
  onClose: () => void
  dashboardId?: string
  json: string
  onBeforeMount?: BeforeMount
}

export function ExportDashboardDialog({
  open,
  onClose,
  dashboardId,
  json,
  onBeforeMount
}: Props) {
  const isDarkMode = useDarkMode()

  return (
    <BaseDialog
      title="Export dashboard JSON"
      open={open}
      onClose={onClose}
      onCancel={onClose}
      cancelText="Close"
      footerBorder={false}
      extraButtons={
        <div className="absolute left-4 inline-flex">
          <a
            download={
              dashboardId ? `dashboard-${dashboardId}.json` : 'dashboard.json'
            }
            href={'data:text/json;charset=utf-8,' + encodeURIComponent(json)}
          >
            <Button role="text" icon={<LuDownload />}>
              Save to a file
            </Button>
          </a>
        </div>
      }
    >
      <form className="relative">
        <div className="px-[18px] py-4">
          <div
            className="absolute right-10 top-8 z-10"
            onClick={(evt) => evt.preventDefault()}
          >
            <CopyButton text={json} size={16} />
          </div>
          <div className="focus-within:border-primary-300 h-[324px] overflow-hidden rounded-sm border">
            <MonacoEditor
              value={json}
              theme={isDarkMode ? 'sentio-dark' : 'sentio'}
              language="json"
              beforeMount={onBeforeMount}
              options={{
                readOnly: true,
                minimap: { enabled: false },
                lineNumbers: 'off'
              }}
            />
          </div>
        </div>
      </form>
    </BaseDialog>
  )
}
