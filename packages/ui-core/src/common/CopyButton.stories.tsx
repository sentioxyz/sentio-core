import '../styles.css'
import { CopyButton } from './CopyButton'

export const Default = () => (
  <div className="p-4">
    <CopyButton text="Hello, World!" size={16} />
  </div>
)

export const WithCustomChildren = () => (
  <div className="p-4">
    <CopyButton text="Custom button text" size={24}>
      <button className="rounded-sm bg-blue-500 px-4 py-2 text-white hover:bg-blue-600">
        Copy Me
      </button>
    </CopyButton>
  </div>
)

export const WithAsyncText = () => (
  <div className="p-4 space-y-4">
    <p className="text-sm text-gray-500">
      Async Text Example
    </p>
    <CopyButton
      text={async () => {
        await new Promise((resolve) => setTimeout(resolve, 1000))
        return `Async content fetched at ${new Date().toISOString()}`
      }}
      size={16}
    />
  </div>
)
