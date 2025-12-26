import { Component, MutableRefObject } from 'react'
import MonacoEditor from '@monaco-editor/react'
import { Monaco } from '@monaco-editor/react'
import { Location } from '@sentio/debugger'
import { setSentioTheme } from '../editor/SentioTheme'
import {
  openCodeEditor,
  setSolidityLanguage,
  setSolidityProviders
} from '../editor/solidity'
import { SourceStore } from '../editor/SourceStore'
import { parseUri } from '../utils/debug-helpers'
import { ArrowUturnLeftIcon } from '@heroicons/react/24/outline'
import { SoliditySourceParser } from '../editor/SoliditySourceParser'
import { Definition, FileRange, Occurrence } from '@sentio/scip'
import { HoverContextWidget } from './HoverContextWidget'
import { Button, classNames } from '@sentio/ui-core'
import * as monaco from 'monaco-editor'

const monacoEditorOptions: monaco.editor.IStandaloneEditorConstructionOptions =
  {
    model: null,
    readOnly: true,
    scrollBeyondLastLine: false,
    hover: {
      hidingDelay: 400
    }
  }

interface Props {
  model?: monaco.editor.ITextModel
  location?: Location
  store?: SourceStore
  setSig: any
  setContract: any
  openSlideOver: any
  openRefSlider?: ((tabName?: string | undefined) => void) | undefined
  setRefSliderData?: ((data: any) => void) | undefined
  contractAddress?: string
  setContractAddress?: (addr: string) => void
  onOpenRef?: MutableRefObject<
    (
      address: string,
      filePath: string,
      line?: number | undefined
    ) => void | undefined
  >
  chain?: string | number
  isDarkMode?: boolean
  onTrackEvent?: (eventName: string, properties?: Record<string, any>) => void
}

interface SourceViewState {
  isEditorReady: boolean
  currentModel: null | monaco.editor.ITextModel
  hoverDef?: Definition
  occur?: Occurrence
  hoverRefs?: FileRange[]
  hoverImpls?: FileRange[]
  hoverInterfaces?: FileRange[]
}

/**
 * SourceView Component
 *
 * A Monaco editor-based component for viewing and navigating Solidity source code.
 * Integrates with SourceStore for code intelligence features like hover tooltips,
 * go-to-definition, and reference finding.
 *
 * @example
 * ```tsx
 * import { SourceView, LocationViewer, LocationStatus, isLocationStatus } from '@sentio/ui-web3'
 *
 * // Use with LocationViewer's renderSourceView prop
 * <LocationViewer
 *   currentLocation={currentLocation}
 *   currentModel={currentModel}
 *   store={store}
 *   contractAddress={contractAddress}
 *   setContractAddress={setContractAddress}
 *   chainId={chainId}
 *   isDarkMode={isDarkMode}
 *   onOpenRef={onOpenRef}
 *   openSlider={openSlider}
 *   setSliderData={setSliderData}
 *   renderSourceView={(props) => (
 *     <SourceView
 *       model={currentModel}
 *       location={isLocationStatus(currentLocation) ? undefined : currentLocation}
 *       store={store}
 *       setSig={setSig}
 *       setContract={setContract}
 *       openSlideOver={openSlideOver}
 *       openRefSlider={props.openSlider}
 *       setRefSliderData={props.setSliderData}
 *       contractAddress={props.contractAddress}
 *       setContractAddress={props.setContractAddress}
 *       onOpenRef={props.onOpenRef}
 *       chain={props.chainId}
 *       isDarkMode={props.isDarkMode}
 *       onTrackEvent={(eventName, properties) => {
 *         // Optional: Track analytics events
 *         console.log(eventName, properties)
 *       }}
 *     />
 *   )}
 * />
 * ```
 */
export class SourceView extends Component<Props, SourceViewState> {
  editorRef: monaco.editor.IStandaloneCodeEditor | null = null
  editorDecorationsRef: any = null
  monacoRef: Monaco | null = null
  state = {
    isEditorReady: false,
    currentModel: null,
    hoverDef: undefined,
    hoverImpls: undefined,
    hoverInterfaces: undefined,
    occur: undefined,
    hoverRefs: []
  }
  highlightDecorations: string[] = []
  store: SourceStore | null = null
  disposals: monaco.IDisposable[] = []

