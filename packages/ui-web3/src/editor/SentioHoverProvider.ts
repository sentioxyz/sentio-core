import { Monaco } from '@monaco-editor/react'
import type * as monacoEditor from 'monaco-editor'
import { SoliditySourceParser } from './SoliditySourceParser'

export class SentioHoverProvider
  implements monacoEditor.languages.HoverProvider
{
  props: any
  monaco: Monaco
  constructor(props: any, monaco: any) {
    this.props = props
    this.monaco = monaco
  }

  async provideHover(
    model: monacoEditor.editor.ITextModel,
    position: monacoEditor.Position,
    token: monacoEditor.CancellationToken
  ): Promise<monacoEditor.languages.Hover> {
    if (this.props.onHover) {
      return this.props.onHover(model, position, token)
    }
    const { address, path } = this.props.parseMonacoUri(model.uri)
    const parser = this.props.store.getParser(address) as SoliditySourceParser
    const res = parser.getHover(path, position.lineNumber, position.column)
    return res
  }
}
