import { useCallback, useEffect, useState, useRef, useMemo } from 'react'
import { useResizeDetector } from 'react-resize-detector'
import { Location } from '@sentio/debugger'
import {
  FlatCallTraceTree,
  LocationViewer,
  OverviewContext,
  SpinLoading
} from '@sentio/ui-web3'

import { useTxnModel } from '~/content/lib/debug/use-transaction-model'
import { useCallTrace } from '~/content/lib/debug/use-call-trace'
import { sentioTxUrl } from '~/utils/url'
import { RelatedTxn } from '../contract/RelatedTxn'
import { SlideoverProvider, useSlideoverContext } from './SlideoverContext'

enum LocationStatus {
  None = 0,
  NotFound = 1,
  Loading = 2,
  Error = 3
}

interface Props {
  hash: string
  chainId: string
}

const CallTracePanelContent = ({ hash, chainId }: Props) => {
  const { data: callTrace, loading } = useCallTrace(hash, chainId, true)
  const [defLocation, setDefLocation] = useState<Location>()
  const [currentLocation, setCurrentLocation] = useState<
    Location | LocationStatus
  >(LocationStatus.None)
  const [currentModel, setCurrentModel] = useState<any>()
  const modelRefreshRef = useRef<any>()
  const { getModel, store } = useTxnModel(hash, chainId)
  const [expandDepth, setExpandDepth] = useState()
  const { width, ref: wrapperRef } = useResizeDetector({ handleHeight: false })
  const placeholderRef = useRef<HTMLDivElement>(null)
  const editorContainerRef = useRef<HTMLDivElement>(null)
  const [contractAddress, setContractAddress] = useState<string>('')

  const { sig, setSig, contract, setContract, visible, openSlideOver } =
    useSlideoverContext()

  useEffect(() => {
    // calculate the position of containerRef
    setTimeout(() => {
      if (
        placeholderRef.current === null ||
        editorContainerRef.current === null ||
        wrapperRef.current === null
      ) {
        return
      }
      const { top: wrapperTop } = wrapperRef.current.getBoundingClientRect()
      const { top: placeholderTop } =
        placeholderRef.current.getBoundingClientRect()
      editorContainerRef.current.style.top = `${placeholderTop - wrapperTop - 1}px`
    }, 0)
  }, [currentLocation, wrapperRef])

  const overviewContext = useMemo(() => {
    return {
      routeTo: (path = '', dropBuild = false) => {
        window.open(sentioTxUrl(chainId, hash) + '/' + path, '_blank')
      },
      setMask: () => {}
    }
  }, [])

  const onInstruction = useCallback(
    (index, location?: Location, dlocation?: Location) => {
      if (!getModel) {
        return
      }
      if (index === -1 || location?.compilationId === undefined) {
        setCurrentLocation(LocationStatus.NotFound)
        setDefLocation(dlocation)
        return
      }
      setCurrentLocation(LocationStatus.Loading)

      new Promise((resolve, reject) => {
        if (modelRefreshRef.current) {
          modelRefreshRef.current()
        }
        modelRefreshRef.current = reject
        const fetchModel = () => {
          const model = getModel({
            compilationId: location.compilationId ?? '',
            filePath: location?.sourcePath ?? ''
          })
          if (model === undefined) {
            // model is undefined when the model is not ready
            setTimeout(() => {
              fetchModel()
            }, 1000)
          } else {
            resolve(model)
          }
        }
        fetchModel()
      })
        .then((model) => {
          setCurrentModel(model)
          setDefLocation(dlocation)
          if (model === null) {
            setCurrentLocation(LocationStatus.NotFound)
          } else {
            setCurrentLocation(location)
          }
        })
        .catch((e) => {
          setCurrentLocation(LocationStatus.Error)
        })
    },
    [getModel]
  )

  const onPreviewDef = useCallback(
    (location) => {
      if (!getModel) {
        return
      }
      const model = getModel({
        compilationId: location.compilationId ?? '',
        filePath: location?.sourcePath ?? ''
      })
      if (!model) {
        return
      }
      setCurrentModel(model)
      setCurrentLocation(location)
    },
    [defLocation, getModel]
  )

  console.log('current Location', currentLocation, defLocation)

  return (
    <OverviewContext.Provider value={overviewContext}>
      <div ref={wrapperRef}>
        <SpinLoading loading={loading} showMask className="min-h-[600px]">
          {callTrace && (
            <FlatCallTraceTree
              virtual
              height={600}
              data={callTrace}
              onInstruction={onInstruction}
              expander={expandDepth}
              editorNode={
                <div
                  className={
                    currentLocation === LocationStatus.None ? 'hidden' : ''
                  }
                  ref={placeholderRef}
                  style={{ width: width ? width - 30 : undefined, height: 200 }}
                >
                  <LocationViewer
                    currentLocation={currentLocation}
                    defLocation={defLocation}
                    onPreviewDef={onPreviewDef}
                    currentModel={currentModel}
                    store={store}
                    contractAddress={contractAddress}
                    setContractAddress={setContractAddress}
                    setSig={setSig}
                    setContract={setContract}
                    openSlideOver={openSlideOver}
                  />
                </div>
              }
              gasUsed={true}
            />
          )}
        </SpinLoading>
      </div>
      <RelatedTxn
        chainId={chainId}
        open={visible as boolean}
        onClose={() => {
          openSlideOver(false)
        }}
        sig={sig as string}
        address={contract as string}
      />
    </OverviewContext.Provider>
  )
}

export const CallTracePanel = ({ hash, chainId }: Props) => {
  return (
    <SlideoverProvider>
      <CallTracePanelContent hash={hash} chainId={chainId} />
    </SlideoverProvider>
  )
}
