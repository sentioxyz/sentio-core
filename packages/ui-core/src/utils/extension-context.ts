import { createContext, useContext } from 'react'
export { useDarkMode } from './use-dark-mode'

export const SvgFolderContext = createContext('')
export const useDetectExtenstion = () => {
  const folderPath = useContext(SvgFolderContext)
  return Boolean(folderPath)
}
