import {
  ScipSolidity,
  Source,
  SymbolInformationKind as SIK,
  Definition,
  Occurrence,
  FileRange,
  Symbol,
  FileReferences
} from '@sentio/scip'
import {
  GetContractIndexRequest,
  GetContractIndexResponse,
  SolidityService
} from 'gen/service/solidity/protos/service.pb'
import { Monaco } from '@monaco-editor/react'
import type * as monacoEditor from 'monaco-editor'
import { identity, isEqual } from 'lodash'
import { getChainSearchSupport } from 'lib/data/use-chain-config'
import { withJsonApiAutoToken } from 'lib/data/with-json-api'

const DefaultIcon = 'codicon-symbol-field'
const SymbolIcons = {
  [SIK.Method]: 'codicon-symbol-method',
  [SIK.Function]: 'codicon-symbol-method',
  [SIK.Event]: 'codicon-symbol-event',
  [SIK.Variable]: 'codicon-symbol-variable',
  [SIK.Class]: 'codicon-symbol-class',
  [SIK.Namespace]: 'codicon-symbol-namespace'
}

type FileRangeWithLines = FileRange & { lines?: string[] }

export const enum SymbolKind {
  File = 0,
  Module = 1,
  Namespace = 2,
  Package = 3,
  Class = 4,
  Method = 5,
  Property = 6,
  Field = 7,
  Constructor = 8,
  Enum = 9,
  Interface = 10,
  Function = 11,
  Variable = 12,
  Constant = 13,
  String = 14,
  Number = 15,
  Boolean = 16,
  Array = 17,
  Object = 18,
  Key = 19,
  Null = 20,
  EnumMember = 21,
  Struct = 22,
  Event = 23,
  Operator = 24,
  TypeParameter = 25
}

const SymbolMap = {
  [SIK.Array]: SymbolKind.Array,
  [SIK.Assertion]: SymbolKind.Boolean, // not sure
  [SIK.AssociatedType]: SymbolKind.TypeParameter, // not sure
  [SIK.Attribute]: SymbolKind.Property,
  [SIK.Axiom]: SymbolKind.Struct, // not sure
  [SIK.Boolean]: SymbolKind.Boolean,
  [SIK.Class]: SymbolKind.Class,
  [SIK.Constant]: SymbolKind.Constant,
  [SIK.Constructor]: SymbolKind.Constructor,
  [SIK.DataFamily]: SymbolKind.Struct, // not sure
  [SIK.Enum]: SymbolKind.Enum,
  [SIK.EnumMember]: SymbolKind.EnumMember,
  [SIK.Event]: SymbolKind.Event,
  [SIK.Fact]: SymbolKind.Number,
  [SIK.Field]: SymbolKind.Field,
  [SIK.File]: SymbolKind.File,
  [SIK.Function]: SymbolKind.Function,
  [SIK.Getter]: SymbolKind.Method,
  [SIK.Grammar]: SymbolKind.Function, // not sure
  [SIK.Instance]: SymbolKind.Object,
  [SIK.Interface]: SymbolKind.Interface,
  [SIK.Key]: SymbolKind.Key,
  [SIK.Lang]: SymbolKind.String, // not sure
  [SIK.Lemma]: SymbolKind.Struct, // not sure
  [SIK.Macro]: SymbolKind.Function, // not sure
  [SIK.Method]: SymbolKind.Method,
  [SIK.MethodReceiver]: SymbolKind.Object, // not sure
  [SIK.Message]: SymbolKind.Enum, // not sure
  [SIK.Module]: SymbolKind.Module,
  [SIK.Namespace]: SymbolKind.Namespace,
  [SIK.Null]: SymbolKind.Null,
  [SIK.Object]: SymbolKind.Object,
  [SIK.Operator]: SymbolKind.Operator,
  [SIK.Package]: SymbolKind.Package,
  [SIK.PackageObject]: SymbolKind.Object,
  [SIK.Parameter]: SymbolKind.TypeParameter, // not sure
  [SIK.ParameterLabel]: SymbolKind.String, // not sure
  [SIK.Pattern]: SymbolKind.Struct, // not sure
  [SIK.Predicate]: SymbolKind.Struct, // not sure
  [SIK.Property]: SymbolKind.Property,
  [SIK.Protocol]: SymbolKind.Struct, // not sure
  [SIK.Quasiquoter]: SymbolKind.Struct, // not sure
  [SIK.SelfParameter]: SymbolKind.TypeParameter, // not sure
  [SIK.Setter]: SymbolKind.Method,
  [SIK.Signature]: SymbolKind.Constant,
  [SIK.Subscript]: SymbolKind.TypeParameter,
  [SIK.String]: SymbolKind.String,
  [SIK.Struct]: SymbolKind.Struct,
  [SIK.Tactic]: SymbolKind.Constant,
  [SIK.Theorem]: SymbolKind.Constant, // not sure
  [SIK.ThisParameter]: SymbolKind.Object, // not sure
  [SIK.Trait]: SymbolKind.Constant, // not sure
  [SIK.Type]: SymbolKind.TypeParameter, // not sure
  [SIK.TypeAlias]: SymbolKind.TypeParameter, // not sure
  [SIK.TypeClass]: SymbolKind.Class,
  [SIK.TypeFamily]: SymbolKind.TypeParameter, // not sure
  [SIK.TypeParameter]: SymbolKind.TypeParameter,
  [SIK.Union]: SymbolKind.Struct,
  [SIK.Value]: SymbolKind.Variable,
  [SIK.Variable]: SymbolKind.Variable,
  [SIK.Contract]: SymbolKind.Function,
  [SIK.Library]: SymbolKind.Module,
  [SIK.Modifier]: SymbolKind.Function,
  [SIK.Error]: SymbolKind.Struct, // not sure
  [SIK.UnspecifiedKind]: SymbolKind.Enum, // not sure
  [SIK.Number]: SymbolKind.Number
}

