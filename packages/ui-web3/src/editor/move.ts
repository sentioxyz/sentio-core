import { Monaco } from '@monaco-editor/react'
import { moveLanguageConfig, moveTokenProvider } from './MoveLanguage'

let registerred = false

export const registerSentioMove = (monaco?: Monaco) => {
  if (!monaco || registerred) {
    return
  }

  monaco.languages.register({ id: 'sentio-move', extensions: ['.move'] })
  monaco.languages.setMonarchTokensProvider(
    'sentio-move',
    moveTokenProvider as any
  )
  monaco.languages.setLanguageConfiguration(
    'sentio-move',
    moveLanguageConfig as any
  )

  registerred = true
}
