import {
  useState,
  useMemo,
  createContext,
  useContext,
  useCallback,
  useRef,
  useEffect,
  memo,
  CSSProperties,
  ReactNode,
  isValidElement,
  useTransition
} from 'react'
import Tree from './Tree'
import { useVirtualizer } from '@tanstack/react-virtual'
import isNumber from 'lodash/isNumber'
import { cx as classNames } from 'class-variance-authority'
import { throttle } from 'lodash'

type FieldDataNode<T, ChildFieldName extends string = 'children'> = T &
  Partial<Record<ChildFieldName, FieldDataNode<T, ChildFieldName>[]>>

export type DataNode = FieldDataNode<{
  key: KeyType
  title?: React.ReactNode | ((data: DataNode) => React.ReactNode)
  depth?: number
  raw?: any
}>

type KeyType = string | number

export const SUFFIX_NODE_KEY = 'selectedKey_after'
export const ROOT_KEY = 'root'

const TreeContext = createContext<{
  expandKeys: KeyType[]
  onExpand: (key: KeyType) => void
  onClick?: (data: DataNode) => void
}>({
  expandKeys: [],
  onExpand: (key) => {
    console.log(key)
  }
})

const ControledTree = ({
  item,
  selected,
  contentClassName,
  expandIcon,
  collapseIcon
}: {
  item: DataNode
  selected?: boolean
  contentClassName?: string
  expandIcon?: React.ReactNode
  collapseIcon?: React.ReactNode
}) => {
  const { expandKeys, onExpand, onClick } = useContext(TreeContext)
  let titleNode: React.ReactNode
  if (typeof item.title === 'function') {
    titleNode = item.title(item)
  } else {
    titleNode = item.title
  }
  const onOpenClick = useCallback(() => {
    onExpand(item.key)
  }, [onExpand, item.key])
  const onNodeClick = useCallback(() => {
    onClick?.(item)
  }, [item])
  const isLeaf = item.children === undefined || item.children?.length === 0

  return (
    <Tree
      contentClassName={classNames(
        selected ? 'bg-sentio-gray-100' : '',
        item.key === SUFFIX_NODE_KEY ? '!px-0 !py-0' : '',
        contentClassName
      )}
      className={
        item.key === SUFFIX_NODE_KEY
          ? 'sticky left-0 inline-block !overflow-visible'
          : 'group/tree'
      }
      showToggle={!isLeaf}
      open={expandKeys.includes(item.key)}
      depth={item.depth}
      key={item.key}
      content={titleNode}
      onOpenClick={onOpenClick}
      onClick={onNodeClick}
      expandIcon={expandIcon}
      collapseIcon={collapseIcon}
    />
  )
}

const DEFAULT_ROW_HEIGHT = 35

interface Props {
  data?: DataNode[]
  defaultExpandAll?: boolean
  virtual?: boolean
  rowHeight?: number | ((index: number, isSelected?: boolean) => number)
  height?: CSSProperties['height']
  onClick?: (item: DataNode) => void
  suffixNode?: ReactNode
  expandDepth?: number
  contentClassName?: string
  expandIcon?: React.ReactNode
  collapseIcon?: React.ReactNode
  scrollToKey?: KeyType // auto scroll to this key
  scrollIntoView?: boolean
  isRootKey?: (v: KeyType) => boolean
}

const DefaultSuffixNode = <div className="h-[200px]"></div>
function checkRootKeyDefault(v: KeyType) {
  return v === ROOT_KEY
}

