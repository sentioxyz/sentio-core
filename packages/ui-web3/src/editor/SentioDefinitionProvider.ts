import { Monaco } from '@monaco-editor/react'
import type * as monacoEditor from 'monaco-editor'
import { SoliditySourceParser } from './SoliditySourceParser'

export class SentioDefinitionProvider
  implements monacoEditor.languages.DefinitionProvider
{
  private readonly props: any
  private readonly monaco: Monaco
  constructor(props: any, monaco: any) {
    this.props = props
    this.monaco = monaco
  }

  async provideDefinition(
    model: monacoEditor.editor.ITextModel,
    position: monacoEditor.Position,
    token: monacoEditor.CancellationToken
  ): Promise<monacoEditor.languages.Definition> {
    const { address, path } = this.props.parseMonacoUri(model.uri)
    const parser = this.props.store.getParser(address) as SoliditySourceParser
    return parser.getDefinition(
      this.monaco,
      path,
      position.lineNumber,
      position.column
    )
  }
}
