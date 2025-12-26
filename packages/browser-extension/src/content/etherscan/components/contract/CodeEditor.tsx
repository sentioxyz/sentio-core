import * as monaco from 'monaco-editor'
import MonacoEditor from '@monaco-editor/react'
import { useCallback, useEffect, useState, useRef } from 'react'
import { RelatedTxn } from './RelatedTxn'
import { sentioTheme } from '@sentio/ui-web3'

const setSentioTheme = (monaco: any) => {
  monaco.editor.defineTheme('sentio', sentioTheme)
}

const monacoEditorOptions: monaco.editor.IStandaloneEditorConstructionOptions =
  {
    readOnly: true,
    minimap: {
      enabled: false
    },
    hover: {
      hidingDelay: 400
    },
    scrollBeyondLastLine: false
  }

interface Props {
  path: string
  model: monaco.editor.ITextModel
  scrollIntoView: (path: string) => void
  chainId: string
  addHistory: (data: any) => void
}

export const CodeEditor = ({
  path,
  model,
  scrollIntoView,
  chainId,
  addHistory
}: Props) => {
  const decorationsCollectionRef = useRef<any>(null)
  const pathRef = useRef<string>(path)
  pathRef.current = path
  const disposablesRef = useRef<monaco.IDisposable[]>([])
  const [sig, setSig] = useState('')
  const [contract, setContract] = useState('')
  const [visible, setVisible] = useState(false)

  const jump = useCallback(
    (
      sourceEditor: monaco.editor.ICodeEditor,
      resource: monaco.Uri,
      selectionOrPosition?: monaco.IPosition | monaco.IRange
    ) => {
      const fn = () => {
        let range
        // Go to definition from hover popup.
        if (resource.query) {
          const searchParams = new URLSearchParams(resource.query)
          if (searchParams.has('lineNumber') && searchParams.has('column')) {
            const lineNumber =
              parseInt(searchParams.get('lineNumber') || '') + 1
            const column = parseInt(searchParams.get('column') || '') + 1
            range = {
              startLineNumber: lineNumber,
              startColumn: column,
              endLineNumber: lineNumber,
              endColumn: column
            }
          }
        }
        // Go to definition from context menu.
        if (selectionOrPosition) {
          if ((selectionOrPosition as monaco.IPosition).lineNumber) {
            const { lineNumber, column } =
              selectionOrPosition as monaco.IPosition
            range = {
              startLineNumber: lineNumber,
              startColumn: column,
              endLineNumber: lineNumber,
              endColumn: column
            }
          } else {
            range = selectionOrPosition as monaco.IRange
          }
        }
        if (range) {
          sourceEditor.revealRangeInCenterIfOutsideViewport(range)
          if (decorationsCollectionRef.current) {
            decorationsCollectionRef.current.clear()
          }
          const decorationsCollection =
            sourceEditor.createDecorationsCollection([
              {
                range: range,
                options: {
                  className: 'rangeHighlight',
                  isWholeLine: true
                }
              }
            ])
          decorationsCollectionRef.current = decorationsCollection
          setTimeout(() => {
            decorationsCollection.clear()
          }, 3000)
        }
      }
      if (addHistory) {
        addHistory({
          sourceEditor,
          resource,
          selectionOrPosition,
          jumpFn: fn
        })
      }
      fn()
    },
    []
  )

  const handleEditorDidMount = useCallback(
    (editor: monaco.editor.IStandaloneCodeEditor, monaco) => {
      setSentioTheme(monaco)
      editor.setModel(model)

      let active = false

      editor.onMouseMove(() => (active = true))
      editor.onMouseLeave(() => (active = false))

      disposablesRef.current.push(
        monaco.editor.registerEditorOpener({
          openCodeEditor(
            sourceEditor: monaco.editor.ICodeEditor,
            resource: monaco.Uri,
            selectionOrPosition?: monaco.IPosition | monaco.IRange
          ) {
            if (!active) {
              return
            }
            sourceEditor.focus()
            if (resource.query) {
              const searchParams = new URLSearchParams(resource.query)
              if (
                searchParams.has('signatureHash') &&
                searchParams.has('contract')
              ) {
                setContract(searchParams.get('contract') || '')
                setSig(searchParams.get('signatureHash') || '')
                setVisible(true)
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

            for (const ed of monaco.editor.getEditors()) {
              const model = ed.getModel()
              if (model.uri.path === resource.path) {
                jump(ed, resource, selectionOrPosition)
                if (pathRef.current !== resource.path) {
                  scrollIntoView(resource.path)
                } else {
                  // ignore
                }
              }
            }

            return true
          }
        })
      )
    },
    []
  )

  useEffect(() => {
    return () => {
      disposablesRef.current.forEach((disposable) => disposable.dispose())
    }
  }, [])

  return (
    <>
      <div
        className="absolute inset-0 whitespace-normal bg-white"
        style={{ zIndex: 99 }}
      >
        <MonacoEditor
          options={monacoEditorOptions}
          defaultLanguage="sentio-solidity"
          onMount={handleEditorDidMount}
        />
      </div>
      <RelatedTxn
        chainId={chainId}
        open={visible}
        onClose={() => {
          setVisible(false)
        }}
        sig={sig}
        address={contract}
      />
    </>
  )
}
