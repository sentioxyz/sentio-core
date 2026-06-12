import {
  Document,
  Index,
  Occurrence,
  SymbolInformation,
  SymbolInformation_Kind,
  SymbolRole
} from './gen/scip_pb'

// TODO used shared types
export interface Source {
  sourcePath: string
  source: string
}

export interface Definition {
  symbol: SymbolInformation
  signatureHash: string | undefined
  sourcePath: string
  range: Range | undefined
  implementations: Definition[]
  // occurrence: Occurrence
}

export function hoverContent(s: SymbolInformation) {
  return `${SymbolInformation_Kind[s.kind]?.toLowerCase()} ${s.signatureDocumentation?.text.split('\n')[0]}`
}

export interface OutlineTree extends Definition {
  children: OutlineTree[]
}

export class ScipSolidity {
  private readonly index: Index
  private readonly sources: Source[]

  private readonly fileScip = new Map<string, FileScipSolidityManager>()
  private readonly defMap = new Map<string, Definition>()
  private readonly referencesMap = new Map<string, FileRange[]>()

  constructor(sources: Source[], index: Index) {
    this.index = index
    this.sources = sources

    const documentMap = new Map<string, Document>()
    for (const sym of this.index.externalSymbols) {
      if (sym.symbol) {
        this.defMap.set(sym.symbol, {
          symbol: sym,
          sourcePath: sym.documentation[0] || '', // URL
          signatureHash: extractSighash(sym),
          range: undefined,
          implementations: []
        })
      }
    }

    for (const doc of this.index.documents) {
      documentMap.set(doc.relativePath, doc)
    }

    const implementationMap = new Map<string, Definition[]>()

    for (const source of this.sources) {
      const doc = documentMap.get(source.sourcePath)
      if (!doc) {
        console.error("can't find index for doc", source.sourcePath)
        continue
      }

      const fileScip = new FileScipSolidityManager(source, doc)
      this.fileScip.set(source.sourcePath, fileScip)

      // build symbol map
      for (const sym of doc.symbols) {
        if (sym.symbol) {
          const occur = fileScip.getSymbolOccurrence(sym.symbol)
          if (occur) {
            const def = {
              symbol: sym,
              sourcePath: source.sourcePath,
              signatureHash: extractSighash(sym),
              range: getRange(occur.range),
              implementations: []
            }
            this.defMap.set(sym.symbol, def)

            for (const relation of sym.relationships) {
              if (relation.isImplementation && relation.symbol) {
                let inheritances = implementationMap.get(relation.symbol)
                if (!inheritances) {
                  inheritances = []
                  implementationMap.set(relation.symbol, inheritances)
                }
                inheritances.push(def)
              }
            }
          } else {
            console.error(
              "symbol don't have occurence at",
              sym.symbol,
              source.sourcePath
            )
          }
        } else {
          console.error('empty symbol found at', source.sourcePath)
        }
      }

      // build reference map
      for (const occur of doc.occurrences) {
        let role = SymbolRole.UnspecifiedSymbolRole
        if (hasRole(occur.symbolRoles, SymbolRole.Definition)) {
          role = SymbolRole.Definition
        }
        if (hasRole(occur.symbolRoles, SymbolRole.WriteAccess)) {
          role = SymbolRole.WriteAccess
        }
        if (hasRole(occur.symbolRoles, SymbolRole.ReadAccess)) {
          role = SymbolRole.ReadAccess
        }
        const range = getRange(occur.range)
        let references = this.referencesMap.get(occur.symbol)
        if (!references) {
          references = []
          this.referencesMap.set(occur.symbol, references)
        }
        references.push({
          ...range,
          role,
          sourcePath: doc.relativePath
        })
      }
    }

    for (const definition of this.definitions()) {
      definition.implementations =
        implementationMap.get(definition.symbol.symbol) || []
    }
  }

  getOutline(path: string): OutlineTree[] {
    return this.fileScip.get(path)?.getOutline() || []
  }

