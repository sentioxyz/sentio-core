import { Definition, FileRange, Occurrence, getRange } from '@sentio/scip'
import { useEffect, useRef, ReactNode, useMemo, useCallback } from 'react'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { Button } from '@sentio/ui-core'
import { isEqual } from 'lodash'
import { useMonaco } from '@monaco-editor/react'

function LinkifyText({
  text,
  className
}: {
  text: string
  className?: string
}) {
  const urlRegex = /(https?:\/\/[^\s]+)/g
  const parts = text.split(urlRegex)

  return (
    <span className={className}>
      {parts.map((part, index) => {
        if (part.match(urlRegex)) {
          return (
            <a
              key={index}
              href={part}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-500 underline"
            >
              {part}
            </a>
          )
        }
        return <span key={index}>{part}</span>
      })}
    </span>
  )
}

function renderDocLines(lines?: string[]): ReactNode {
  if (!lines || lines.length === 0) {
    return null
  }
  const splitLines = lines.map((line) => line.split('\n')).flat()
  return splitLines.map((line, index) => {
    const match = line.trim().match(/^(@\w+)\s+(.*)/)
    if (match) {
      switch (match[1]) {
        case '@param':
        case '@return': {
          const [paramName] = match[2].trim().split(/\s+/)
          return (
            <div className="space-x-1" key={index}>
              <span className="text-deep-purple-400 dark:text-deep-purple-700">
                {match[1]}
              </span>
              <span className="text-deep-purple-400 dark:text-deep-purple-700">
                {paramName}
              </span>
              <span className="text-gray">
                {match[2].replace(paramName, '')}
              </span>
            </div>
          )
        }
        default:
          return (
            <div className="space-x-1" key={index}>
              <span className="text-deep-purple-400 dark:text-deep-purple-700">
                {match[1]}
              </span>
              <LinkifyText className="text-gray" text={match[2]} />
            </div>
          )
      }
    }
    return (
      <div key={index}>
        <LinkifyText className="text-gray" text={line} />
      </div>
    )
  })
}

function getFileUri(monaco: any, id = '', path = '') {
  return monaco.Uri.parse(`file:///${id}/${path}`)
}

interface HoverContextWidgetProps {
  editor: monaco.editor.IStandaloneCodeEditor
  occurrence?: Occurrence
  data?: Definition
  references?: FileRange[]
  interfaces?: FileRange[]
  implementations?: FileRange[]
  contractAddress?: string
  chainId?: string
  editorDecorationsRef?: React.MutableRefObject<globalThis.monaco.editor.IEditorDecorationsCollection | null>
  onModelChange?: (uri: monaco.Uri, line?: number) => void
  openSlider?: (tabName?: string) => void
  setSliderData?: (data: any) => void
  supportedTxnSearchChains?: string[]
  onTrackEvent?: (eventName: string, properties?: Record<string, any>) => void
}

