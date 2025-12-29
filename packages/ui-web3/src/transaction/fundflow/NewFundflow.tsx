import { useEffect, useRef, useState, memo, ReactNode } from 'react'
import {
  Button,
  BaseDialog,
  Empty,
  useBoolean,
  useMobile,
  useDarkMode
} from '@sentio/ui-core'
import {
  ArrowDownTrayIcon,
  ArrowsPointingOutIcon,
  XMarkIcon
} from '@heroicons/react/24/outline'
import { DecodedCallTrace } from '@sentio/debugger-common'
import { cx } from 'class-variance-authority'
import {
  TransferItem,
  processDecodedCallTrace,
  generateNodesAndEdges
} from './FlowUtils'
import { exportSVG } from './export-utils'
import { FundFlow } from './FundFlow'
import isEqual from 'lodash/isEqual'
import { Transaction } from '../types'

const DefaultEmptyFundflow = (
  <div className="relative h-full">
    <div className="absolute bottom-0 left-0 right-0 top-0 z-[1] pt-32">
      <Empty title="This transaction has no fund flow" />
    </div>
  </div>
)

interface Props {
  transaction?: Transaction
  data: DecodedCallTrace
  dataLoading?: boolean
  empty?: React.ReactNode
  onEmpty?: (isEmpty: boolean) => void
  chainId?: string
  // Tag-related props (optional)
  tagMap?: Map<string, any>
  defaultTagMap?: Map<string, string>
  setTagAddressList?: (addresses: string[]) => void
  // Custom render function for the graph visualization
  renderGraph?: (graphString: string, zoomable?: boolean) => ReactNode
}

function _TransactionFundflow({
  data,
  transaction,
  dataLoading,
  empty = DefaultEmptyFundflow,
  onEmpty,
  chainId,
  tagMap = new Map(),
  defaultTagMap = new Map(),
  setTagAddressList,
  renderGraph
}: Props) {
  const isDarkMode = useDarkMode()
  const [flowNodes, setNodes] = useState<any[]>([])
  const [flowEdges, setEdges] = useState<any[]>([])
  const {
    value: dialogOpen,
    setTrue: openDialog,
    setFalse: closeDialog
  } = useBoolean(false)
  const ref = useRef<HTMLDivElement>(null)
  const [isLoading, setLoading] = useState(true)
  const [items, setItems] = useState<TransferItem[]>([])
  const dataRef = useRef<any>(null)
  const isMobile = useMobile()

  useEffect(() => {
    if (dataLoading || chainId === undefined) {
      return
    }
    if (isEqual(data, dataRef.current)) {
      return
    }
    dataRef.current = data

    const processedItems = processDecodedCallTrace(
      data,
      chainId,
      setTagAddressList
    )
    setItems(processedItems)
    setLoading(processedItems.length === 0 ? false : true)
  }, [data, dataLoading, chainId, setTagAddressList])

  useEffect(() => {
    if (items.length === 0 || !chainId) {
      setLoading(false)
      return
    }

    const transactionFrom = transaction?.from ?? ''
    const transactionTo = transaction?.to ?? ''

    const { nodes, edges } = generateNodesAndEdges({
      items,
      transactionFrom,
      transactionTo,
      chainId,
      tagMap,
      defaultTagMap,
      isDarkMode,
      trimAddress: isMobile,
      trimAmount: isMobile
    })

    setNodes((pre) => {
      if (isEqual(pre, nodes)) {
        return pre
      }
      return nodes
    })
    setEdges((pre) => (isEqual(pre, edges) ? pre : edges))
    setLoading(false)
  }, [
    items,
    tagMap,
    transaction?.from,
    transaction?.to,
    chainId,
    defaultTagMap,
    isDarkMode,
    isMobile
  ])

  useEffect(() => {
    if (isLoading) {
      return
    }
    onEmpty?.(flowNodes.length === 0)
  }, [isLoading, flowNodes.length, onEmpty])

  if (!isLoading && flowNodes.length === 0) {
    console.log('Rendering empty fundflow')
    return empty
  }

  return (
    <div
      className={cx(
        'relative h-fit w-full overflow-auto',
        isLoading || dataLoading ? 'invisible' : 'visible'
      )}
    >
      <div
        ref={ref}
        className="h-[calc(100vh-200px)] w-[calc(100vw-50px)] overflow-hidden pt-20 sm:h-fit sm:w-full"
      >
        <FundFlow
          nodes={flowNodes}
          edges={flowEdges}
          renderGraph={renderGraph}
        />
      </div>
      <div className="absolute right-2 top-2">
        {isMobile ? null : (
          <Button
            size="md"
            icon={<ArrowsPointingOutIcon />}
            role="text"
            onClick={openDialog}
          ></Button>
        )}
        {flowNodes.length > 0 && (
          <Button
            size="md"
            icon={<ArrowDownTrayIcon />}
            role="text"
            onClick={() => {
              exportSVG(`fundflow of ${transaction?.hash}`, ref.current)
            }}
          />
        )}
      </div>
      <BaseDialog
        title={
          <div className="flex items-center justify-between px-4">
            <h3 className="text-base">{`Fund flow of ${transaction?.hash}`}</h3>
            <Button
              size="md"
              icon={<XMarkIcon />}
              onClick={closeDialog}
              role="text"
            ></Button>
          </div>
        }
        open={dialogOpen}
        onClose={closeDialog}
        panelClassName="md:max-w-max w-screen"
        onOk={closeDialog}
        okText="Close"
      >
        <div className="h-[80vh] w-[90vw]">
          <FundFlow
            nodes={flowNodes}
            edges={flowEdges}
            zoomable
            renderGraph={renderGraph}
          />
        </div>
      </BaseDialog>
    </div>
  )
}

export const TransactionFundflow = memo(_TransactionFundflow)