export const FlatTree = (props: Props) => {
  const {
    data,
    defaultExpandAll,
    virtual,
    rowHeight = DEFAULT_ROW_HEIGHT,
    height,
    onClick,
    suffixNode,
    expandDepth,
    contentClassName,
    expandIcon,
    collapseIcon,
    scrollToKey,
    scrollIntoView,
    isRootKey = checkRootKeyDefault
  } = props
  const dataRef = useRef<any>(null)
  const selectedKeyRef = useRef<KeyType>()
  const [expandKeys, setExpandKeys] = useState<KeyType[]>([])
  const [selectedKey, setSelectedKey] = useState<KeyType>()
  const parentRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (defaultExpandAll) {
      const flatten = (data: DataNode[]): KeyType[] => {
        return data.reduce((acc, cur) => {
          const { children, key } = cur
          acc.push(key)
          if (children) {
            acc.push(...flatten(children))
          }
          return acc
        }, [] as KeyType[])
      }
      setExpandKeys(flatten(data || []))
    } else {
      setExpandKeys([])
    }
  }, [data, defaultExpandAll])

  const flattenData = useMemo(() => {
    const expandKeysSet = new Set(expandKeys)
    const flatten = (data: DataNode[], depth = 0): DataNode[] => {
      return data.reduce((acc, cur) => {
        const { children } = cur
        acc.push({ ...cur, depth })
        if (children && expandKeysSet.has(cur.key)) {
          acc.push(...flatten(children, depth + 1))
        }
        return acc
      }, [] as DataNode[])
    }
    const list = flatten(data || [])
    // add node after selectedKey item
    if (selectedKey && isValidElement(suffixNode)) {
      const index = list.findIndex((item) => item.key === selectedKey)
      if (index > -1) {
        list.splice(index + 1, 0, {
          key: SUFFIX_NODE_KEY,
          title: suffixNode,
          depth: 0
        })
      }
    }
    return list
  }, [data, expandKeys, selectedKey, suffixNode])
  dataRef.current = flattenData

  const estimateSize = useCallback(
    (index: number) => {
      if (isNumber(rowHeight)) {
        return rowHeight
      }
      return rowHeight(index, dataRef.current?.[index]?.key === SUFFIX_NODE_KEY)
    },
    [rowHeight]
  )

  const rowVirtualizer = useVirtualizer({
    count: flattenData.length,
    getScrollElement: () => parentRef.current,
    estimateSize,
    overscan: 10
  })

  const contextValue = useMemo(() => {
    return {
      expandKeys: expandKeys,
      onExpand: (key: KeyType) => {
        setExpandKeys((keys) => {
          if (keys.includes(key)) {
            return keys.filter((k) => k !== key)
          } else {
            return [...keys, key]
          }
        })
      },
      onClick: (data: DataNode) => {
        if (onClick === undefined) {
          return
        }
        if (data.key === SUFFIX_NODE_KEY || isRootKey(data.key)) {
          return
        }
        setSelectedKey((key) => {
          if (key === data.key) {
            selectedKeyRef.current = undefined
            return undefined
          }
          selectedKeyRef.current = data.key
          return data.key
        })
        onClick?.(data)
      }
    }
  }, [expandKeys, onClick])

  useEffect(() => {
    // set expandedKey by depth
    if (!isNumber(expandDepth)) {
      return
    }

    const flatten = (data: DataNode[], currentDepth = 0): KeyType[] => {
      return data.reduce((acc, cur) => {
        const { children, key } = cur
        // Expand nodes that are at depth less than expandDepth
        if (currentDepth < expandDepth && children && children.length > 0) {
          acc.push(key)
          acc.push(...flatten(children, currentDepth + 1))
        }
        return acc
      }, [] as KeyType[])
    }
    setExpandKeys(flatten(data || []))
  }, [data, expandDepth])

  useEffect(() => {
    setSelectedKey(undefined)
  }, [expandDepth])

  useEffect(() => {
    if (dataRef.current && scrollToKey) {
      const index = dataRef.current.findIndex(
        (item: any) => item.key === scrollToKey
      )
      if (index > -1) {
        rowVirtualizer.scrollToIndex(index, {
          align: 'center',
          behavior: 'auto'
        })
      }
    }
  }, [scrollToKey])

  const visibleItems = rowVirtualizer.getVirtualItems()
  const [isPending, startTransition] = useTransition()
  const onScroll = useMemo(() => {
    if (!scrollIntoView) {
      return () => {}
    }
    const throttleFn = throttle(
      () => {
        startTransition(() => {
          parentRef.current?.scrollIntoView(true)
        })
      },
      1000,
      { trailing: true }
    )
    let lastScrollTop = 0
    return (evt: React.UIEvent<HTMLDivElement>) => {
      const scrollTop = evt.currentTarget.scrollTop
      if (scrollTop > lastScrollTop) {
        throttleFn()
      }
      lastScrollTop = scrollTop
    }
  }, [scrollIntoView])

  return (
    <TreeContext.Provider value={contextValue}>
      {virtual ? (
        <div
          className="overflow-auto"
          ref={parentRef}
          style={{
            height
          }}
          onScroll={onScroll}
        >
          <div
            style={{
              height: `${rowVirtualizer.getTotalSize()}px`,
              width: '100%',
              position: 'relative'
            }}
          >
            <div
              className="absolute left-0 top-0 w-max min-w-full"
              style={{
                transform: visibleItems?.[0]?.start
                  ? `translateY(${visibleItems[0].start}px)`
                  : undefined
              }}
            >
              {rowVirtualizer.getVirtualItems().map((virtualItem) => (
                <div
                  key={virtualItem.key}
                  className={
                    dataRef.current[virtualItem.index].key === scrollToKey
                      ? 'bg-primary-100'
                      : ''
                  }
                >
                  <ControledTree
                    item={dataRef.current[virtualItem.index]}
                    selected={
                      selectedKey === dataRef.current[virtualItem.index].key
                    }
                    contentClassName={contentClassName}
                    expandIcon={expandIcon}
                    collapseIcon={collapseIcon}
                  />
                </div>
              ))}
            </div>
          </div>
        </div>
      ) : (
        <>
          {flattenData.map((item, index) => (
            <div
              key={item.key || index}
              className={item.key === scrollToKey ? 'bg-primary-100' : ''}
            >
              <ControledTree
                item={item}
                selected={selectedKey === item.key}
                contentClassName={contentClassName}
                expandIcon={expandIcon}
                collapseIcon={collapseIcon}
              />
            </div>
          ))}
        </>
      )}
    </TreeContext.Provider>
  )
}

export default memo(FlatTree)