export const HoverContextWidget = ({
  editor,
  occurrence,
  data,
  references,
  implementations,
  interfaces,
  contractAddress,
  chainId,
  editorDecorationsRef,
  onModelChange,
  openSlider,
  setSliderData,
  supportedTxnSearchChains = [],
  onTrackEvent
}: HoverContextWidgetProps) => {
  const monaco = useMonaco()
  const nodeRef = useRef<HTMLDivElement>(null)
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor>(editor)
  const contentWidgetRef = useRef<any>(null)
  editorRef.current = editor
  const occurrenceRef = useRef<Occurrence | undefined>(occurrence)
  occurrenceRef.current = occurrence
  const decorationRef = useRef<monaco.editor.IEditorDecorationsCollection>()
  const { symbol = {} } = data || {}
  const { signatureDocumentation, documentation, displayName } = symbol
  const description = signatureDocumentation?.text || displayName
  const abortControllerRef = useRef<AbortController[]>([])

  const [setDelayedHide, clearDelayedHide] = useMemo(() => {
    const clearDelayedHide = () => {
      abortControllerRef.current?.forEach((c) => c.abort())
      abortControllerRef.current = []
    }

    const setDelayedHide = (proxyFn: () => void, delayTime?: number) => {
      if (!delayTime) {
        clearDelayedHide()
        proxyFn()
        return
      }

      const controller = new AbortController()
      abortControllerRef.current.push(controller)
      const timeoutId = setTimeout(() => {
        if (!controller.signal.aborted) {
          proxyFn()
        }
      }, delayTime)

      controller.signal.addEventListener('abort', () => {
        clearTimeout(timeoutId)
      })
    }

    return [setDelayedHide, clearDelayedHide]
  }, [])

  const hideMenu = useCallback(() => {
    abortControllerRef.current?.forEach((c) => c.abort())
    abortControllerRef.current = []
    if (contentWidgetRef.current) {
      editorRef.current.removeContentWidget(contentWidgetRef.current)
      contentWidgetRef.current = null
    }
    nodeRef.current?.classList.add('hidden')
    decorationRef.current?.clear()
  }, [])

  const showMenu = useCallback(
    async (
      startLineNumber: number,
      startColumn: number,
      endLineNumber: number,
      endColumn: number
    ) => {
      if (!nodeRef.current) {
        return
      }

      try {
        abortControllerRef.current?.forEach((c) => c.abort())
        abortControllerRef.current = []
        const controller = new AbortController()
        abortControllerRef.current?.push(controller)
        await new Promise((resolve) => setTimeout(resolve, 300))
        if (controller.signal.aborted) return
      } catch {
        return
      }
      decorationRef.current?.set([
        {
          range: {
            startLineNumber,
            startColumn,
            endLineNumber,
            endColumn
          },
          options: {
            isWholeLine: false,
            className: 'hover-context-widget-decoration'
          }
        }
      ])

      if (!contentWidgetRef.current) {
        contentWidgetRef.current = {
          allowEditorOverflow: true,
          getId: () => 'hover-context-widget',
          getDomNode: () => nodeRef.current,
          getPosition: () => ({
            position: {
              lineNumber: startLineNumber,
              column: startColumn
            },
            preference: [1, 2]
          })
        }
        editorRef.current.addContentWidget(contentWidgetRef.current)
      } else {
        contentWidgetRef.current.getPosition = () => ({
          position: {
            lineNumber: startLineNumber,
            column: startColumn
          },
          preference: [1, 2]
        })
        editorRef.current.layoutContentWidget(contentWidgetRef.current)
      }
      nodeRef.current?.classList.remove('hidden')
      clearDelayedHide()
    },
    []
  )

  useEffect(() => {
    if (occurrence?.range) {
      const { start, end } = getRange(occurrence.range)
      const startLine = start.line + 1
      const startColumn = start.character + 1
      const endLine = end.line + 1
      const endColumn = end.character + 1
      showMenu(startLine, startColumn, endLine, endColumn)
      return
    }
    hideMenu()
  }, [occurrence])

  useEffect(() => {
    const disposal = editor.onDidChangeModel(() => {
      hideMenu()
    })
    decorationRef.current = editor.createDecorationsCollection()

    const disposal2 = editor.onMouseMove((e) => {
      if ((e.target as any)?.detail === 'hover-context-widget') {
        clearDelayedHide()
        return
      }

      if (!occurrenceRef.current || !occurrenceRef.current.range) {
        hideMenu()
        return
      }
      const { start, end } = getRange(occurrenceRef.current.range)
      const startLine = start.line + 1
      const startColumn = start.character + 1
      const endLine = end.line + 1
      const endColumn = end.character + 1
      const position = e.target.position
      if (
        position?.lineNumber === startLine &&
        position?.column >= startColumn &&
        position?.column <= endColumn
      ) {
        showMenu(startLine, startColumn, endLine, endColumn)
      } else {
        setDelayedHide(hideMenu, 100)
      }
    })

    return () => {
      disposal.dispose()
      disposal2.dispose()
      decorationRef.current?.clear()
      decorationRef.current = undefined
    }
  }, [editor])

  const isCurrentDefinition = useMemo(() => {
    if (!occurrence?.range || !data?.range) {
      return false
    }
    const occurRange = getRange(occurrence.range)
    return isEqual(data?.range, occurRange)
  }, [data?.range, occurrence?.range])
  const isExternalDefinition = useMemo(() => {
    if (data?.sourcePath) {
      return (
        data.sourcePath.startsWith('http://') ||
        data.sourcePath.startsWith('https://')
      )
    }
    return false
  }, [data?.sourcePath])
  const hasDefinition = useMemo(() => {
    return references && references?.length > 0
  }, [references])
  const hasImplementation = useMemo(() => {
    const hasImpl = implementations && implementations?.length > 0
    const hasInterface = interfaces && interfaces?.length > 0
    return hasImpl || hasInterface
  }, [implementations, interfaces])
  const storageKey = useMemo(() => {
    if (data?.symbol) {
      const { enclosingSymbol, kind, displayName } = data.symbol
      if (kind === 'Variable' && enclosingSymbol?.startsWith('contract ')) {
        return displayName
      }
    }
    return ''
  }, [data?.symbol])
  return (
    <div>
      <div
        className="dark:bg-sentio-gray-100 text-text-foreground absolute z-10 hidden rounded border bg-white text-xs shadow-sm"
        ref={nodeRef}
      >
        <SyntaxHighlighter
          PreTag="div"
          language="solidity"
          useInlineStyles={false}
          codeTagProps={{
            className:
              'p-2 font-mono max-w-[600px] break-words block font-medium'
          }}
          wrapLongLines={true}
        >
          {description}
        </SyntaxHighlighter>
        {documentation && documentation.length > 0 ? (
          <div className="max-h-[300px] max-w-[600px] space-y-1 overflow-auto border-t px-2 pb-2 pt-1">
            {renderDocLines(documentation)}
          </div>
        ) : null}
        <div className="max-w-[600px] space-x-2 border-t px-2 py-2">
          <Button
            disabled={isCurrentDefinition}
            size="sm"
            role="tertiary"
            onClick={() => {
              if (!data?.sourcePath) {
                return
              }

              if (isExternalDefinition) {
                window.open(data.sourcePath, '_blank')
                return
              }

              try {
                if (!data?.range || !monaco) {
                  return
                }
                const targetUri = getFileUri(
                  monaco,
                  contractAddress,
                  data.sourcePath
                )
                const model = monaco.editor.getModel(targetUri)
                const prevModel = editorRef.current.getModel()
                if (!model) {
                  return
                }
                if (prevModel?.uri.toString() !== model.uri.toString()) {
                  onTrackEvent?.('Code Search', {
                    type: 'switch file',
                    previous: prevModel?.uri.toString() || '',
                    current: model.uri.toString(),
                    chain: chainId
                  })
                  editorRef.current.setModel(model)
                }
                const range = data.range
                editorRef.current.revealRangeInCenterIfOutsideViewport({
                  startLineNumber: range?.start.line + 1,
                  startColumn: range?.start.character + 1,
                  endLineNumber: range?.end.line + 1,
                  endColumn: range?.end.character + 1
                })
                editorDecorationsRef?.current?.set([
                  {
                    range: {
                      startLineNumber: range?.start.line + 1,
                      startColumn: 0,
                      endLineNumber: range?.start.line + 1,
                      endColumn: 0
                    },
                    options: {
                      isWholeLine: true,
                      className: 'selected-line'
                    }
                  }
                ])
                onModelChange?.(
                  editorRef.current.getModel()!.uri,
                  range.start.line + 1
                )
                hideMenu()
              } catch {
                return
              }
            }}
          >
            {isCurrentDefinition
              ? 'You are at the definition'
              : isExternalDefinition
                ? 'go to definition'
                : 'Go to definition'}
          </Button>
          {!isExternalDefinition ? (
            <>
              <Button
                disabled={!hasDefinition}
                size="sm"
                role="tertiary"
                onClick={() => {
                  if (!contractAddress) {
                    return
                  }
                  openSlider?.('references')
                  setSliderData?.({
                    references,
                    implementations,
                    interfaces,
                    functionSignature: data?.signatureHash,
                    symbol
                  })
                  hideMenu()
                }}
              >
                {hasDefinition ? 'Find references' : 'No references'}
              </Button>
              <Button
                disabled={!hasImplementation}
                size="sm"
                role="tertiary"
                onClick={() => {
                  if (!contractAddress) {
                    return
                  }
                  openSlider?.('implementations')
                  setSliderData?.({
                    references,
                    implementations,
                    interfaces,
                    functionSignature: data?.signatureHash,
                    symbol
                  })
                  hideMenu()
                }}
              >
                {hasImplementation ? 'View hierarchy' : 'No hierarchy'}
              </Button>
              {data?.signatureHash &&
              supportedTxnSearchChains.includes(chainId || '') ? (
                <Button
                  size="sm"
                  role="tertiary"
                  onClick={() => {
                    openSlider?.('related-txns')
                    setSliderData?.({
                      references,
                      functionSignature: data?.signatureHash,
                      symbol
                    })
                    hideMenu()
                  }}
                >
                  Related transactions
                </Button>
              ) : null}
              {storageKey ? (
                <Button
                  size="sm"
                  role="tertiary"
                  onClick={() => {
                    openSlider?.('storage')
                    hideMenu()
                  }}
                >
                  View storage
                </Button>
              ) : null}
            </>
          ) : null}
        </div>
      </div>
    </div>
  )
}
