import { createContext, useContext, ReactNode } from 'react'
import { DecodedCallTrace } from '@sentio/debugger-common'

interface SubFundflowContextType {
  open: (data: DecodedCallTrace) => void
  close: () => void
}

const SubFundflowContext = createContext<SubFundflowContextType | undefined>(
  undefined
)

export const useSubFundflow = () => {
  const context = useContext(SubFundflowContext)
  if (!context) {
    return {
      open: () => {},
      close: () => {}
    }
  }
  return context
}

interface SubFundflowProviderProps {
  children: ReactNode
  transaction?: any
}

export function SubFundflowProvider({
  children,
  transaction
}: SubFundflowProviderProps) {
  return <>{children}</>
}
