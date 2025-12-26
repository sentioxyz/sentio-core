import { FetchAndCompileResponse } from './types'
import { ScipSolidity, Source } from '@sentio/scip'
import { SoliditySourceParser } from './SoliditySourceParser'

function getKey(address: string, networkId: string) {
  return `${address.toLowerCase()}@${networkId}`
}

function parseKey(key: string) {
  const [address, networkId] = key.split('@')
  return { address, networkId }
}

export class SourceStore {
  readonly sources: Map<string, Source[]>
  readonly parsers: Map<string, SoliditySourceParser>
  readonly networkId: string
  readonly simulationId?: string
  readonly projectOwner?: string
  readonly projectSlug?: string
  readonly isForkedChain?: boolean
  constructor(
    data: FetchAndCompileResponse,
    networkId: string,
    private readonly scipInstance?: Promise<ScipSolidity>,
    private readonly simId?: string,
    projectOwner?: string,
    projectSlug?: string,
    isForkedChain?: boolean
  ) {
    this.sources = new Map()
    this.parsers = new Map()
    this.networkId = networkId
    this.simulationId = simId
    this.projectOwner = projectOwner
    this.projectSlug = projectSlug
    this.isForkedChain = isForkedChain
    this.parseSource(data, networkId)
  }

  private parseSource(data?: FetchAndCompileResponse, networkId?: string) {
    if (!networkId || !data) {
      return
    }

    const { result } = data
    result?.forEach((item) => {
      const { address, sources } = item
      const key = getKey(address, networkId)
      this.sources.set(
        key,
        sources.map((source) => {
          return {
            source: source.source,
            sourcePath: source.sourcePath
          } as Source
        })
      )
    })
  }

  getSource(address: string) {
    const key = getKey(address, this.networkId)
    return this.sources.get(key)
  }

  getParser(address: string) {
    const key = getKey(address, this.networkId)
    if (this.parsers.has(key)) {
      return this.parsers.get(key)
    }
    const sourceList = this.getSource(address)
    if (!sourceList) {
      throw new Error('source list is not ready')
    }
    const newInstance = new SoliditySourceParser(
      sourceList,
      this.networkId,
      address,
      this.simulationId,
      this.scipInstance,
      this.projectOwner,
      this.projectSlug,
      undefined,
      this.isForkedChain
    )
    this.parsers.set(key, newInstance)
    return newInstance
  }

  findTextOccurrences(searchTerm: string): TextOccurrence[] {
    const occurrences: TextOccurrence[] = []
    const s = searchTerm.toLowerCase()
    for (const key of this.sources.keys()) {
      for (const source of this.sources.get(key) || []) {
        const lines = source.source.split('\n')

        lines.forEach((line, lineNumber) => {
          const lowerLine = line.toLowerCase()
          const index = lowerLine.indexOf(s)
          // find first occurrence
          if (index !== -1) {
            const { address } = parseKey(key)
            occurrences.push({
              sourcePath: source.sourcePath,
              lineNumber: lineNumber + 1,
              start: index + 1,
              line,
              address: address
            })
          }
        })
      }
    }

    return occurrences
  }
}

export interface TextOccurrence {
  line: string
  lineNumber: number
  start: number
  sourcePath: string
  address: string
}
