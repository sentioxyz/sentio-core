import { Disclosure } from '@headlessui/react'
import { ChevronRightIcon, DocumentTextIcon } from '@heroicons/react/24/outline'
import { classNames } from '@sentio/ui-core'
import { useEffect, useState } from 'react'

export interface TreeNode {
  text: string
  path: string
  children: TreeNode[]
}

interface Props {
  node: TreeNode
  selectedPath: string
  onSelect: (path: string) => void
  depth?: number
  renderItem?: (
    item: TreeNode,
    depth?: number,
    selectedPath?: string
  ) => JSX.Element
}

function defaultRenderItem(
  item: TreeNode,
  depth?: number,
  selectedPath?: string
) {
  const isFile = !item.children.length
  return (
    <div
      className={classNames(
        'flex cursor-pointer items-center gap-1 rounded',
        isFile && 'mb-2 pr-2',
        selectedPath === item.path
          ? 'bg-[#DBE7F980] dark:bg-gray-100'
          : 'hover:bg-gray-50'
      )}
      style={{ paddingLeft: `${depth}rem` }}
      title={item.text}
    >
      {isFile && <DocumentTextIcon className="h-4 w-4 shrink-0" />}
      <span className="truncate">{item.text}</span>
    </div>
  )
}

function SourceTreeItem({
  node: item,
  selectedPath,
  onSelect,
  depth = 0,
  renderItem = defaultRenderItem
}: Props) {
  const [open, setOpen] = useState(false)

  useEffect(() => {
    if (selectedPath.startsWith(item.path)) {
      setOpen(true)
    }
  }, [selectedPath])

  return (
    <div key={item.path}>
      <>
        <button
          className="mb-2 flex w-full gap-1 rounded px-1 hover:bg-gray-50"
          style={{ paddingLeft: `${depth}rem` }}
          onClick={() => setOpen(!open)}
        >
          <ChevronRightIcon
            className={`${open ? 'rotate-90 transform' : ''} text-gray h-3 w-3 shrink-0 self-center`}
          />
          {renderItem(item)}
        </button>
        {open && (
          <SourceTree
            node={item}
            selectedPath={selectedPath}
            onSelect={onSelect}
            depth={depth + 1}
            renderItem={renderItem}
          />
        )}
      </>
    </div>
  )
}

export function SourceTree({ node, ...props }: Props) {
  const {
    selectedPath,
    onSelect,
    depth = 0,
    renderItem = defaultRenderItem
  } = props
  return (
    <>
      {node.children.map((item, index) => {
        const isFile = !item.children.length
        if (isFile) {
          return (
            <div key={item.path} onClick={() => onSelect(item.path)}>
              {renderItem(item, depth, selectedPath)}
            </div>
          )
        }

        return <SourceTreeItem node={item} {...props} key={item.path} />
      })}
    </>
  )
}