  getLine(path: string, line: number) {
    return this.fileScip.get(path)?.getLine(line)
  }

  getLines(path: string) {
    return this.fileScip.get(path)?.getLines()
  }

  getSymbols(path: string): { symbol: SymbolInformation; occur: Occurrence }[] {
    return this.fileScip.get(path)?.getSymbols() || []
  }

  findDefinition(path: string, line: number, character: number) {
    const occur = this.fileScip.get(path)?.locateOccurrence(line, character)
    if (occur && occur.symbol) {
      return this.defMap.get(occur.symbol)
    }
    return undefined
  }

  findReferences(symbol: string): FileReferences {
    const references = [...(this.referencesMap.get(symbol) || [])]
    const def = this.defMap.get(symbol)

    const interfaces: FileRange[] = []
    // iterate all symbol it overrides
    for (const relation of def?.symbol.relationships || []) {
      if (relation.isImplementation && relation.symbol) {
        const relationDef = this.defMap.get(relation.symbol)
        if (relationDef && relationDef.range && relationDef.sourcePath) {
          interfaces.push({
            ...relationDef.range,
            role: SymbolRole.UnspecifiedSymbolRole,
            sourcePath: relationDef.sourcePath
          })
        }
      }
      if (relation.isReference && relation.symbol) {
        const relationRefs = this.referencesMap
          .get(relation.symbol)
          ?.filter((r) => r.role !== SymbolRole.Definition)
        if (relationRefs) {
          references.push(...relationRefs)
        }
      }
    }

    const implementations = []
    // iterate all symbols that overrides it
    for (const inherit of def?.implementations || []) {
      const def = this.defMap.get(inherit.symbol.symbol)
      if (def?.range) {
        implementations.push({
          ...def.range,
          role: SymbolRole.UnspecifiedSymbolRole,
          sourcePath: def.sourcePath
        })
      }
    }

    return {
      references: uniqFileRanges(references),
      interfaces: uniqFileRanges(interfaces),
      implementations: uniqFileRanges(implementations)
    }
  }

  findReferencesByLocation(
    path: string,
    line: number,
    character: number
  ): FileRange[] {
    const occur = this.fileScip.get(path)?.locateOccurrence(line, character)
    if (occur && occur.symbol) {
      return this.referencesMap.get(occur.symbol) || []
    }
    return []
  }

  definitions() {
    const res: Definition[] = []
    for (const def of this.defMap.values()) {
      if (def.symbol.kind === SymbolInformation_Kind.File) {
        continue
      }
      res.push(def)
    }
    return res
  }

  locateOccurrence(
    path: string,
    line: number,
    character: number
  ): Occurrence | undefined {
    return this.fileScip.get(path)?.locateOccurrence(line, character)
  }
}

class FileScipSolidityManager {
  private readonly source: Source
  private readonly document: Document
  private readonly lines: string[]

  private readonly occurrencesByLine: Occurrence[][] = []
  private readonly symbolOccurrence = new Map<string, Occurrence>()

  constructor(source: Source, document: Document) {
    this.source = source
    this.document = document

    this.lines = this.source.source.split('\n')

    for (let i = 0; i < this.lines.length; i++) {
      this.occurrencesByLine.push([])
    }
    for (const occur of this.document.occurrences) {
      const line = occur.range[0]
      this.occurrencesByLine[line].push(occur)
    }
    for (const occurs of this.occurrencesByLine) {
      occurs.sort((a, b) => a.range[1] - b.range[1])
    }

    for (const occur of this.document.occurrences) {
      if (occur.symbol) {
        if (hasRole(occur.symbolRoles, SymbolRole.Definition)) {
          this.symbolOccurrence.set(occur.symbol, occur)
        }
      }
    }
  }

  getLine(line: number) {
    return this.lines[line]
  }

  getLines() {
    return this.lines.join('\n')
  }

  locateOccurrence(line: number, character: number): Occurrence | undefined {
    const occurs = this.occurrencesByLine[line]
    for (const occur of occurs) {
      if (character >= occur.range[1] && character < occur.range[2]) {
        return occur
      }
    }
    return undefined
  }

