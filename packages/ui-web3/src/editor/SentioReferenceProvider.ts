import { Monaco } from '@monaco-editor/react'
import type * as monacoEditor from 'monaco-editor'
import { SoliditySourceParser } from './SoliditySourceParser'

export class SentioReferenceProvider
  implements monacoEditor.languages.ReferenceProvider
{
  private readonly props: any
  private readonly monaco: Monaco

  constructor(props: any, monaco: any) {
    this.props = props
    this.monaco = monaco
  }

  provideReferences(
    model: monacoEditor.editor.ITextModel,
    position: monacoEditor.Position,
    context: monacoEditor.languages.ReferenceContext,
    token: monacoEditor.CancellationToken
  ): monacoEditor.languages.ProviderResult<monacoEditor.languages.Location[]> {
    const { address, path } = this.props.parseMonacoUri(model.uri)
    const parser = this.props.store.getParser(address) as SoliditySourceParser
    return parser.getImplementation(
      this.monaco,
      path,
      position.lineNumber,
      position.column
    )
  }
}
