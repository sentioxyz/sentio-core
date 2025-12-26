import { Monaco } from '@monaco-editor/react'
import type * as monacoEditor from 'monaco-editor'
import { SoliditySourceParser } from './SoliditySourceParser'

export class SentioDocumentSymbolProvider
  implements monacoEditor.languages.DocumentSymbolProvider
{
  private readonly props: any
  private readonly monaco: Monaco

  constructor(props: any, monaco: any) {
    this.props = props
    this.monaco = monaco
  }

  async provideDocumentSymbols(
    model: monacoEditor.editor.ITextModel,
    token: monacoEditor.CancellationToken
  ): Promise<monacoEditor.languages.DocumentSymbol[]> {
    const { address, path } = this.props.parseMonacoUri(model.uri)
    try {
      const parser = this.props.store.getParser(address) as SoliditySourceParser
      return parser.getDocumentSymbols(this.monaco, path)
    } catch {
      return []
    }
  }
}
