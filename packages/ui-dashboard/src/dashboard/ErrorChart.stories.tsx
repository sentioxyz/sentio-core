import type { Story } from '@ladle/react'
import { ErrorChart } from './ErrorChart'

// Inline gray logo — no external file dependency
const grayLogoSrc =
  'data:image/svg+xml;base64,' +
  btoa(
    '<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">' +
      '<circle cx="32" cy="32" r="28" fill="#9ca3af"/>' +
      '<text x="32" y="38" text-anchor="middle" font-size="22" font-family="sans-serif" fill="#fff">S</text>' +
      '</svg>'
  )

export const NotFound: Story = () => (
  <div className="h-64 w-full">
    <ErrorChart
      data={{ code: 5 }}
      onNavigateToDatasource={() => alert('navigate')}
    />
  </div>
)

export const RateLimited: Story = () => (
  <div className="relative h-64 w-full">
    <ErrorChart data={{ status: 429 }} />
  </div>
)

export const GenericError: Story = () => (
  <div className="h-64 w-full">
    <ErrorChart
      data={{ message: 'Query execution failed: timeout after 30s' }}
    />
  </div>
)

export const WithLogo: Story = () => (
  <div className="h-64 w-full">
    <ErrorChart
      data={{ message: 'Something went wrong' }}
      logoSrc={grayLogoSrc}
    />
  </div>
)
