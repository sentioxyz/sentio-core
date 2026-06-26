import type { Story } from '@ladle/react'
import { ErrorChart } from './ErrorChart'

export const NotFound: Story = () => (
  <div className="h-64 w-full">
    <ErrorChart
      data={{ code: 5 }}
      onNavigateToDatasource={() => alert('navigate')}
    />
  </div>
)

export const RateLimited: Story = () => (
  <div className="h-64 w-full">
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
      logoSrc="/gray-logo.png"
    />
  </div>
)
