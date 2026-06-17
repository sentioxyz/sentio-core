import { createContext, useContext } from 'react'
import { Button as NewButton, type ButtonProps } from '@sentio/ui-core'
import { IoMdRefresh } from 'react-icons/io'

export const RefreshContext = createContext<{
  refresh?: () => void
  isRefreshing?: boolean
}>({})

export const RefreshButton = (props: Partial<ButtonProps>) => {
  const { refresh, isRefreshing } = useContext(RefreshContext)
  if (!refresh) return null
  return (
    <div className="grid items-center justify-items-center">
      <NewButton
        size="sm"
        role="text"
        onClick={(evt) => {
          evt.preventDefault()
          refresh()
        }}
        processing={isRefreshing}
        icon={<IoMdRefresh />}
        className="text-text-foreground-secondary!"
        {...props}
      >
        Retry
      </NewButton>
    </div>
  )
}
