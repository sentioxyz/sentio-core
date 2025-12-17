import '../styles.css'
import { BarLoading } from './BarLoading'

// Ladle works by rendering exported React components from *.stories.* files.
// Export simple React components (no Storybook APIs) so Ladle can pick them up.

export const Default = () => (
  <div style={{ height: 200 }}>
    <BarLoading />
  </div>
)

export const CustomHint = () => (
  <div style={{ height: 200 }}>
    <BarLoading hint="Loading your data..." />
  </div>
)

export const CustomWidth = () => (
  <div style={{ height: 200 }}>
    <BarLoading width={300} hint="Wide loading bar" />
  </div>
)

export const GrayStyle = () => (
  <div style={{ height: 200 }}>
    <BarLoading gray={true} hint="Gray style loading" />
  </div>
)