  getSymbolOccurrence(symbol: string) {
    return this.symbolOccurrence.get(symbol)
  }

  getOutline(): OutlineTree[] {
    const outlines = new Map<string, OutlineTree>()
    const roots: OutlineTree[] = []
    let fileSymbol = ''

    for (const sym of this.document.symbols) {
      if (!sym.symbol || sym.kind === SymbolInformation_Kind.File) {
        if (sym.symbol) {
          fileSymbol = sym.symbol
        }
        continue
      }
      if (
        isLocalSymbol(sym.symbol) &&
        sym.kind === SymbolInformation_Kind.Variable
      ) {
        continue
      }

      const occur = this.getSymbolOccurrence(sym.symbol)
      if (!occur) {
        console.log(sym.symbol, 'has no definition point')
        continue
      }
      const outline: OutlineTree = {
        symbol: sym,
        signatureHash: extractSighash(sym),
        sourcePath: this.source.sourcePath,
        range: getRange(occur.range),
        implementations: [],
        children: []
      }
      outlines.set(sym.symbol, outline)
    }

    for (const [symbol, outline] of outlines.entries()) {
      if (
        outline.symbol.enclosingSymbol &&
        outline.symbol.enclosingSymbol !== fileSymbol
      ) {
        const parent = outlines.get(outline.symbol.enclosingSymbol)
        if (parent) {
          parent.children.push(outline)
        } else {
          console.log('Failed to find parent for', symbol)
        }
      } else {
        roots.push(outline)
      }
    }

    return roots
  }

  getSymbols(): { symbol: SymbolInformation; occur: Occurrence }[] {
    const symbols: { symbol: SymbolInformation; occur: Occurrence }[] = []
    for (const sym of this.document.symbols) {
      if (sym.symbol && !isLocalSymbol(sym.symbol)) {
        const occur = this.getSymbolOccurrence(sym.symbol)
        if (!occur) {
          console.log(sym.symbol, 'has no definition point')
          continue
        }
        symbols.push({ symbol: sym, occur })
      }
    }
    return symbols
  }
}

interface Range {
  start: Position
  end: Position
}

interface Position {
  line: number
  character: number
}

export interface FileRange extends Range {
  sourcePath: string
  role: SymbolRole
}

export interface FileReferences {
  references: FileRange[]
  interfaces: FileRange[] // all symbol it overrides
  implementations: FileRange[] // all symbols that overrides it
}

export function getRange(scipRange: number[]): Range {
  const start: Position = {
    line: scipRange[0],
    character: scipRange[1]
  }
  let end: Position = { line: 0, character: 0 }
  if (scipRange.length == 4) {
    end = {
      line: scipRange[2],
      character: scipRange[3]
    }
  } else {
    end = {
      line: scipRange[0],
      character: scipRange[2]
    }
  }
  return {
    start,
    end
  }
}

function isLocalSymbol(symbol: string): boolean {
  return symbol.startsWith('local')
}

// SymbolRole is a bitmask enum in scip.proto, so the enum values can be
// tested against the symbol_roles field directly.
function hasRole(mask: number, role: SymbolRole) {
  return (mask & role) !== 0
}

export function extractSighash(symbol: SymbolInformation) {
  if (symbol.kind !== SymbolInformation_Kind.Function) {
    return undefined
  }
  if (
    isLocalSymbol(symbol.symbol) ||
    symbol.signatureDocumentation?.text === undefined
  ) {
    return undefined
  }

  return symbol.signatureDocumentation.text.split('\n')[1]
}

function uniqFileRanges(arr: FileRange[]) {
  const getKey = (r: FileRange) =>
    `${r.sourcePath}:${r.start.line}:${r.start.character}:${r.end.line}:${r.end.character}`
  const map = new Map<string, FileRange>()
  for (const r of arr) {
    map.set(getKey(r), r)
  }
  return Array.from(map.values())
}
