import { memo, MutableRefObject, ReactNode } from 'react'
import { Location } from '@sentio/debugger'
import { BarLoading } from '@sentio/ui-core'
import { ForwardDef } from './ForwardDef'
import { SourceView } from '../SourceView'

export enum LocationStatus {
  None = 0,
  NotFound = 1,
  Loading = 2,
  Error = 3
}

export function isLocationStatus(v: Location | LocationStatus) {
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
  // Callbacks for search functionality
  setSig?: (sig: string) => void
  setContract?: (contract: string) => void
  openSlideOver?: (visible: boolean) => void
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
  setSig,
  setContract,
  openSlideOver
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

  return (
    <SourceView
      model={currentModel}
      location={
        isLocationStatus(currentLocation)
          ? undefined
          : (currentLocation as Location)
      }
      store={store}
      setSig={setSig}
      setContract={setContract}
      openSlideOver={openSlideOver}
      openRefSlider={openSlider}
      setRefSliderData={setSliderData}
      contractAddress={contractAddress}
      setContractAddress={setContractAddress}
      onOpenRef={onOpenRef}
      chain={chainId}
      isDarkMode={isDarkMode}
    />
  )
})
