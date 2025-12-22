import '../../styles.css'
import { FlatTree, DataNode } from './FlatTree'
import { useState } from 'react'
import { LuChevronRight, LuChevronDown, LuFolder, LuFile } from 'react-icons/lu'

// Sample tree data
const createSampleData = (): DataNode[] => [
  {
    key: 'node-1',
    title: 'Parent Node 1',
    children: [
      {
        key: 'node-1-1',
        title: 'Child Node 1-1',
        children: [
          {
            key: 'node-1-1-1',
            title: 'Leaf Node 1-1-1'
          },
          {
            key: 'node-1-1-2',
            title: 'Leaf Node 1-1-2'
          }
        ]
      },
      {
        key: 'node-1-2',
        title: 'Child Node 1-2'
      }
    ]
  },
  {
    key: 'node-2',
    title: 'Parent Node 2',
    children: [
      {
        key: 'node-2-1',
        title: 'Child Node 2-1'
      },
      {
        key: 'node-2-2',
        title: 'Child Node 2-2',
        children: [
          {
            key: 'node-2-2-1',
            title: 'Leaf Node 2-2-1'
          }
        ]
      }
    ]
  },
  {
    key: 'node-3',
    title: 'Parent Node 3',
    children: [
      {
        key: 'node-3-1',
        title: 'Child Node 3-1'
      }
    ]
  }
]

const createFileSystemData = (): DataNode[] => [
  {
    key: 'root',
    title: (
      <div className="flex items-center gap-2">
        <LuFolder className="text-yellow-500" />
        <span>src</span>
      </div>
    ),
    children: [
      {
        key: 'components',
        title: (
          <div className="flex items-center gap-2">
            <LuFolder className="text-yellow-500" />
            <span>components</span>
          </div>
        ),
        children: [
          {
            key: 'button.tsx',
            title: (
              <div className="flex items-center gap-2">
                <LuFile className="text-blue-500" />
                <span>Button.tsx</span>
              </div>
            )
          },
          {
            key: 'input.tsx',
            title: (
              <div className="flex items-center gap-2">
                <LuFile className="text-blue-500" />
                <span>Input.tsx</span>
              </div>
            )
          }
        ]
      },
      {
        key: 'utils',
        title: (
          <div className="flex items-center gap-2">
            <LuFolder className="text-yellow-500" />
            <span>utils</span>
          </div>
        ),
        children: [
          {
            key: 'helpers.ts',
            title: (
              <div className="flex items-center gap-2">
                <LuFile className="text-gray-500" />
                <span>helpers.ts</span>
              </div>
            )
          }
        ]
      },
      {
        key: 'index.ts',
        title: (
          <div className="flex items-center gap-2">
            <LuFile className="text-gray-500" />
            <span>index.ts</span>
          </div>
        )
      }
    ]
  }
]

const createLargeData = (count: number = 100): DataNode[] => {
  const data: DataNode[] = []
  for (let i = 0; i < count; i++) {
    data.push({
      key: `parent-${i}`,
      title: `Parent Node ${i}`,
      children: [
        {
          key: `parent-${i}-child-1`,
          title: `Child ${i}-1`
        },
        {
          key: `parent-${i}-child-2`,
          title: `Child ${i}-2`
        },
        {
          key: `parent-${i}-child-3`,
          title: `Child ${i}-3`
        }
      ]
    })
  }
  return data
}

// Basic tree example
export const Default = () => {
  return (
    <div style={{ padding: 16 }}>
      <FlatTree data={createSampleData()} />
    </div>
  )
}

// Tree with all nodes expanded by default
export const DefaultExpandAll = () => {
  return (
    <div style={{ padding: 16 }}>
      <FlatTree data={createSampleData()} defaultExpandAll />
    </div>
  )
}

// Tree with click handler
export const WithClickHandler = () => {
  const [selectedNode, setSelectedNode] = useState<DataNode | null>(null)

  return (
    <div style={{ padding: 16 }}>
      <div className="mb-4">
        <p className="text-sm text-gray-600">
          Selected Node: <strong>{selectedNode?.key || 'None'}</strong>
        </p>
      </div>
      <FlatTree
        data={createSampleData()}
        onClick={(node) => {
          console.log('Clicked:', node)
          setSelectedNode(node)
        }}
      />
    </div>
  )
}

