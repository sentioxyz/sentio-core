import { useState, useRef, useCallback, useEffect, useMemo } from 'react'
import { BaseDialog } from '@sentio/ui-core'
import type { DashboardLike } from '../types/dashboard'

interface Props {
  dashboard?: DashboardLike
  open: boolean
  onClose: () => void
  onUpdate: (data: DashboardLike) => Promise<void>
}

export const EditDashboardDialog = ({
  dashboard,
  open,
  onClose,
  onUpdate
}: Props) => {
  const [processing, setProcessing] = useState(false)
  const [updateDisabled, setUpdateDisabled] = useState(true)
  const inputElementRef = useRef<HTMLInputElement | null>(null)

  const onCloseAndReset = useCallback(() => {
    onClose?.()
    if (dashboard?.name && inputElementRef.current) {
      inputElementRef.current.value = dashboard.name
    }
  }, [onClose, dashboard?.name])

  const onOk = useCallback(() => {
    if (!inputElementRef.current?.value) {
      return
    }
    setProcessing(true)
    onUpdate({ ...dashboard, name: inputElementRef.current?.value })
      .then(() => {
        onCloseAndReset()
      })
      .finally(() => {
        setProcessing(false)
      })
  }, [onCloseAndReset, onUpdate, dashboard])

  useEffect(() => {
    if (open && dashboard?.name && inputElementRef.current) {
      inputElementRef.current.value = dashboard.name
    }
  }, [open, dashboard?.name])

  const onInputChange = useCallback(
    (evt: React.ChangeEvent<HTMLInputElement>) => {
      const value = evt.target.value
      if (!value || value === dashboard?.name) {
        setUpdateDisabled(true)
      } else {
        setUpdateDisabled(false)
      }
    },
    [dashboard?.name]
  )

  const okProps = useMemo(
    () => ({
      processing,
      disabled: updateDisabled
    }),
    [updateDisabled, processing]
  )

  return (
    <BaseDialog
      title="Edit Dashboard"
      open={open}
      onClose={onCloseAndReset}
      cancelText="Close"
      onCancel={onCloseAndReset}
      onOk={onOk}
      okProps={okProps}
      okText="Update"
      footerBorder={false}
      initialFocus={inputElementRef}
    >
      <form
        method="dialog"
        className="text-text-foreground relative mb-4 mt-2 px-4"
        onSubmit={onOk}
      >
        <div className="grid py-2 text-sm">
          <div className="sm:text-ilabel text-text-foreground-secondary mb-2 mt-1">
            Dashboard Name
          </div>
          <input
            defaultValue={dashboard?.name}
            placeholder="Provide a new name for your dashboard"
            type="text"
            required={true}
            name="dashboard-name"
            id="new-dashboard-name"
            className="focus:border-primary-600 focus:ring-primary-600/30 focus:ring-3 hover:border-primary-600 sm:text-ilabel border-main block w-full rounded-md"
            ref={inputElementRef}
            onChange={onInputChange}
          />
        </div>
      </form>
    </BaseDialog>
  )
}
