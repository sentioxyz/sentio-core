import { API_HOST } from './host'
import { CompileSpecType } from './types'

export type UploadUserCompilationRequest = {
  name?: string
  projectOwner?: string
  projectSlug?: string
  compileSpec?: CompileSpecType
  // fromContract?: {
  //   networkId?: string
  //   address?: string
  //   overrideSource?: { [key: string]: string }
  // }
}

export async function uploadToSentioCompilation(data: UploadUserCompilationRequest, apiKey: string) {
  try {
    if (!apiKey) {
      throw new Error('API Key is required')
    }
    const res = await fetch(`${API_HOST}/api/v1/solidity/user_compilation`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'api-key': apiKey
      },
      body: JSON.stringify(data)
    })
    if (!res.ok) {
      throw new Error('Failed to upload compilation')
    }
    return await res.json()
  } catch (error) {
    return {
      error
    }
  }
}
