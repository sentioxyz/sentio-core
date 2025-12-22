import { SoliditySourceParser } from './SoliditySourceParser'
import { SourceStore } from './SourceStore'

export class CompilationSourceStore extends SourceStore {
  getParser(compilationId: string) {
    const key = `${compilationId}@compilation`
    if (this.parsers.has(key)) {
      return this.parsers.get(key)
    }

    const sourceList = this.getSource(compilationId)
    if (!sourceList) {
      throw new Error('source list is not ready')
    }
    const newInstance = new SoliditySourceParser(
      sourceList,
      undefined,
      undefined,
      undefined,
      undefined,
      this.projectOwner,
      this.projectSlug,
      compilationId,
      this.isForkedChain
    )
    this.parsers.set(key, newInstance)
    return newInstance
  }
}
