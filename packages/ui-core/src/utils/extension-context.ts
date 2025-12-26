import { createContext, useContext } from 'react'

export const SvgFolderContext = createContext('')
export const useDetectExtenstion = () => {
  const folderPath = useContext(SvgFolderContext)
  return Boolean(folderPath)
}

export const DarkModeContext = createContext(false)
export const useDarkMode = () => {
  return useContext(DarkModeContext)
}
