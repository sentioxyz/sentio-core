import React from 'react'
import { BarLoading, Button } from '@sentio/ui-core'
import { useResizeDetector } from 'react-resize-detector'
import { isString } from 'lodash'
import { RefreshButton } from '../charts/RefreshContext'

interface Props {
  data?: any
  logoSrc?: string
  onNavigateToDatasource?: () => void
}

export const ErrorChart = React.memo(function ErrorChart({
  data,
  logoSrc,
  onNavigateToDatasource
}: Props) {
  const { ref, width, height } = useResizeDetector()
  let hintMessage: React.ReactNode
  if (data?.code === 5) {
    hintMessage = (
      <span>
        No active processor found
        <br /> Check the
        <Button role="link" onClick={onNavigateToDatasource}>
          datasource
        </Button>
        page for more details.
      </span>
    )
  } else if (data?.status === 429) {
    return (
      <BarLoading
        className="bg-default-bg absolute w-full"
        hint={
          <span className="text-xs font-normal">
            Too many simultaneous requests, retrying later...
          </span>
        }
        width={160}
      />
    )
  } else {
    hintMessage = data?.message ?? 'Something went wrong'
  }
  const imageSize = Math.min(
    Math.min(64, width ? width * 0.4 : 64),
    Math.min(64, height ? height * 0.4 : 64)
  )
  return (
    <div className="flex h-full w-full items-center" ref={ref}>
      <div className="w-full space-y-4 text-center">
        {imageSize < 10 || !logoSrc ? null : (
          <img
            className="mx-auto"
            src={logoSrc}
            width={imageSize}
            height={imageSize}
            style={{ width: imageSize, height: 'auto' }}
            alt="gray logo"
          />
        )}
        <div
          title={isString(hintMessage) ? hintMessage : undefined}
          className={`text-text-foreground-secondary font-icontent text-icontent px-4 ${
            height && height < 200 ? 'line-clamp-1' : 'line-clamp-2'
          }`}
        >
          {hintMessage}
        </div>
        <RefreshButton />
      </div>
    </div>
  )
})
