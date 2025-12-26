import '../../styles.css'
import { ResizeTable } from './ResizeTable'
import { ColumnDef } from '@tanstack/react-table'
import { useEffect, useState } from 'react'

// Sample data types
type Person = {
  id: number
  firstName: string
  lastName: string
  age: number
  email: string
  status: 'active' | 'inactive' | 'pending'
}

type Product = {
  id: number
  name: string
  category: string
  price: number
  stock: number
  rating: number
}

// Sample data generators
const createPeopleData = (count: number = 20): Person[] => {
  const firstNames = [
    'John',
    'Jane',
    'Bob',
    'Alice',
    'Charlie',
    'Emma',
    'David',
    'Sarah',
    'Michael',
    'Lisa'
  ]
  const lastNames = [
    'Smith',
    'Johnson',
    'Williams',
    'Brown',
    'Jones',
    'Garcia',
    'Miller',
    'Davis',
    'Rodriguez',
    'Martinez'
  ]
  const statuses: Person['status'][] = ['active', 'inactive', 'pending']

  return Array.from({ length: count }, (_, i) => ({
    id: i + 1,
    firstName: firstNames[i % firstNames.length],
    lastName: lastNames[i % lastNames.length],
    age: 20 + (i % 50),
    email: `user${i + 1}@example.com`,
    status: statuses[i % statuses.length]
  }))
}

const createProductData = (count: number = 50): Product[] => {
  const products = [
    'Laptop',
    'Phone',
    'Tablet',
    'Monitor',
    'Keyboard',
    'Mouse',
    'Headphones',
    'Camera',
    'Printer',
    'Speaker'
  ]
  const categories = ['Electronics', 'Accessories', 'Audio', 'Computing']

  return Array.from({ length: count }, (_, i) => ({
    id: i + 1,
    name: `${products[i % products.length]} ${i + 1}`,
    category: categories[i % categories.length],
    price: Math.round((Math.random() * 1000 + 100) * 100) / 100,
    stock: Math.floor(Math.random() * 100),
    rating: Math.round((Math.random() * 2 + 3) * 10) / 10
  }))
}

// Column definitions
const peopleColumns: ColumnDef<Person>[] = [
  {
    accessorKey: 'id',
    header: 'ID',
    size: 80,
    minSize: 60
  },
  {
    accessorKey: 'firstName',
    header: 'First Name',
    size: 150,
    minSize: 100
  },
  {
    accessorKey: 'lastName',
    header: 'Last Name',
    size: 150,
    minSize: 100
  },
  {
    accessorKey: 'age',
    header: 'Age',
    size: 80,
    minSize: 60
  },
  {
    accessorKey: 'email',
    header: 'Email',
    size: 250,
    minSize: 150
  },
  {
    accessorKey: 'status',
    header: 'Status',
    size: 120,
    minSize: 80,
    cell: ({ getValue }) => {
      const status = getValue() as Person['status']
      const colors = {
        active: 'bg-green-100 text-green-800',
        inactive: 'bg-gray-100 text-gray-800',
        pending: 'bg-yellow-100 text-yellow-800'
      }
      return (
        <span
          className={`rounded px-2 py-1 text-xs font-medium ${colors[status]}`}
        >
          {status}
        </span>
      )
    }
  }
]

const productColumns: ColumnDef<Product>[] = [
  {
    accessorKey: 'id',
    header: 'ID',
    size: 70,
    minSize: 50
  },
  {
    accessorKey: 'name',
    header: 'Product Name',
    size: 200,
    minSize: 150
  },
  {
    accessorKey: 'category',
    header: 'Category',
    size: 150,
    minSize: 100
  },
  {
    accessorKey: 'price',
    header: 'Price',
    size: 100,
    minSize: 80,
    cell: ({ getValue }) => `$${(getValue() as number).toFixed(2)}`
  },
  {
    accessorKey: 'stock',
    header: 'Stock',
    size: 100,
    minSize: 80
  },
  {
    accessorKey: 'rating',
    header: 'Rating',
    size: 100,
    minSize: 80,
    cell: ({ getValue }) => `⭐ ${getValue()}`
  }
]