  constructor(props: Props) {
    super(props)
    this.handleEditorDidMount = this.handleEditorDidMount.bind(this)
    this.resetPosition = this.resetPosition.bind(this)
  }

  handleEditorDidMount(
    editor: monaco.editor.IStandaloneCodeEditor,
    monaco: Monaco
  ) {
    const {
      setSig,
      setContract,
      openSlideOver,
      onOpenRef,
      chain,
      onTrackEvent
    } = this.props
    this.editorRef = editor
    this.monacoRef = monaco
    this.editorDecorationsRef = {
      current: editor.createDecorationsCollection()
    }
    this.setState({
      isEditorReady: true
    })
    setSentioTheme(monaco)
    setSolidityLanguage(monaco)
    const collectionRef = {
      current: editor.createDecorationsCollection()
    } as any

    if (onOpenRef) {
      onOpenRef.current = (address, filePath, line) => {
        const targetUri = monaco.Uri.parse(`file:///${address}/${filePath}`)
        const model = monaco.editor.getModel(targetUri)
        const prevModel = editor.getModel()
        if (!model) {
          return
        }
        if (prevModel?.uri.toString() !== model.uri.toString()) {
          onTrackEvent?.('Code Search', {
            type: 'switch file',
            previous: prevModel?.uri.toString() || '',
            current: model.uri.toString(),
            chain: chain
          })
          editor.setModel(model)
        }
        if (line) {
          editor.revealLineInCenterIfOutsideViewport(line)
        }
        this.editorDecorationsRef.current.set([
          {
            range: {
              startLineNumber: line,
              startColumn: 0,
              endLineNumber: line,
              endColumn: 0
            },
            options: {
              isWholeLine: true,
              className: 'selected-line'
            }
          }
        ])
      }
    }

    if (this.props.store) {
      const store = this.props.store
      this.disposals.push(
        ...setSolidityProviders(
          monaco,
          this.props.store,
          parseUri,
          this.props.openRefSlider
            ? (
                model: monaco.editor.ITextModel,
                position: monaco.Position,
                token: monaco.CancellationToken
              ) => {
                if (model.uri.scheme !== 'file' || !store) {
                  return
                }
                const { address, path } = parseUri(model.uri)
                const parser = store.getParser(address) as SoliditySourceParser
                parser
                  .getHoverData(path, position.lineNumber, position.column)
                  .then(
                    ({
                      definition,
                      occurence,
                      references,
                      implementations,
                      interfaces
                    }) => {
                      this.setState({
                        hoverDef: definition,
                        occur: occurence,
                        hoverRefs: references,
                        hoverImpls: implementations,
                        hoverInterfaces: interfaces
                      })
                      this.props.setContractAddress?.(address)
                    }
                  )
              }
            : undefined
        )
      )
      this.disposals.push(
        monaco.editor.registerEditorOpener({
          openCodeEditor: (
            sourceEditor: monaco.editor.ICodeEditor,
            resource: monaco.Uri,
            selectionOrPosition?: monaco.IPosition | monaco.IRange
          ) => {
            return openCodeEditor(
              monaco,
              sourceEditor,
              resource,
              selectionOrPosition,
              (contract, sig) => {
                setSig(sig)
                setContract(contract)
                openSlideOver(true)
              },
              (uri) => {
                const currentModel = monaco.editor.getModel(uri)
                if (currentModel) {
                  this.setState({
                    currentModel
                  })
                }
              },
              collectionRef
            )
          }
        })
      )
    }
  }

