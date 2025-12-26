import {
  CSSProperties,
  forwardRef,
  memo,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState
} from 'react'
import {
  Row,
  Cell,
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  ColumnDef,
  flexRender,
  ColumnResizeMode,
  TableState
} from '@tanstack/react-table'
import { useVirtualizer } from '@tanstack/react-virtual'
import { classNames } from '../../utils/classnames'
import {
  HiOutlineSortDescending,
  HiOutlineSortAscending,
  HiChevronDown
} from 'react-icons/hi'
import { debounce, isEqual } from 'lodash'
import { PopupMenuButton } from '../menu/PopupMenuButton'
import { ColumnOrderState } from '@tanstack/react-table'
import { MoveLeftIcon, MoveRightIcon, RenameIcon, DeleteIcon } from './Icons'
import { IoReload } from 'react-icons/io5'

const reorder = (list: any[], startIndex: number, endIndex: number) => {
  const result = Array.from(list)
  const [removed] = result.splice(startIndex, 1)
  result.splice(endIndex, 0, removed)

  return result
}

interface Props {
  data: any
  columns: ColumnDef<any>[]
  columnResizeMode: ColumnResizeMode
  onClick?: (row: Row<any>, cell: Cell<any, any>) => void
  height?: CSSProperties['height']
  onFetchMore?: () => void
  hasMore?: boolean
  isFetching?: boolean

  state?: Partial<TableState>
  onStateChange?: (state: TableState) => void

  //sort
  allowSort?: boolean
  manualSorting?: boolean // server-side sorting

  //resize
  allowResizeColumn?: boolean

  //edit column
  allowEditColumn?: boolean
  onColumnRename?: (data: ColumnDef<any>) => void
  onColumnRemove?: (data: ColumnDef<any>) => void

  minSize?: number
  minWidth?: number

  rowClassNameFn?: (row: Row<any>) => string

  // Virtual scrolling options
  enableVirtualization?: boolean
  estimatedRowHeight?: number
  overscan?: number
}

function onPreventClick(e: React.MouseEvent) {
  e.stopPropagation()
}

