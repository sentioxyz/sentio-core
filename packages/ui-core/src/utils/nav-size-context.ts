import { createContext } from 'react'

export const NavSizeContext = createContext({
  small: true,
  showLabel: true,
  setSmall: (small: boolean) => {
    // do nothing
  },
  setShowLabel: (showLabel: boolean) => {
    // do nothing
  }
})
