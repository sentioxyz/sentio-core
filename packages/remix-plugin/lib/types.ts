import { Location } from '@remix-project/remix-astwalker'

export type FunctionItem = {
  contractName: string
  name: string
  src: string
  kind: string
  location: Location | null
  filePath: string
  functionSelector?: string
  functionName?: string
}

export type CompileSpecType =
  | {
      solidityVersion: string
      contractName: string
      constructorArgs: string
      multiFile: {
        source: Record<string, string>
        compilerSettings: string
      }
    }
  | {}
