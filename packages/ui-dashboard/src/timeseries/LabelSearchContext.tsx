import React, { createContext, useContext, useState, ReactNode } from 'react'

interface LabelSearchContextType {
  labelSearchQuery: string
  setLabelSearchQuery: (query: string) => void
}

const LabelSearchContext = createContext<LabelSearchContextType | undefined>(
  undefined
)

interface LabelSearchProviderProps {
  children: ReactNode
}

export function LabelSearchProvider({ children }: LabelSearchProviderProps) {
  const [labelSearchQuery, setLabelSearchQuery] = useState('')

  return (
    <LabelSearchContext.Provider
      value={{ labelSearchQuery, setLabelSearchQuery }}
    >
      {children}
    </LabelSearchContext.Provider>
  )
}

export function useLabelSearchContext(): LabelSearchContextType | undefined {
  return useContext(LabelSearchContext)
}

export function useLabelSearch(defaultQuery?: string): {
  labelSearchQuery: string
  setLabelSearchQuery: (query: string) => void
} {
  const context = useLabelSearchContext()
  const [localQuery, setLocalQuery] = useState(defaultQuery || '')

  if (context) {
    return context
  }

  return {
    labelSearchQuery: localQuery,
    setLabelSearchQuery: setLocalQuery
  }
}
