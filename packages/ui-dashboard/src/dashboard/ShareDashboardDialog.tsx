import { useMemo, useState } from 'react'
import {
  BaseDialog,
  CopyButton,
  Checkbox,
  dateTimeToString
} from '@sentio/ui-core'
import type { DateTimeValue } from '@sentio/ui-core'
import type {
  DashboardSharingLike,
  SharingConfigLike
} from '../types/dashboard'

interface Props {
  open: boolean
  initData?: DashboardSharingLike
  onUnshare?: () => void
  onClose: () => void
  onConfigChange?: (config: SharingConfigLike) => void
  startTime?: DateTimeValue
  endTime?: DateTimeValue
  tz?: string
}

export const ShareDashboardDialog = ({
  open,
  initData,
  onUnshare,
  onClose,
  onConfigChange,
  startTime,
  endTime,
  tz
}: Props) => {
  const [isReadonly, setIsReadonly] = useState(
    initData?.config?.isReadonly ?? false
  )
  const [hideModifiers, setHideModifiers] = useState(
    initData?.config?.hideModifiers ?? false
  )

  const linkText = useMemo(() => {
    if (initData?.id) {
      let timeSuffix =
        startTime && endTime
          ? `?from=${encodeURIComponent(dateTimeToString(startTime))}&to=${encodeURIComponent(dateTimeToString(endTime))}&tz=${tz}`
          : ''
      if (tz) {
        timeSuffix += `&tz=${tz}`
      }
      return `${location.origin}/share/${initData?.id}${timeSuffix}`
    }
    return ''
  }, [initData?.id, startTime, endTime])

  const handleConfigChange = (
    newIsReadonly: boolean,
    newHideModifiers: boolean
  ) => {
    const config: SharingConfigLike = {
      isReadonly: newIsReadonly,
      hideModifiers: newHideModifiers
    }
    onConfigChange?.(config)
  }

  const handleReadonlyChange = (checked: boolean) => {
    setIsReadonly(checked)
    handleConfigChange(checked, hideModifiers)
  }

  const handleHideModifiersChange = (checked: boolean) => {
    setHideModifiers(checked)
    handleConfigChange(isReadonly, checked)
  }

  return (
    <BaseDialog
      title="Sharing: ON"
      open={open}
      onCancel={() => {
        onUnshare?.()
        onClose()
      }}
      cancelText="Revoke URL"
      cancelProps={{
        status: 'danger'
      }}
      okText="Done"
      onOk={() => {
        onClose()
      }}
      onClose={onClose}
      buttonsClassName="justify-between"
      footerBorder={false}
    >
      <div className="mx-4 my-4">
        <div className="flex overflow-hidden rounded-md border  pl-3">
          <span className="text-ilabel font-ilabel text-text-foreground flex-1 grow truncate leading-8">
            {linkText}
          </span>
          <div className="group cursor-pointer border-l  bg-gray-200 px-2 py-1 hover:bg-gray-100">
            <CopyButton
              text={linkText}
              size={18}
              className="text-text-foreground group-hover:text-primary h-4 w-4 align-middle"
            />
          </div>
        </div>
        <div className="text-text-foreground-secondary mt-2 text-[11px]">
          Anyone with the link can access this dashboard. You can revoke the
          link at any time.
        </div>

        {/* Sharing Configuration Options */}
        <div className="mt-4 space-y-3 border-t pt-4">
          <div className="text-text-foreground text-sm font-medium">
            Access Settings
          </div>

          <div className="space-y-4">
            <div>
              <Checkbox
                checked={isReadonly}
                onChange={handleReadonlyChange}
                label="Panel read-only access"
              />
              <div className="text-text-foreground-secondary ml-6 text-xs">
                Viewers can only view the dashboard without entering panel edit
                mode or copy configuration.
              </div>
            </div>

            <div>
              <Checkbox
                checked={hideModifiers}
                onChange={handleHideModifiersChange}
                label="Hide controls"
              />
              <div className="text-text-foreground-secondary ml-6 text-xs">
                Hide panel creator and modifier for viewers
              </div>
            </div>
          </div>
        </div>
      </div>
    </BaseDialog>
  )
}
