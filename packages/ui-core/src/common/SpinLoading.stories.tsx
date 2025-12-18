import '../styles.css'
import { SpinLoading } from './SpinLoading'

// Ladle works by rendering exported React components from *.stories.* files.
// Export simple React components (no Storybook APIs) so Ladle can pick them up.

export const Default = () => (
  <div style={{ height: 200, position: 'relative' }}>
    <SpinLoading loading={true} />
  </div>
)

export const WithMask = () => (
  <div style={{ height: 300, position: 'relative' }}>
    <SpinLoading loading={true} showMask={true}>
      <div className="p-8 bg-blue-50 dark:bg-blue-900/20">
        <h3 className="text-lg font-bold mb-4">Content with Mask</h3>
        <p className="text-gray-600 dark:text-gray-400">
          The mask overlay makes the content less visible while loading.
        </p>
      </div>
    </SpinLoading>
  </div>
)

export const CustomSize = () => (
  <div style={{ height: 200, position: 'relative' }}>
    <SpinLoading loading={true} size={80} />
  </div>
)
