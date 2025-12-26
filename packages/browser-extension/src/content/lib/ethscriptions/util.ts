export function hexToUTF8(hexString) {
  if (hexString.indexOf('0x') === 0) {
    hexString = hexString.slice(2)
  }

  const bytes = new Uint8Array(hexString.length / 2)

  for (let index = 0; index < bytes.length; index++) {
    const start = index * 2
    const hexByte = hexString.slice(start, start + 2)
    const byte = Number.parseInt(hexByte, 16)
    if (Number.isNaN(byte) || byte < 0)
      throw new Error(`Invalid byte sequence ("${hexByte}" in "${hexString}").`)
    bytes[index] = byte
  }

  const result = new TextDecoder().decode(bytes)
  return result.replace(/\0/g, '')
}

export const dataURIPattern = new RegExp(
  '^data:(?:(?<mediatype>(?<mimetype>.+?/.+?)?(?<parameters>(?:;.+=.+?)*))?(?<extension>;base64)?,(?<data>.*))'
)

export interface DataURI {
  isValid: boolean
  mediaType?: string
  mimeType?: string
  parameters?: Record<string, string>
  base64?: string
  data?: string
}

export function parseDataURI(_dataURI: string): DataURI {
  try {
    const dataURI = _dataURI.replaceAll('\x00', '')
    const match = dataURI.match(dataURIPattern)

    if (match && match.groups && dataURI.startsWith('data:')) {
      let parameters: Record<string, string> | undefined = undefined
      if (match.groups.mediatype) {
        const paramsRegex = /;(?<paramName>[^=;]+)(?:=(?<paramValue>[^;]+))?/g
        let paramMatch
        parameters = {}
        while (
          (paramMatch = paramsRegex.exec(match.groups.mediatype)) !== null
        ) {
          const { paramName, paramValue } = paramMatch.groups as any
          if (paramName) {
            parameters[paramName] = paramValue || ''
          }
        }
      }

      return {
        isValid: true,
        mediaType: match.groups.mediatype || '',
        mimeType: match.groups.mimetype || '',
        parameters,
        base64: match.groups.extension || '',
        data: match.groups.data || ''
      }
    }
  } catch (e) {
    console.log(`failed parse datauri string ${_dataURI}`)
  }
  return { isValid: false }
}

export function parseTransfers(hexString: string) {
  const transfers: string[] = []
  if (hexString.indexOf('0x') === 0) {
    hexString = hexString.slice(2)
    if (hexString.length % 64 === 0 && hexString.length >= 64) {
      for (let i = 0; i < hexString.length; i += 64) {
        transfers.push('0x' + hexString.slice(i, i + 64))
      }
    }
  }
  return transfers.length > 0 ? transfers : undefined
}
