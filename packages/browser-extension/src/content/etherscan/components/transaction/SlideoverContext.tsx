import { createContext, useContext, useState, ReactNode } from 'react'

interface SlideoverContextType {
  sig: string
  setSig: (sig: string) => void
  contract: string
  setContract: (contract: string) => void
  visible: boolean
  openSlideOver: (visible: boolean) => void
}

const SlideoverContext = createContext<SlideoverContextType | undefined>(
  undefined
)

export function SlideoverProvider({ children }: { children: ReactNode }) {
  const [sig, setSig] = useState<string>('')
  const [contract, setContract] = useState<string>('')
  const [visible, setVisible] = useState<boolean>(false)

  const value = {
    sig,
    setSig,
    contract,
    setContract,
    visible,
    openSlideOver: setVisible
  }

  return (
    <SlideoverContext.Provider value={value}>
      {children}
    </SlideoverContext.Provider>
  )
}

export function useSlideoverContext() {
  const context = useContext(SlideoverContext)
  if (context === undefined) {
    throw new Error(
      'useSlideoverContext must be used within a SlideoverProvider'
    )
  }
  return context
}
