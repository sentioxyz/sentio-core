import { ErrorBoundary } from 'react-error-boundary'

const errorHint = (
  <div
    style={{
      fontSize: '13px',
      textAlign: 'center',
      color: '#757575'
    }}
  >
    Something went wrong
  </div>
)

export const ErrorBoundaryWrapper = ({
  children,
  fallback = errorHint
}: {
  children: React.ReactNode
  fallback?: React.ReactElement
}) => {
  return (
    <ErrorBoundary
      fallback={fallback}
      onError={(error: Error, info: { componentStack: string }) => {
        // ignore
      }}
    >
      {children}
    </ErrorBoundary>
  )
}
