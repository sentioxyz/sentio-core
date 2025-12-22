import {
  solidityLanguageConfig,
  solidityTokensProvider
} from './SolidityLanguage'
import { SentioDefinitionProvider } from './SentioDefinitionProvider'
import { SentioDocumentSymbolProvider } from './SentioDocumentSymbolProvider'
import { SentioHoverProvider } from './SentioHoverProvider'
import { SentioReferenceProvider } from './SentioReferenceProvider'
import { Monaco } from '@monaco-editor/react'
import { SourceStore } from './SourceStore'
import { parseMonacoUriFn } from 'lib/debug/types'
import { trackEvent } from 'lib/tracking'

let registerred = false

export const setSolidityLanguage = (monaco?: Monaco) => {
  if (!monaco || registerred) {
    return
  }

  monaco.languages.register({ id: 'sentio-solidity', extensions: ['.sol'] })
  monaco.languages.setMonarchTokensProvider(
    'sentio-solidity',
    solidityTokensProvider as any
  )
  monaco.languages.setLanguageConfiguration(
    'sentio-solidity',
    solidityLanguageConfig as any
  )

  registerred = true
}

export const setSolidityProviders = (
  monaco?: Monaco,
  store?: SourceStore,
  parseMonacoUri?: parseMonacoUriFn,
  onHover?: (
    model: monaco.editor.ITextModel,
    position: monaco.Position,
    token: monaco.CancellationToken
  ) => void
): monaco.IDisposable[] => {
  if (!monaco || !store) {
    return []
  }
  return [
    monaco.languages.registerDefinitionProvider(
      'sentio-solidity',
      new SentioDefinitionProvider({ store, parseMonacoUri }, monaco)
    ),
    monaco.languages.registerDocumentSymbolProvider(
      'sentio-solidity',
      new SentioDocumentSymbolProvider({ store, parseMonacoUri }, monaco)
    ),
    monaco.languages.registerReferenceProvider(
      'sentio-solidity',
      new SentioReferenceProvider({ store, parseMonacoUri }, monaco)
    ),
    monaco.languages.registerHoverProvider(
      'sentio-solidity',
      new SentioHoverProvider({ store, parseMonacoUri, onHover }, monaco)
    )
  ]
}

type txnSearch = (contract: string, methodSig: string) => void

export const openCodeEditor = (
  monaco: Monaco,
  sourceEditor: monaco.editor.ICodeEditor,
  resource: monaco.Uri,
  selectionOrPosition?: monaco.IPosition | monaco.IRange,
  onSearchTxn?: txnSearch,
  onModelChange?: (uri: monaco.Uri, line?: number) => void,
  editorDecorationsRef?: React.MutableRefObject<globalThis.monaco.editor.IEditorDecorationsCollection | null>
) => {
  if (resource.query) {
    const searchParams = new URLSearchParams(resource.query)
    if (searchParams.has('signatureHash') && searchParams.has('contract')) {
      onSearchTxn?.(
        searchParams.get('contract') || '',
        searchParams.get('signatureHash') || ''
      )
      trackEvent('Code Search', {
        type: 'related transactions',
        contract: searchParams.get('contract') || '',
        signatureHash: searchParams.get('signatureHash') || ''
      })
      return false
    }

    const lineNumber = parseInt(searchParams.get('curLine') || '')
    const column = parseInt(searchParams.get('curColumn') || '')
    const selection = new monaco.Selection(
      lineNumber,
      column,
      lineNumber,
      column
    )
    sourceEditor.setSelection(selection)
    const handlerId =
      searchParams.get('jump') === 'def'
        ? 'editor.action.revealDefinition'
        : 'editor.action.goToReferences'
    sourceEditor.trigger(null, handlerId, null)
    return true
  }

  const model = monaco.editor.getModel(resource)
  const prevModel = sourceEditor.getModel()
  if (!model) {
    return false
  }
  sourceEditor.setModel(model)
  if (prevModel?.uri.toString() !== model.uri.toString()) {
    trackEvent('Code Search', {
      type: 'switch file',
      previous: prevModel?.uri.toString() || '',
      current: model.uri.toString()
    })
  }
  let line: number | undefined
  if (selectionOrPosition) {
    if ((selectionOrPosition as monaco.IPosition).lineNumber) {
      const { lineNumber, column } = selectionOrPosition as monaco.IPosition
      sourceEditor.revealRangeInCenterIfOutsideViewport({
        startLineNumber: lineNumber,
        startColumn: column,
        endLineNumber: lineNumber,
        endColumn: column
      })
      editorDecorationsRef?.current?.set([
        {
          range: {
            startLineNumber: lineNumber,
            startColumn: column,
            endLineNumber: lineNumber,
            endColumn: column
          },
          options: {
            isWholeLine: true,
            className: 'selected-line'
          }
        }
      ])
      line = lineNumber
    } else {
      sourceEditor.revealRangeInCenterIfOutsideViewport(
        selectionOrPosition as monaco.IRange
      )
      editorDecorationsRef?.current?.set([
        {
          range: selectionOrPosition as monaco.IRange,
          options: {
            isWholeLine: true,
            className: 'selected-line'
          }
        }
      ])
      line = (selectionOrPosition as monaco.IRange).startLineNumber
    }
  }
  onModelChange?.(resource, line)
  return true
}