function getRange(monaco: Monaco, scipRange: number[]): monacoEditor.IRange {
  if (scipRange.length == 4) {
    return new monaco.Range(
      scipRange[0] + 1,
      scipRange[1] + 1,
      scipRange[2] + 1,
      scipRange[3] + 1
    )
  } else {
    return new monaco.Range(
      scipRange[0] + 1,
      scipRange[1] + 1,
      scipRange[0] + 1,
      scipRange[2] + 1
    )
  }
}

function getUri(monaco: Monaco, path: string, address?: string) {
  return monaco.Uri.parse(`file:///${address}/${path}`)
}

function escape(str: string) {
  // ignore markdown escape for now
  return str
  // return escapeMarkdown(str, [])
}

function renderDocLines(lines?: string[]) {
  if (!lines || lines.length === 0) {
    return ''
  }
  const splitLines = lines.map((line) => line.split('\n')).flat()
  return (
    splitLines
      .map((line) => {
        const match = line.trim().match(/^(@\w+)\s+(.*)/)
        if (match) {
          switch (match[1]) {
            case '@param':
            case '@return':
              // eslint-disable-next-line no-case-declarations
              const [paramName] = match[2].trim().split(/\s+/)
              return `<span style="color:#7664FC;">${
                match[1]
              }</span>&nbsp;<span style="color:#7664FC;">\`${paramName}\`</span>&nbsp;<span style="color:#717379;">${escape(
                match[2].replace(paramName, '')
              )}</span>`
            default:
              return `<span style="color:#7664FC;">${match[1]}</span>&nbsp;<span style="color:#717379;">${escape(
                match[2]
              )}</span>`
          }
        }
        return `<span style="color:#717379;">${escape(line)}</span>`
      })
      .join('<br>') + '<br>'
  )
}

const contractIndexCache = new Map<string, Promise<GetContractIndexResponse>>()
async function getContractIndex(req: GetContractIndexRequest) {
  const key = JSON.stringify({
    address: req.address?.toLowerCase(),
    'chainSpec.chainId': req.networkId,
    txId: req.txId,
    projectOwner: req.projectOwner,
    projectSlug: req.projectSlug
  })
  if (contractIndexCache.has(key)) {
    return contractIndexCache.get(key)!
  }
  const res = withJsonApiAutoToken(SolidityService.GetContractIndex)(req)
  contractIndexCache.set(key, res)
  return res
}

const emptyHover: monacoEditor.languages.Hover = {
  contents: []
}

export class SoliditySourceParser {
  private readonly scipInstance: Promise<ScipSolidity | null>
  private readonly source: Source[]
  private readonly networkId?: string
  private readonly address?: string
  private readonly simulationId?: string
  private readonly projectOwner?: string
  private readonly projectSlug?: string
  private readonly userCompilationId?: string
  private readonly isForkedChain?: boolean