// Basic table example
export const Default = () => {
  return (
    <div style={{ padding: 16 }}>
      <ResizeTable
        data={createPeopleData(20)}
        columns={peopleColumns}
        columnResizeMode="onChange"
      />
    </div>
  )
}

// Table with sorting enabled
export const WithSorting = () => {
  return (
    <div style={{ padding: 16 }}>
      <p className="mb-4 text-sm text-gray-600">Click column headers to sort</p>
      <ResizeTable
        data={createPeopleData(20)}
        columns={peopleColumns}
        columnResizeMode="onChange"
        allowSort
      />
    </div>
  )
}

// Table with column resizing
export const WithColumnResize = () => {
  return (
    <div style={{ padding: 16 }}>
      <p className="mb-4 text-sm text-gray-600">
        Drag column dividers to resize
      </p>
      <ResizeTable
        data={createPeopleData(20)}
        columns={peopleColumns}
        columnResizeMode="onChange"
        allowResizeColumn
      />
    </div>
  )
}

// Table with column editing (reorder, remove)
export const WithColumnEditing = () => {
  const [columns, setColumns] = useState(peopleColumns)

  return (
    <div style={{ padding: 16 }}>
      <p className="mb-4 text-sm text-gray-600">
        Click the dropdown icon in column headers to reorder or remove columns
      </p>
      <ResizeTable
        data={createPeopleData(20)}
        columns={columns}
        columnResizeMode="onChange"
        allowEditColumn
        onColumnRemove={(col) => {
          setColumns((prev) => prev.filter((c) => c !== col))
        }}
      />
    </div>
  )
}

// Table with fixed height and scrolling
export const FixedHeight = () => {
  return (
    <div style={{ padding: 16 }}>
      <p className="mb-4 text-sm text-gray-600">
        Table with fixed height (400px)
      </p>
      <ResizeTable
        data={createPeopleData(50)}
        columns={peopleColumns}
        columnResizeMode="onChange"
        height={400}
      />
    </div>
  )
}

// Table with row click handler
export const WithRowClick = () => {
  const [selectedRow, setSelectedRow] = useState<Person | null>(null)

  return (
    <div style={{ padding: 16 }}>
      <div className="mb-4 rounded border border-blue-200 bg-blue-50 p-4">
        <p className="text-sm text-blue-800">
          Selected:{' '}
          {selectedRow
            ? `${selectedRow.firstName} ${selectedRow.lastName}`
            : 'None'}
        </p>
      </div>
      <ResizeTable
        data={createPeopleData(20)}
        columns={peopleColumns}
        columnResizeMode="onChange"
        onClick={(row) => {
          setSelectedRow(row.original)
        }}
      />
    </div>
  )
}

// Table with virtualization for large datasets
export const VirtualizedTable = () => {
  return (
    <div style={{ padding: 16 }}>
      <p className="mb-4 text-sm text-gray-600">
        Virtualized table with 1000 rows for optimal performance
      </p>
      <ResizeTable
        data={createPeopleData(1000)}
        columns={peopleColumns}
        columnResizeMode="onChange"
        height={500}
        enableVirtualization
        estimatedRowHeight={35}
        overscan={10}
      />
    </div>
  )
}

// Table with infinite scroll / load more
export const InfiniteScroll = () => {
  const [data, setData] = useState(createProductData(30))
  const [isFetching, setIsFetching] = useState(false)
  const [hasMore, setHasMore] = useState(true)

  const handleFetchMore = () => {
    setIsFetching(true)

    // Simulate API call
    setTimeout(() => {
      const currentLength = data.length
      const newData = createProductData(20).map((item, i) => ({
        ...item,
        id: currentLength + i + 1
      }))

      setData((prev) => [...prev, ...newData])
      setIsFetching(false)

      // Stop loading after 200 items
      if (currentLength + 20 >= 200) {
        setHasMore(false)
      }
    }, 1000)
  }

  return (
    <div style={{ padding: 16 }}>
      <p className="mb-4 text-sm text-gray-600">
        Scroll to bottom to load more items. Currently loaded: {data.length}
      </p>
      <ResizeTable
        data={data}
        columns={productColumns}
        columnResizeMode="onChange"
        height={400}
        onFetchMore={handleFetchMore}
        hasMore={hasMore}
        isFetching={isFetching}
      />
    </div>
  )
}

