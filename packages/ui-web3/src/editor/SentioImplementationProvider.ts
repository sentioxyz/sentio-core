import { Monaco } from '@monaco-editor/react'
import type * as monacoEditor from 'monaco-editor'
import { SoliditySourceParser } from './SoliditySourceParser'

export class SentioImplementationProvider
  implements monacoEditor.languages.ImplementationProvider
{
  private readonly props: any
  private readonly monaco: Monaco

  constructor(props: any, monaco: any) {
    this.props = props
    this.monaco = monaco
  }

  provideImplementation(
    model: monacoEditor.editor.ITextModel,
    position: monacoEditor.Position,
    token: monacoEditor.CancellationToken
  ): monacoEditor.languages.ProviderResult<monacoEditor.languages.Definition> {
    const path = model.uri.path.replace('/', '')
    const address = model.uri.authority
    const parser = this.props.store.getParser(address) as SoliditySourceParser
    return parser.getImplementation(
      this.monaco,
      path,
      position.lineNumber,
      position.column
    )
  }
}
