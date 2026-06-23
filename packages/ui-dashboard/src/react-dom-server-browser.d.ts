// @types/react-dom does not declare the `server.browser` subpath. We use it so
// the tooltip renderToString runs in the browser without the Node build's
// `global` reference (see charts/TimeSeriesChart.tsx).
declare module 'react-dom/server.browser' {
  import type { ReactNode } from 'react'
  export function renderToString(element: ReactNode): string
}
