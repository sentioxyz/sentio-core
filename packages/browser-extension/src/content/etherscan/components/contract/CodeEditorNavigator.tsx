import { useState, useCallback, useEffect, useRef } from 'react'
import * as monaco from 'monaco-editor'

type CodeHistory = {
  sourceEditor: monaco.editor.ICodeEditor
  resource: monaco.Uri
  selectionOrPosition?: monaco.IPosition | monaco.IRange
  jumpFn?: () => void
}

interface Props {
  history?: CodeHistory[]
}

export const CodeEditorNavigator = ({ history = [] }: Props) => {
  const [index, setIndex] = useState(history.length - 1)
  const historyRef = useRef<CodeHistory[]>([])
  historyRef.current = history

  useEffect(() => {
    if (history.length === 0) {
      setIndex(0)
      return
    }
    setIndex(history.length - 1)
  }, [history])

  const onIndexChange = useCallback((index: number) => {
    const { sourceEditor, jumpFn, selectionOrPosition, resource } =
      historyRef.current[index]
    const domNode = sourceEditor.getDomNode()
    if (!domNode) {
      return
    }
    // scroll to the editor
    const rect = domNode.getBoundingClientRect()
    if (!rect) {
      return
    }
    window.scrollBy(0, rect.top - 100)
    // jump to definition
    jumpFn?.()
  }, [])

  const onPrevious = useCallback(() => {
    setIndex((index) => {
      if (index > 0) {
        setTimeout(() => onIndexChange(index - 1), 0)
        return index - 1
      }
      return index
    })
  }, [])
  const onNext = useCallback(() => {
    setIndex((index) => {
      if (history && index < history.length - 1) {
        setTimeout(() => onIndexChange(index + 1), 0)
        return index + 1
      }
      return index
    })
  }, [history])

  const isPreviousDisabled = index === 0
  const isNextDisabled = !history || index >= history.length - 1

  if (!history || history.length === 0) {
    return null
  }

  return (
    <div
      className="_sentio_"
      style={{
        display: 'inline-flex',
        gap: '8px',
        marginRight: '16px'
      }}
    >
      <button
        className={`inline-block rounded px-2 py-1 font-bold text-white ${
          isPreviousDisabled
            ? 'cursor-not-allowed bg-gray-500'
            : 'bg-blue-500 hover:bg-blue-700'
        }`}
        onClick={onPrevious}
        title="To previous code location"
      >
        <i className="fas fa-arrow-left"></i>
      </button>
      <button
        className={`inline-block rounded px-2 py-1 font-bold text-white ${
          isNextDisabled
            ? 'cursor-not-allowed bg-gray-500'
            : 'bg-blue-500 hover:bg-blue-700'
        }`}
        onClick={onNext}
        title="To next code location"
      >
        <i className="fas fa-arrow-right"></i>
      </button>
    </div>
  )
}
