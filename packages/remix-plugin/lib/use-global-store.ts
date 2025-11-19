import { CompiledContract, CompilationSource, CompilationResult } from '@remixproject/plugin-api'
import { useCallback, useMemo, useRef, useState } from 'react'
import { SourceMappings } from '@remix-project/remix-astwalker'
import { encodeFunctionSignature, flattenTypes } from 'web3-eth-abi'
import { CompileSpecType } from './types'

function uuid() {
  return Math.random().toString(16).slice(2)
}

export interface ContractItemType {
  id: string
  file: string
  name: string
  data: CompiledContract
}

export const useGlobalContractStore = () => {
  const ContractMapRef = useRef(new Set<ContractItemType>())
  const [version, setVersion] = useState(0)

  const addContractItem = useCallback(function addContractItem(item: Omit<ContractItemType, 'id'>) {
    const ContractMap = ContractMapRef.current
    const targetItem = Array.from(ContractMap).find((i) => i.file === item.file && i.name === item.name)
    if (!targetItem) {
      ContractMap.add({
        id: uuid(),
        ...item
      })
    } else {
      targetItem.data = item.data
    }
    setVersion((v) => v + 1)
  }, [])

  const getContractItem = useCallback(function getContractItem(file: string, name: string) {
    const ContractMap = ContractMapRef.current
    return Array.from(ContractMap).find((i) => i.file === file && i.name === name)
  }, [])

  // eslint-disable-next-line react-hooks/exhaustive-deps
  const allContracts = useMemo(() => Array.from(ContractMapRef.current), [version])

  return {
    addContractItem,
    getContractItem,
    allContracts
  }
}

type CompilationItemType = {
  path: string
  contracts: {
    [contract: string]: CompiledContract
  }
  ast: CompilationSource['ast']
}

export type SignatureMapItem = {
  contractName: string
  file: string
  name: string
}

export const useGlobalCompilationResultStore = () => {
  const compilationsRef = useRef(new Map<string, CompilationItemType>())
  const sourcesRef = useRef(new Map<string, string>())
  const sourcesRelationRef = useRef(new Map<string, string[]>())
  const signaturesRef = useRef(new Map<string, SignatureMapItem[]>())
  const metadataRef = useRef(new Map<string, any>())
  const [version, setVersion] = useState(0)
  const [fileName, setFileName] = useState<string>('')

  const parseResult = useCallback(function addStoreItem(data: CompilationResult) {
    const store = compilationsRef.current
    const { contracts, sources } = data
    const pathKeys = Object.keys(contracts)
    for (const path of pathKeys) {
      const ast = sources[path].ast
      Object.keys(contracts[path]).forEach((contract) => {
        const abi = contracts[path][contract].abi
        abi.forEach((abiItem: any) => {
          try {
            const key = encodeFunctionSignature(abiItem as any)
            const name = `${abiItem.name} (${flattenTypes(true, abiItem.inputs).join(',')})`
            const signatureMap = signaturesRef.current.get(key) || []
            // check if the signature already exists
            const exist = signatureMap.find(
              (item) => item.contractName === contract && item.file === path && item.name === name
            )
            if (exist) return
            signatureMap.push({
              contractName: contract,
              file: path,
              name
            })
            signaturesRef.current.set(key, signatureMap)
          } catch {}
        })
        const metadata = contracts[path][contract].metadata
        if (metadata) {
          metadataRef.current.set(`${path}:${contract}`, metadata)
        }
      })
      store.set(path, {
        path,
        contracts: contracts[path],
        ast
      })
    }
    setVersion((v) => v + 1)
  }, [])

  const addSource = useCallback(function addSource(path: string, content: string) {
    sourcesRef.current.set(path, content)
    setVersion((v) => v + 1)
  }, [])

  const addSources = useCallback(function addSources(sources: Record<string, { content: string }>, target: string) {
    const sourceList: string[] = []
    for (const path in sources) {
      sourcesRef.current.set(path, sources[path].content)
      sourceList.push(path)
    }
    sourcesRelationRef.current.set(target, sourceList)
    setVersion((v) => v + 1)
  }, [])

  const getStoreItem = useCallback(function getStoreItem(path: string) {
    return compilationsRef.current.get(path)
  }, [])

  // eslint-disable-next-line react-hooks/exhaustive-deps
  const compilationResult = useMemo(() => getStoreItem(fileName), [version, fileName])
  const sourceMapping = useMemo(() => {
    const sourceContent = sourcesRef.current.get(fileName)
    if (!sourceContent) return
    return new SourceMappings(sourceContent)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [version, fileName])

  const getMethodBySelector = useCallback(
    (selector: string) => {
      return signaturesRef.current.get(selector)
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [version]
  )

  const beforeSimulate = useCallback(function getCompilationSources(targetPath: string, targetContract: string) {
    const sourceFiles = sourcesRelationRef.current.get(targetPath)
    const metadata = metadataRef.current.get(`${targetPath}:${targetContract}`)
    if (!sourceFiles || !metadata) return {}
    try {
      const metadataJson = JSON.parse(metadata)
      const res = {
        solidityVersion: metadataJson.compiler.version,
        contractName: targetContract,
        constructorArgs: '',
        multiFile: {
          source: {} as Record<string, string>,
          compilerSettings: JSON.stringify({
            remappings: metadataJson.settings.remappings,
            optimizer: metadataJson.settings.optimizer
          })
        }
      }
      for (const path of sourceFiles) {
        const content = sourcesRef.current.get(path)
        if (content) {
          res.multiFile.source[path] = content
        }
      }
      return res
    } catch {
      return {}
    }
  }, [])

  return {
    parseResult,
    fileName,
    setFileName,
    compilationResult,
    addSource,
    addSources,
    sourceMapping,
    getMethodBySelector,
    beforeSimulate
  }
}