// Table with custom row styling
export const CustomRowStyling = () => {
  return (
    <div style={{ padding: 16 }}>
      <p className="mb-4 text-sm text-gray-600">
        Rows with 'active' status have green background
      </p>
      <ResizeTable
        data={createPeopleData(20)}
        columns={peopleColumns}
        columnResizeMode="onChange"
        rowClassNameFn={(row) => {
          const person = row.original as Person
          return person.status === 'active' ? 'bg-green-50' : ''
        }}
      />
    </div>
  )
}

// Table with all features combined
export const AllFeatures = () => {
  const [columns, setColumns] = useState(productColumns)
  const [selectedRow, setSelectedRow] = useState<Product | null>(null)
  const [data, setData] = useState<any[]>([])
  useEffect(() => {
    setData(createProductData(500))
  }, [])

  return (
    <div style={{ padding: 16 }}>
      <div className="mb-4 rounded border border-blue-200 bg-blue-50 p-4">
        <p className="text-sm font-semibold text-blue-900">Selected Product</p>
        <p className="text-sm text-blue-800">
          {selectedRow
            ? `${selectedRow.name} - $${selectedRow.price}`
            : 'Click a row to select'}
        </p>
      </div>
      <p className="mb-4 text-sm text-gray-600">
        Full featured table: sorting, resizing, column editing, row selection,
        virtualization
      </p>
      <ResizeTable
        data={data}
        columns={columns}
        columnResizeMode="onChange"
        height={500}
        allowSort
        allowResizeColumn
        allowEditColumn
        enableVirtualization
        onClick={(row) => {
          setSelectedRow(row.original)
        }}
        onColumnRemove={(col) => {
          setColumns((prev) => prev.filter((c) => c !== col))
        }}
        rowClassNameFn={(row) => {
          const product = row.original as Product
          return product === selectedRow ? 'bg-blue-100' : ''
        }}
      />
    </div>
  )
}

// Table with onEnd resize mode
export const ResizeOnEnd = () => {
  return (
    <div style={{ padding: 16 }}>
      <p className="mb-4 text-sm text-gray-600">
        Column resize applies when mouse is released (onEnd mode)
      </p>
      <ResizeTable
        data={createPeopleData(20)}
        columns={peopleColumns}
        columnResizeMode="onEnd"
        allowResizeColumn
      />
    </div>
  )
}

// Table with minimum width constraint
export const WithMinWidth = () => {
  return (
    <div style={{ padding: 16 }}>
      <p className="mb-4 text-sm text-gray-600">
        Table with minimum width of 1200px (columns are scaled to fit)
      </p>
      <ResizeTable
        data={createPeopleData(20)}
        columns={peopleColumns}
        columnResizeMode="onChange"
        minWidth={1200}
      />
    </div>
  )
}

// Compact table with smaller rows
export const CompactTable = () => {
  const compactColumns: ColumnDef<Person>[] = [
    {
      accessorKey: 'id',
      header: 'ID',
      size: 60,
      minSize: 50
    },
    {
      accessorKey: 'firstName',
      header: 'Name',
      size: 100,
      minSize: 80
    },
    {
      accessorKey: 'email',
      header: 'Email',
      size: 200,
      minSize: 150
    },
    {
      accessorKey: 'status',
      header: 'Status',
      size: 80,
      minSize: 60
    }
  ]

  return (
    <div style={{ padding: 16 }}>
      <p className="mb-4 text-sm text-gray-600">
        Compact table with fewer columns
      </p>
      <ResizeTable
        data={createPeopleData(30)}
        columns={compactColumns}
        columnResizeMode="onChange"
        height={400}
        minSize={50}
      />
    </div>
  )
}