const _ResizeTable = forwardRef<HTMLDivElement, Props>(function _ResizeTable(
  {
    data,
    columns,
    columnResizeMode,
    onClick,
    height,
    onFetchMore,
    hasMore,
    isFetching,
    allowSort,
    allowEditColumn,
    allowResizeColumn,
    state = {},
    onStateChange,
    onColumnRemove,
    onColumnRename,
    minSize,
    manualSorting,
    minWidth,
    rowClassNameFn,
    enableVirtualization = false,
    estimatedRowHeight = 35,
    overscan = 5
  }: Props,
  tableContainerRef
) {
  const adjustedColumns = useMemo(() => {
    let totalWidth = 0
    const newColumns = columns.map((c) => {
      const item = Object.assign({ minSize }, c)
      totalWidth += item.size || item.minSize || 0
      return item
    })
    if (minWidth && totalWidth < minWidth) {
      const ratio = minWidth / totalWidth
      newColumns.forEach((c) => {
        if (c.size) {
          c.size = Math.floor(c.size * ratio)
        } else if (c.minSize) {
          c.size = Math.floor(c.minSize * ratio)
        }
      })
    }
    return newColumns
  }, [columns, minSize, minWidth])

  const [tableState, setTableState] = useState<TableState>(() => {
    const initialState = {
      pagination: {
        pageIndex: 0,
        pageSize: 10
      },
      ...state
    }
    return initialState as any
  })

  const table = useReactTable({
    data,
    columns: adjustedColumns,
    columnResizeMode: columnResizeMode,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: allowSort ? getSortedRowModel() : undefined,
    state: tableState,
    onStateChange: setTableState,
    manualSorting
  })

  useEffect(() => {
    if (state && Object.keys(state).length > 0) {
      setTableState((prev) => {
        const newState: TableState = {
          ...prev,
          ...state,
          pagination: prev.pagination ||
            state.pagination || { pageIndex: 0, pageSize: 10 }
        }
        return isEqual(prev, newState) ? prev : newState
      })
    }
  }, [state])

  const debounceStateChange = useMemo(() => {
    if (!onStateChange) return undefined
    return debounce(onStateChange, 500, {})
  }, [onStateChange])

  useEffect(() => {
    debounceStateChange?.(tableState)
  }, [debounceStateChange, tableState])

  const fetchMoreOnBottomReached = useMemo(() => {
    return debounce((containerRefElement?: HTMLDivElement | null) => {
      if (containerRefElement) {
        const { scrollHeight, scrollTop, clientHeight } = containerRefElement
        if (
          scrollHeight - scrollTop - clientHeight < 300 &&
          !isFetching &&
          hasMore
        ) {
          onFetchMore?.()
        }
      }
    }, 500)
  }, [onFetchMore, isFetching, hasMore])

  const tableContainerElementRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (tableContainerRef) {
      if (typeof tableContainerRef === 'function') {
        tableContainerRef(tableContainerElementRef.current)
      } else {
        tableContainerRef.current = tableContainerElementRef.current
      }
    }
  }, [tableContainerRef])

  const rowVirtualizer = useVirtualizer({
    count: enableVirtualization ? table.getRowModel().rows.length : 0,
    getScrollElement: () => tableContainerElementRef.current,
    estimateSize: useCallback(() => estimatedRowHeight, [estimatedRowHeight]),
    overscan,
    enabled: enableVirtualization
  })

  const virtualRows = enableVirtualization
    ? rowVirtualizer.getVirtualItems()
    : []

  const paddingTop =
    enableVirtualization && virtualRows.length > 0 ? virtualRows[0].start : 0
  const paddingBottom =
    enableVirtualization && virtualRows.length > 0
      ? rowVirtualizer.getTotalSize() -
        (virtualRows[virtualRows.length - 1].start +
          virtualRows[virtualRows.length - 1].size)
      : 0

  return (
    <div
      className="overflow-auto"
      style={height ? { height } : undefined}
      ref={tableContainerElementRef}
      onScroll={(e) => fetchMoreOnBottomReached(e.target as HTMLDivElement)}
    >
      <table
        className="w-fit"
        {...{
          style: {
            width: table.getCenterTotalSize()
          }
        }}
      >
        <thead className="dark:bg-sentio-gray-100 sticky top-0 z-[1] bg-white">
          {table.getHeaderGroups().map((headerGroup) => (
            <tr
              key={headerGroup.id}
              className="relative flex w-fit cursor-pointer items-center border-b"
            >
              {headerGroup.headers.map((header, i) => (
                <th
                  key={header.id}
                  colSpan={header.colSpan}
                  style={{
                    width: header.getSize()
                  }}
                  className="text-ilabel group/th blinked dark:hover:!bg-sentio-gray-300 dark:bg-sentio-gray-100 text-text-foreground hover:!bg-primary-50 relative flex items-center whitespace-nowrap bg-white px-2 py-2 text-left font-semibold"
                  onClick={header.column.getToggleSortingHandler()}
                >
                  <span className="flex w-full flex-1 overflow-hidden">
                    <span className="flex-1 truncate">
                      {header.isPlaceholder
                        ? null
                        : flexRender(
                            header.column.columnDef.header,
                            header.getContext()
                          )}
                    </span>
                    {header.column.getCanSort() && allowSort ? (
                      <span
                        className={classNames(
                          header.column.getIsSorted()
                            ? 'hover:text-text-foreground visible hover:bg-gray-200'
                            : 'invisible',
                          'ml-2 flex-none rounded px-1 py-0.5 text-gray-600 group-hover:visible group-focus:visible',
                          'inline-block cursor-pointer',
                          'shrink-0'
                        )}
                      >
                        {header.column.getIsSorted() ? (
                          header.column.getIsSorted() == 'desc' ? (
                            <HiOutlineSortDescending className="h-4 w-4" />
                          ) : (
                            <HiOutlineSortAscending className="h-4 w-4" />
                          )
                        ) : (
                          ''
                        )}
                      </span>
                    ) : null}
                  </span>
                  {allowEditColumn !== false && (
                    <span
                      className="invisible inline-block group-hover/th:visible"
                      onClick={onPreventClick}
                    >
                      <PopupMenuButton
                        buttonClassName="align-text-bottom"
                        onSelect={(commandKey: string) => {
                          const colOrder = headerGroup.headers.map(
                            (item) => (item as any)?.id
                          )
                          switch (commandKey) {
                            case 'reorder.left':
                              table.setColumnOrder(
                                reorder(colOrder, i, i - 1) as ColumnOrderState
                              )
                              break
                            case 'reorder.right':
                              table.setColumnOrder(
                                reorder(colOrder, i, i + 1) as ColumnOrderState
                              )
                              break
                            case 'delete':
                              onColumnRemove?.(header.column.columnDef)
                              break
                            default:
                              console.log(commandKey, 'is not applied')
                          }
                        }}
                        buttonIcon={<HiChevronDown className="icon mr-2" />}
                        items={[
                          [
                            {
                              key: 'reorder.left',
                              label: 'Move column left',
                              icon: <MoveLeftIcon className="mr-2" />,
                              disabled: i === 0
                            },
                            {
                              key: 'reorder.right',
                              label: 'Move column right',
                              icon: <MoveRightIcon className="mr-2" />,
                              disabled: i === headerGroup.headers.length - 1
                            }
                          ],
                          ...(onColumnRename
                            ? [
                                [
                                  {
                                    key: 'rename',
                                    label: 'Rename column',
                                    icon: <RenameIcon className="mr-2" />
                                  }
                                ]
                              ]
                            : []),
                          ...(!onColumnRemove
                            ? []
                            : [
                                [
                                  {
                                    key: 'delete',
                                    label: 'Remove column',
                                    icon: <DeleteIcon className="mr-2" />,
                                    status: 'danger'
                                  }
                                ]
                              ])
                        ]}
                      />
                    </span>
                  )}
                  {header.column.getCanResize() ? (
                    <div
                      onMouseDown={header.getResizeHandler()}
                      onTouchStart={header.getResizeHandler()}
                      className={classNames(
                        `text-md hover:bg-primary-200/50 absolute right-0 top-0 inline-block flex
                          h-full w-2 cursor-col-resize touch-none select-none items-center text-gray-400`
                      )}
                      style={{
                        transform:
                          columnResizeMode === 'onEnd' &&
                          header.column.getIsResizing()
                            ? `translateX(${table.getState().columnSizingInfo.deltaOffset}px)`
                            : ''
                      }}
                      onClick={(e) => e.stopPropagation()}
                    >
                      ⋮
                    </div>
                  ) : null}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody>
          {enableVirtualization && paddingTop > 0 && (
            <tr>
              <td style={{ height: `${paddingTop}px` }} />
            </tr>
          )}

          {enableVirtualization
            ? virtualRows.map((virtualRow) => {
                const row = table.getRowModel().rows[virtualRow.index]
                return (
                  <tr
                    key={row.id}
                    data-index={virtualRow.index}
                    className={classNames(
                      'hover:!bg-primary-50 dark:hover:!bg-sentio-gray-300 group flex w-fit items-center border-b',
                      onClick ? 'cursor-pointer' : '',
                      rowClassNameFn ? rowClassNameFn(row) : ''
                    )}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <td
                        key={cell.id}
                        {...{
                          style: {
                            width: cell.column.getSize()
                          }
                        }}
                        onClick={() => onClick && onClick(row, cell)}
                        className="text-ilabel dark:text-text-foreground-secondary truncate whitespace-nowrap py-2 pl-2 text-gray-600"
                      >
                        {flexRender(
                          cell.column.columnDef.cell,
                          cell.getContext()
                        )}
                      </td>
                    ))}
                  </tr>
                )
              })
            : table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className={classNames(
                    'hover:!bg-primary-50 dark:hover:!bg-sentio-gray-300 blinked group flex w-fit items-center border-b',
                    onClick ? 'cursor-pointer' : '',
                    rowClassNameFn ? rowClassNameFn(row) : ''
                  )}
                >
                  {row.getVisibleCells().map((cell) => (
                    <td
                      key={cell.id}
                      {...{
                        style: {
                          width: cell.column.getSize()
                        }
                      }}
                      onClick={() => onClick && onClick(row, cell)}
                      className="text-ilabel dark:text-text-foreground-secondary truncate whitespace-nowrap py-2 pl-2 text-gray-600"
                    >
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext()
                      )}
                    </td>
                  ))}
                </tr>
              ))}

          {enableVirtualization && paddingBottom > 0 && (
            <tr>
              <td style={{ height: `${paddingBottom}px` }} />
            </tr>
          )}

          {onFetchMore && (
            <tr>
              <td
                colSpan={table.getHeaderGroups()[0].headers.length}
                className="text-ilabel hover:bg-primary-50 cursor-pointer py-2 text-center text-gray-600"
                onClick={() => {
                  if (isFetching) return
                  onFetchMore?.()
                }}
              >
                {isFetching || hasMore ? (
                  <span className="inline-flex items-center gap-2">
                    <IoReload
                      className={classNames(
                        'h-4 w-4',
                        isFetching ? 'animate-spin' : ''
                      )}
                    />
                    <span>Loading...</span>
                  </span>
                ) : (
                  'No more data'
                )}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
})

export const ResizeTable = memo(_ResizeTable)

export default ResizeTable