  constructor(
    source: Source[],
    networkId?: string,
    address?: string,
    simulationId?: string,
    scipInstance?: Promise<ScipSolidity>,
    projectOwner?: string,
    projectSlug?: string,
    userCompilationId?: string,
    isForkedChain?: boolean
  ) {
    this.source = source
    this.networkId = networkId
    this.address = address
    this.simulationId = simulationId
    this.projectOwner = projectOwner
    this.projectSlug = projectSlug
    this.userCompilationId = userCompilationId
    this.isForkedChain = isForkedChain
    this.scipInstance = scipInstance || this.getScipInstance()
  }

  private async getScipInstance(): Promise<ScipSolidity | null> {
    try {
      // TODO cache API return
      const request = { address: this.address } as GetContractIndexRequest
      if (this.isForkedChain && this.networkId) {
        request.chainSpec = {
          forkId: this.networkId
        }
      } else if (this.networkId) {
        request.chainSpec = { chainId: this.networkId }
      }
      if (this.simulationId) {
        request.txId = { simulationId: this.simulationId }
      }
      if (this.projectOwner && this.projectSlug) {
        Object.assign(request, {
          projectOwner: this.projectOwner,
          projectSlug: this.projectSlug
        })
      }
      if (this.userCompilationId) {
        request.userCompilationId = this.userCompilationId
      }
      const res = await getContractIndex(request)
      return new ScipSolidity(this.source, res.index!)
    } catch {
      //TODO handle error
    }
    return null
  }

  public async getRawSymbols(path: string) {
    const scip = await this.scipInstance
    if (!scip) return []
    return scip.getOutline(path)
  }

  public async getDocumentSymbols(
    path: string
  ): Promise<monacoEditor.languages.DocumentSymbol[]> {
    const scip = await this.scipInstance
    if (!scip) return []
    const res: monacoEditor.languages.DocumentSymbol[] = []
    scip.getSymbols(path).forEach((item) => {
      const { symbol, occur } = item
      res.push({
        name: symbol.displayName || symbol.symbol || '',
        detail: symbol.symbol!,
        kind: SymbolMap[symbol.kind!] as any,
        range: getRange(occur.range!),
        selectionRange: getRange(occur.enclosingRange!),
        tags: []
      })
    })
    return res
  }

  public async getDefinition(
    monaco: Monaco,
    path: string,
    lineNumber: number,
    column: number
  ): Promise<monacoEditor.languages.Definition> {
    const scip = await this.scipInstance
    if (!scip) return []
    const definition = scip.findDefinition(path, lineNumber - 1, column - 1)
    if (!definition) return []
    if (!definition.range) {
      return {
        range: {
          startLineNumber: 1,
          startColumn: 1,
          endLineNumber: 1,
          endColumn: 1
        },
        uri: getUri(monaco, definition.sourcePath, this.address)
      }
    }
    const { start, end } = definition.range
    return {
      range: {
        startLineNumber: start.line + 1,
        startColumn: start.character + 1,
        endLineNumber: end.line + 1,
        endColumn: end.character + 1
      },
      targetSelectionRange: {
        startLineNumber: start.line + 1,
        startColumn: start.character + 1,
        endLineNumber: end.line + 1,
        endColumn: end.character + 1
      },
      uri: getUri(monaco, definition!.sourcePath, this.address)
    } as monacoEditor.languages.LocationLink
  }

  public async getHoverData(
    path: string,
    lineNumber: number,
    column: number
  ): Promise<{
    definition?: Definition
    occurence?: Occurrence
    position?: monacoEditor.IPosition
    references?: (FileRange & { lines?: string[] })[]
    implementations?: (FileRange & { lines?: string[] })[]
    interfaces?: (FileRange & { lines?: string[] })[]
  }> {
    const scip = await this.scipInstance
    if (!scip) return {}
    const occurence = scip.locateOccurrence(path, lineNumber - 1, column - 1)
    const definition = scip.findDefinition(path, lineNumber - 1, column - 1)
    if (!definition) return {}
    const { range } = definition
    const res: FileReferences = scip.findReferences(definition.symbol.symbol!)
    const references: FileRangeWithLines[] = []
    const interfaces: FileRangeWithLines[] = []
    const implementations: FileRangeWithLines[] = []

    function parseFn(ref: FileRange) {
      // if (isEqual([ref.start.line, ref.start.character, ref.end.character], occurence?.range)) {
      //   return undefined
      // }
      const startLine = ref.start.line
      const endLine = ref.end.line
      const lines: string[] = []
      for (let i = startLine; i <= endLine; i++) {
        const line = scip?.getLine(ref.sourcePath, i)
        if (line) {
          lines.push(line)
        }
      }
      return { ...ref, lines }
    }
    res.references.forEach((ref) => {
      const parsed = parseFn(ref)
      if (parsed) {
        references.push(parsed)
      }
    })
    res.interfaces.forEach((ref) => {
      const parsed = parseFn(ref)
      if (parsed) {
        interfaces.push(parsed)
      }
    })
    res.implementations.forEach((ref) => {
      const parsed = parseFn(ref)
      if (parsed) {
        implementations.push(parsed)
      }
    })

    return {
      definition,
      occurence,
      position: {
        lineNumber: range ? range?.start.line + 1 : lineNumber,
        column: range ? range?.start.character + 1 : column
      },
      references,
      interfaces,
      implementations
    }
  }

