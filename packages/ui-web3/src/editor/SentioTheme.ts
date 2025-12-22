import { merge } from 'lodash'
import type * as monacoEditor from 'monaco-editor'

export const sentioTheme: monacoEditor.editor.IStandaloneThemeData = {
  base: 'vs',
  inherit: true,
  rules: [
    { token: '', foreground: '000000', background: 'fffffe' },
    { token: 'invalid', foreground: 'd50789' },
    { token: 'emphasis', fontStyle: 'italic' },
    { token: 'strong', fontStyle: 'bold' },

    { token: 'variable', foreground: '001188' },
    { token: 'variable.predefined', foreground: '4864AA' },
    { token: 'constant', foreground: 'e67802' },
    { token: 'comment', foreground: '008000' },
    { token: 'number', foreground: '0756d5' },
    { token: 'number.hex', foreground: '0756d5' },
    { token: 'regexp', foreground: '800000' },
    { token: 'annotation', foreground: '808080' },
    { token: 'type', foreground: '008080' },

    { token: 'delimiter', foreground: '000000' },
    { token: 'delimiter.html', foreground: '383838' },
    { token: 'delimiter.xml', foreground: '0000FF' },

    { token: 'tag', foreground: '800000' },
    { token: 'tag.id.pug', foreground: '4F76AC' },
    { token: 'tag.class.pug', foreground: '4F76AC' },
    { token: 'meta.scss', foreground: '800000' },
    { token: 'metatag', foreground: 'e00000' },
    { token: 'metatag.content.html', foreground: 'FF0000' },
    { token: 'metatag.html', foreground: '808080' },
    { token: 'metatag.xml', foreground: '808080' },
    { token: 'metatag.php', fontStyle: 'bold' },

    { token: 'key', foreground: '863B00' },
    { token: 'string.key.json', foreground: 'A31515' },
    { token: 'string.value.json', foreground: '0451A5' },

    { token: 'attribute.name', foreground: 'FF0000' },
    { token: 'attribute.value', foreground: '0451A5' },
    { token: 'attribute.value.number', foreground: '8a08ed' },
    { token: 'attribute.value.unit', foreground: '8a08ed' },
    { token: 'attribute.value.html', foreground: '0000FF' },
    { token: 'attribute.value.xml', foreground: '0000FF' },

    { token: 'string', foreground: 'd50789' },
    { token: 'string.html', foreground: '0000FF' },
    { token: 'string.sql', foreground: 'FF0000' },
    { token: 'string.yaml', foreground: '0451A5' },

    { token: 'keyword', foreground: '8a08ed' },
    { token: 'keyword.json', foreground: '0451A5' },
    { token: 'keyword.flow', foreground: 'AF00DB' },
    { token: 'keyword.flow.scss', foreground: '0000FF' },

    { token: 'operator.scss', foreground: '666666' },
    { token: 'operator.sql', foreground: '778899' },
    { token: 'operator.swift', foreground: '666666' },
    { token: 'predefined.sql', foreground: 'C700C7' }
  ],
  colors: {
    'diffEditor.removedLineBackground': '#ffe6e6',
    'diffEditor.insertedLineBackground': '#f5f8ee',
    'editor.selectionBackground': '#CDDDF7',
    'editor.selectionHighlightBackground': '#568CE24D',
    'editor.inactiveSelectionBackground': '#F5F8FD'
  }
}

export const sentioThemeDark: monacoEditor.editor.IStandaloneThemeData = {
  base: 'vs-dark',
  inherit: true,
  rules: [
    { token: '', foreground: 'D4D4D4', background: '1E1E1E' },
    { token: 'invalid', foreground: 'f44747' },
    { token: 'emphasis', fontStyle: 'italic' },
    { token: 'strong', fontStyle: 'bold' },

    { token: 'variable', foreground: '74B0DF' },
    { token: 'variable.predefined', foreground: '4864AA' },
    { token: 'variable.parameter', foreground: '9CDCFE' },
    { token: 'constant', foreground: 'C07EE9' },
    { token: 'comment', foreground: '608B4E' },
    { token: 'number', foreground: 'B5CEA8' },
    { token: 'number.hex', foreground: '5BB498' },
    { token: 'regexp', foreground: 'B46695' },
    { token: 'annotation', foreground: 'cc6666' },
    { token: 'type', foreground: 'D25FAB' },

    { token: 'delimiter', foreground: 'DCDCDC' },
    { token: 'delimiter.html', foreground: '808080' },
    { token: 'delimiter.xml', foreground: '808080' },

    { token: 'tag', foreground: 'C07EE9' },
    { token: 'tag.id.pug', foreground: '4F76AC' },
    { token: 'tag.class.pug', foreground: '4F76AC' },
    { token: 'meta.scss', foreground: 'A79873' },
    { token: 'meta.tag', foreground: 'CE9178' },
    { token: 'metatag', foreground: 'DD6A6F' },
    { token: 'metatag.content.html', foreground: '9CDCFE' },
    { token: 'metatag.html', foreground: 'C07EE9' },
    { token: 'metatag.xml', foreground: 'C07EE9' },
    { token: 'metatag.php', fontStyle: 'bold' },

    { token: 'key', foreground: '9CDCFE' },
    { token: 'string.key.json', foreground: '9CDCFE' },
    { token: 'string.value.json', foreground: 'CE9178' },

    { token: 'attribute.name', foreground: '9CDCFE' },
    { token: 'attribute.value', foreground: 'CE9178' },
    { token: 'attribute.value.number.css', foreground: 'B5CEA8' },
    { token: 'attribute.value.unit.css', foreground: 'B5CEA8' },
    { token: 'attribute.value.hex.css', foreground: 'D4D4D4' },

    { token: 'string', foreground: 'CE9178' },
    { token: 'string.sql', foreground: 'FF0000' },

    { token: 'keyword', foreground: 'C07EE9' },
    { token: 'keyword.flow', foreground: 'C586C0' },
    { token: 'keyword.json', foreground: 'CE9178' },
    { token: 'keyword.flow.scss', foreground: 'C07EE9' },

    { token: 'operator.scss', foreground: '909090' },
    { token: 'operator.sql', foreground: '778899' },
    { token: 'operator.swift', foreground: '909090' },
    { token: 'predefined.sql', foreground: 'FF00FF' }
  ],
  colors: {
    'editor.foreGround': '#D4D4D4',
    'editor.background': '#1E1E1E',
    'diffEditor.removedLineBackground': '#d73a4930',
    'diffEditor.insertedLineBackground': '#28a74530'
  }
}

export const setSentioTheme = (
  monaco?: any,
  overrideTheme: any = {},
  overrideDarkTheme: any = {}
) => {
  if (!monaco) {
    return
  }
  monaco.editor.defineTheme('sentio', merge(sentioTheme, overrideTheme))
  monaco.editor.defineTheme(
    'sentio-dark',
    merge(sentioThemeDark, overrideDarkTheme || overrideTheme)
  )
  if (document.body.classList.contains('dark')) {
    monaco.editor.setTheme('sentio-dark')
  } else {
    monaco.editor.setTheme('sentio')
  }
}