  componentDidUpdate(
    prevProps: Readonly<Props>,
    prevState: Readonly<SourceViewState>,
    snapshot?: any
  ): void {
    if (!this.editorRef) {
      return
    }

    if (prevProps.model !== this.props.model) {
      if (prevProps.model && !prevProps.model.isDisposed()) {
        prevProps.model.deltaDecorations(this.highlightDecorations, [])
      }
      this.highlightDecorations = []
    }

    // current model changes, ignore
    if (
      this.state.currentModel &&
      this.state.currentModel !== prevState.currentModel
    ) {
      return
    }

    if (
      prevProps.model === this.props.model &&
      prevProps.location === this.props.location &&
      this.editorRef.getModel() !== null
    ) {
      return
    }

    if (this.props.model) {
      this.editorRef.setModel(this.props.model)
    }

    const { lines } = this.props.location || {}
    if (!lines || !this.props.model) {
      return
    }
    const model = this.props.model
    const editor = this.editorRef
    const range: monaco.IRange = {
      startLineNumber: lines.start.line + 1,
      startColumn: lines.start.column + 1,
      endLineNumber: lines.end.line + 1,
      endColumn: lines.end.column + 1
    }

    // manual fix: range should not cover multiple lines
    if (range.startLineNumber !== range.endLineNumber) {
      ;(range as any).endLineNumber = range.startLineNumber
      try {
        ;(range as any).endColumn =
          model.getLineMaxColumn(range.startLineNumber) + 1
      } catch {
        ;(range as any).endColumn = range.startColumn
      }
    }

    editor.revealRangeInCenterIfOutsideViewport(range)

    const decorations: monaco.editor.IModelDeltaDecoration[] = [
      {
        range: range,
        options: {
          isWholeLine: true,
          className: classNames('debugger-highlighten-lines'),
          inlineClassName: classNames('debugger-highlighten-inline')
        }
      },
      {
        range: range,
        options: {
          isWholeLine: false,
          inlineClassName: classNames('debugger-highlighten-tokens')
        }
      }
    ]
    this.highlightDecorations = model.deltaDecorations(
      this.highlightDecorations,
      decorations
    )
  }

  componentWillUnmount(): void {
    if (this.props.model) {
      this.props.model.deltaDecorations(this.highlightDecorations, [])
      this.highlightDecorations = []
    }
    if (this.editorRef) {
      this.editorRef.setModel(null)
    }
    if (this.disposals) {
      while (this.disposals.length > 0) {
        const item = this.disposals.pop()
        item?.dispose()
      }
    }
  }

  resetPosition(evt: React.MouseEvent) {
    evt.stopPropagation()
    evt.preventDefault()
    this.setState({
      currentModel: null
    })
  }

  render() {
    const {
      model,
      openRefSlider,
      setRefSliderData,
      contractAddress,
      onOpenRef,
      isDarkMode,
      onTrackEvent
    } = this.props
    const {
      currentModel,
      hoverDef,
      hoverRefs,
      occur,
      hoverImpls,
      hoverInterfaces
    } = this.state
    const isJumpedToOtherModel =
      model &&
      currentModel &&
      model.uri.toString() !==
        (currentModel as monaco.editor.ITextModel)?.uri.toString()
    return (
      <>
        <MonacoEditor
          defaultLanguage="sentio-solidity"
          options={monacoEditorOptions}
          onMount={this.handleEditorDidMount}
          keepCurrentModel
          theme={isDarkMode ? 'sentio-dark' : 'sentio'}
        />
        {isJumpedToOtherModel ? (
          <div className="absolute left-2 top-2 opacity-50 hover:opacity-100">
            <Button
              role="primary"
              icon={<ArrowUturnLeftIcon />}
              size="sm"
              onClick={this.resetPosition}
            />
          </div>
        ) : null}
        {this.editorRef && this.editorDecorationsRef ? (
          <HoverContextWidget
            contractAddress={contractAddress}
            editor={this.editorRef}
            data={hoverDef}
            occurrence={occur}
            references={hoverRefs}
            interfaces={hoverInterfaces}
            implementations={hoverImpls}
            editorDecorationsRef={this.editorDecorationsRef}
            openSlider={openRefSlider}
            setSliderData={setRefSliderData}
            onTrackEvent={onTrackEvent}
            onModelChange={(uri, line) => {
              const currentModel = monaco.editor.getModel(uri)
              if (currentModel) {
                this.setState({
                  currentModel
                })
              }
            }}
          />
        ) : null}
      </>
    )
  }

  public reFocus(location: Location) {
    const { lines } = location
    if (!lines || !this.editorRef) {
      return
    }
    const range: monaco.IRange = {
      startLineNumber: lines.start.line + 1,
      startColumn: lines.start.column + 1,
      endLineNumber: lines.end.line + 1,
      endColumn: lines.end.column + 1
    }
    this.editorRef.revealRangeInCenterIfOutsideViewport(range)
  }
}