  public async getHover(
    path: string,
    lineNumber: number,
    column: number
  ): Promise<monacoEditor.languages.Hover> {
    const scip = await this.scipInstance
    if (!scip) return emptyHover
    const definition = scip.findDefinition(path, lineNumber - 1, column - 1)
    if (!definition) return emptyHover
    const {
      symbol: { kind, signatureDocumentation, documentation, displayName },
      range,
      signatureHash
    } = definition
    const description = signatureDocumentation?.text || displayName
    let symbolClass = DefaultIcon
    if (kind && SymbolIcons[kind]) {
      symbolClass = SymbolIcons[kind]
    }

    const contents: monacoEditor.IMarkdownString[] = [
      {
        value: '```sentio-solidity\n' + description + '\n```'
      }
    ]

    if (documentation) {
      contents.push({
        isTrusted: true,
        supportHtml: true,
        value: renderDocLines(documentation)
      })
    }

    const targetQuery = new URLSearchParams()
    targetQuery.set('curLine', lineNumber.toString())
    targetQuery.set('curColumn', column.toString())
    if (range?.start.line) {
      targetQuery.set('lineNumber', range.start.line.toString())
    }
    if (range?.start.character) {
      targetQuery.set('column', range.start.character.toString())
    }

    if (
      definition.sourcePath.startsWith('http://') ||
      definition.sourcePath.startsWith('https://')
    ) {
      // external link
      contents.push({
        isTrusted: true,
        value: `**[go to definition](${definition.sourcePath})**`
      })
      return {
        contents
      }
    }

    contents.push({
      isTrusted: true,
      value: [
        // skip "Go to definition" when current line is definition
        range?.start.line === lineNumber - 1
          ? undefined
          : `**[Go to definition](file:///${this.address}/${
              definition.sourcePath
            }?${targetQuery.toString()}&jump=def)**`,
        `**[Find references](file:///${this.address}/${definition.sourcePath}?${targetQuery.toString()}&jump=ref)**`,
        this.networkId &&
        (await getChainSearchSupport()).includes(this.networkId) &&
        kind === SIK.Function &&
        signatureHash
          ? `**[View related transactions](file:///$?signatureHash=${signatureHash}&contract=${this.address}&jump=external)**`
          : ''
      ]
        .filter(identity)
        .join('&nbsp;&nbsp;&nbsp;&nbsp;|&nbsp;&nbsp;&nbsp;&nbsp;')
    })

    return { contents }
  }

  public async getImplementation(
    monaco: Monaco,
    path: string,
    lineNumber: number,
    column: number
  ): Promise<monacoEditor.languages.Location[]> {
    const scip = await this.scipInstance
    if (!scip) return []
    const fileRanges = scip.findReferencesByLocation(
      path,
      lineNumber - 1,
      column - 1
    )
    if (!fileRanges) return []
    return fileRanges.map((item) => {
      const { start, end } = item
      return {
        range: {
          startLineNumber: start.line + 1,
          startColumn: start.character + 1,
          endLineNumber: end.line + 1,
          endColumn: end.character + 1
        },
        targetSelectionRange: {
          startLineNumber: start.line + 1,
          startColumn: start.character + 1,
          endLineNumber: end.line + 1,
          endColumn: end.character + 1
        },
        uri: getUri(monaco, item.sourcePath, this.address)
      } as monacoEditor.languages.LocationLink
    })
  }

  public async getAllDefinitions(): Promise<Definition[]> {
    const scip = await this.scipInstance
    if (!scip) return []
    return scip.definitions()
  }

  public async getFileContent(path: string): Promise<string> {
    const scip = await this.scipInstance
    if (!scip) return ''
    return scip.getLines(path) || ''
  }
}
