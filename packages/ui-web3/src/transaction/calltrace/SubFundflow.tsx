import {
  createContext,
  useContext,
  useState,
  useCallback,
  ReactNode,
  useEffect,
  useRef
} from 'react'
import { BaseDialog, useDarkMode } from '@sentio/ui-core'
import { DecodedCallTrace } from '@sentio/debugger-common'
import { LuRoute } from 'react-icons/lu'
import { XMarkIcon } from '@heroicons/react/24/outline'
import isEqual from 'lodash/isEqual'
import {
  TransferItem,
  processDecodedCallTrace,
  generateNodesAndEdges
} from '../fundflow/FlowUtils'
import { FundFlow } from '../fundflow/FundFlow'
import { useFallbackNameMap } from '../use-fallback-name'
import { Transaction } from '../types'
import { TagCacheContext } from '../../utils/tag-context'
import { ChainIdContext } from '../transaction-context'

interface SubFundflowContextType {
  open: (data: DecodedCallTrace) => void
  close: () => void
}

const SubFundflowContext = createContext<SubFundflowContextType | undefined>(
  undefined
)

export const useSubFundflow = () => {
  const context = useContext(SubFundflowContext)
  if (!context) {
    return {
      open: () => {},
      close: () => {}
    }
  }
  return context
}

interface SubFundflowProviderProps {
  children: ReactNode
  transaction?: Transaction
}

export function SubFundflowProvider({
  children,
  transaction
}: SubFundflowProviderProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [flowNodes, setNodes] = useState<any[]>([])
  const [flowEdges, setEdges] = useState<any[]>([])
  const [items, setItems] = useState<TransferItem[]>([])
  const tagMap = useContext(TagCacheContext).tagCache
  const defaultTagMap = useFallbackNameMap()
  const dataRef = useRef<DecodedCallTrace | null>(null)
  const chainId = useContext(ChainIdContext)
  const isDarkMode = useDarkMode()

  const processData = useCallback(
    (data: DecodedCallTrace) => {
      if (!chainId) return

      if (isEqual(data, dataRef.current)) {
        return
      }
      dataRef.current = data

      const processedItems = processDecodedCallTrace(data, chainId)
      setItems(processedItems)
    },
    [chainId]
  )

  useEffect(() => {
    if (items.length === 0 || !chainId) {
      setNodes([])
      setEdges([])
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
      isDarkMode
    })

    setNodes((pre) => {
      if (isEqual(pre, nodes)) {
        return pre
      }
      return nodes
    })
    setEdges((pre) => (isEqual(pre, edges) ? pre : edges))
  }, [
    items,
    tagMap,
    transaction?.from,
    transaction?.to,
    chainId,
    defaultTagMap,
    isDarkMode
  ])

  const open = useCallback(
    (data: DecodedCallTrace) => {
      processData(data)
      setIsOpen(true)
    },
    [processData]
  )

  const close = useCallback(() => {
    setIsOpen(false)
  }, [])

  const contextValue: SubFundflowContextType = {
    open,
    close
  }

  // Global Esc key listener
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && isOpen) {
        close()
      }
    }

    if (isOpen) {
      document.addEventListener('keydown', handleKeyDown)
    }

    return () => {
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen, close])

  return (
    <SubFundflowContext.Provider value={contextValue}>
      {children}
      <BaseDialog
        title={
          <div className="flex items-center justify-between px-4">
            <h3 className="text-base">{`Sub fund flow of ${dataRef.current?.functionName}`}</h3>
            <button
              onClick={close}
              className="rounded p-1 hover:bg-gray-100 dark:hover:bg-gray-700"
            >
              <XMarkIcon className="h-5 w-5" />
            </button>
          </div>
        }
        open={isOpen}
        onClose={close}
        panelClassName="md:max-w-max w-screen"
        onOk={close}
        okText="Close"
      >
        <div className="h-[80vh] w-[90vw]">
          {flowNodes.length > 0 ? (
            <FundFlow nodes={flowNodes} edges={flowEdges} zoomable />
          ) : (
            <div className="flex h-full flex-col items-center justify-center text-gray-500 dark:text-gray-400">
              <div className="text-center">
                <LuRoute className="mx-auto h-12 w-12 text-gray-400 dark:text-gray-500" />
                <h3 className="mt-4 text-sm font-medium text-gray-900 dark:text-gray-100">
                  No fund flow available
                </h3>
                <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                  There are no fund transfers to display for this call trace.
                </p>
              </div>
            </div>
          )}
        </div>
      </BaseDialog>
    </SubFundflowContext.Provider>
  )
}
