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
      <button className="rounded bg-blue-500 px-4 py-2 text-white hover:bg-blue-600">
        Copy Me
      </button>
    </CopyButton>
  </div>
)
