import { memo, MutableRefObject, ReactNode } from 'react'
import { Location } from '@sentio/debugger'
import { BarLoading } from '@sentio/ui-core'
import { ForwardDef } from './ForwardDef'

export enum LocationStatus {
  None = 0,
  NotFound = 1,
  Loading = 2,
  Error = 3
}

function isLocationStatus(v: Location | LocationStatus) {
  return [
    LocationStatus.None,
    LocationStatus.NotFound,
    LocationStatus.Loading,
    LocationStatus.Error
  ].includes(v as LocationStatus)
}

interface LocationViewerProps {
  currentLocation: Location | LocationStatus
  defLocation?: Location
  onPreviewDef?: (location: Location) => void
  currentModel?: any
  store?: any
  openSlider?: ((tabName?: string | undefined) => void) | undefined
  setSliderData?: ((data: any) => void) | undefined
  contractAddress?: string
  setContractAddress?: (addr: string) => void
  onOpenRef?: MutableRefObject<
    (
      address: string,
      filePath: string,
      line?: number | undefined
    ) => void | undefined
  >
  chainId?: string
  isDarkMode?: boolean
  // Custom render function for the source view when location is available
  renderSourceView?: (props: {
    location: Location
    model?: any
    store?: any
    contractAddress?: string
    setContractAddress?: (addr: string) => void
    chainId?: string
    isDarkMode?: boolean
    onOpenRef?: MutableRefObject<
      (
        address: string,
        filePath: string,
        line?: number | undefined
      ) => void | undefined
    >
    openSlider?: ((tabName?: string | undefined) => void) | undefined
    setSliderData?: ((data: any) => void) | undefined
  }) => ReactNode
}

export const LocationViewer = memo(function LocationViewer({
  currentLocation,
  defLocation,
  onPreviewDef,
  currentModel,
  store,
  openSlider,
  setSliderData,
  contractAddress,
  setContractAddress,
  onOpenRef,
  chainId,
  isDarkMode,
  renderSourceView
}: LocationViewerProps) {
  if (currentLocation === LocationStatus.NotFound) {
    return (
      <div className="mx-auto my-6 space-y-3 text-center">
        <div className="text-text-foreground text-base font-bold">
          No Source
        </div>
        <div className="text-icontent font-icontent text-gray space-y-2">
          <div>
            Unfortunately we do not have the source code for this contract to
            display the exact line of code at callsite.
          </div>
          <ForwardDef location={defLocation} onClick={onPreviewDef} />
        </div>
      </div>
    )
  }
  if (currentLocation === LocationStatus.Loading) {
    return (
      <div className="h-full w-full">
        <BarLoading hint="Loading Source file" />
      </div>
    )
  }
  if (currentLocation === LocationStatus.Error) {
    return (
      <div className="mx-auto my-6 space-y-3 text-center">
        <div className="text-text-foreground text-base font-bold">
          No Source
        </div>
        <div className="text-icontent font-icontent text-gray space-y-2">
          <div>
            Error happened when fetching source code, please try again later.
          </div>
          <ForwardDef location={defLocation} onClick={onPreviewDef} />
        </div>
      </div>
    )
  }
  if (currentLocation === LocationStatus.None) {
    return null
  }

  // If a custom render function is provided, use it
  if (renderSourceView && !isLocationStatus(currentLocation)) {
    return (
      <>
        {renderSourceView({
          location: currentLocation as Location,
          model: currentModel,
          store,
          contractAddress,
          setContractAddress,
          chainId,
          isDarkMode,
          onOpenRef,
          openSlider,
          setSliderData
        })}
      </>
    )
  }

  // Default fallback if no custom render is provided
  return (
    <div className="mx-auto my-6 space-y-3 text-center">
      <div className="text-text-foreground text-base font-bold">
        Source View
      </div>
      <div className="text-icontent font-icontent text-gray">
        Please provide a custom renderSourceView function to display source
        code.
      </div>
    </div>
  )
})