// Virtualized tree for large datasets
export const VirtualizedTree = () => {
  return (
    <div style={{ padding: 16 }}>
      <p className="mb-4 text-sm text-gray-600">
        Virtualized tree with 100 parent nodes (300+ total nodes)
      </p>
      <FlatTree
        data={createLargeData(100)}
        virtual
        height={400}
        rowHeight={35}
      />
    </div>
  )
}

// Tree with custom row heights
export const CustomRowHeight = () => {
  return (
    <div style={{ padding: 16 }}>
      <FlatTree
        data={createSampleData()}
        virtual
        height={400}
        rowHeight={(index, isSelected) => (isSelected ? 60 : 40)}
        defaultExpandAll
      />
    </div>
  )
}

// Tree with custom icons
export const CustomIcons = () => {
  return (
    <div style={{ padding: 16 }}>
      <FlatTree
        data={createFileSystemData()}
        expandIcon={<LuChevronRight className="h-4 w-4" />}
        collapseIcon={<LuChevronDown className="h-4 w-4" />}
        defaultExpandAll
      />
    </div>
  )
}

// Tree with suffix node
export const WithSuffixNode = () => {
  return (
    <div style={{ padding: 16 }}>
      <FlatTree
        data={createSampleData()}
        suffixNode={
          <div className="rounded border border-blue-200 bg-blue-50 p-4">
            <p className="text-sm text-blue-800">
              This is a suffix node that appears after the selected item
            </p>
          </div>
        }
        onClick={(node) => console.log('Selected:', node)}
      />
    </div>
  )
}

// Tree with expand depth control
export const WithExpandDepth = () => {
  const [expandDepth, setExpandDepth] = useState(1)

  return (
    <div style={{ padding: 16 }}>
      <div className="mb-4 flex items-center gap-4">
        <label className="text-sm text-gray-600">Expand Depth:</label>
        <select
          value={expandDepth}
          onChange={(e) => setExpandDepth(Number(e.target.value))}
          className="rounded border px-2 py-1"
        >
          <option value={0}>0</option>
          <option value={1}>1</option>
          <option value={2}>2</option>
          <option value={3}>3</option>
        </select>
      </div>
      <FlatTree data={createSampleData()} expandDepth={expandDepth} />
    </div>
  )
}

// Tree with custom content className
export const CustomContentStyle = () => {
  return (
    <div style={{ padding: 16 }}>
      <FlatTree
        data={createSampleData()}
        contentClassName="hover:bg-blue-50 transition-colors"
        defaultExpandAll
      />
    </div>
  )
}

// Tree with scroll to key
export const ScrollToKey = () => {
  const [scrollToKey, setScrollToKey] = useState<string>('')

  return (
    <div style={{ padding: 16 }}>
      <div className="mb-4 flex items-center gap-4">
        <label className="text-sm text-gray-600">Scroll to key:</label>
        <input
          type="text"
          value={scrollToKey}
          onChange={(e) => setScrollToKey(e.target.value)}
          placeholder="e.g., node-2-2-1"
          className="rounded border px-2 py-1"
        />
      </div>
      <FlatTree
        data={createSampleData()}
        virtual
        height={300}
        scrollToKey={scrollToKey}
        defaultExpandAll
      />
    </div>
  )
}

// File system tree example
export const FileSystemTree = () => {
  return (
    <div style={{ padding: 16 }}>
      <div className="rounded-lg border bg-gray-50 p-4">
        <h3 className="mb-4 text-lg font-semibold">File Explorer</h3>
        <FlatTree
          data={createFileSystemData()}
          expandIcon={<LuChevronRight className="h-4 w-4" />}
          collapseIcon={<LuChevronDown className="h-4 w-4" />}
          onClick={(node) => {
            if (!node.children || node.children.length === 0) {
              console.log('Open file:', node.key)
            }
          }}
        />
      </div>
    </div>
  )
}

// Tree with dynamic title rendering
export const DynamicTitleRendering = () => {
  return (
    <div style={{ padding: 16 }}>
      <FlatTree
        data={[
          {
            key: 'dynamic-1',
            title: (data) => (
              <div className="flex w-full items-center justify-between">
                <span>{data.key}</span>
                <span className="ml-4 text-xs text-gray-500">
                  Depth: {data.depth}
                </span>
              </div>
            ),
            children: [
              {
                key: 'dynamic-1-1',
                title: (data) => (
                  <div className="flex w-full items-center justify-between">
                    <span>{data.key}</span>
                    <span className="ml-4 text-xs text-gray-500">
                      Depth: {data.depth}
                    </span>
                  </div>
                )
              }
            ]
          }
        ]}
        defaultExpandAll
      />
    </div>
  )
}
