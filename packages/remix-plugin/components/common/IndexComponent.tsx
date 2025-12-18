'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import type { Api } from '@remixproject/plugin-utils'
import { Client } from '@remixproject/plugin'
import {
  IRemixApi,
  CompilationResult as CompilationResultType,
  RemixTxEvent
} from '@remixproject/plugin-api'
import { AstWalker } from '@remix-project/remix-astwalker/src/astWalker'
import { sourceLocationFromAstNode } from '@remix-project/remix-astwalker/src/sourceMappings'
import { EthChainName, EthChainIds, EthChainLogos } from './networks'
import Image from 'next/image'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { Separator } from '../ui/separator'
import { RelatedTransactions } from './RelatedTransactions'
import { sampleCompilationData, sampleFnName } from '@/lib/sample'
import { CopyButton } from './CopyButton'
import { SettingContent } from './Setting'
import {
  CollapsibleTrigger,
  Collapsible,
  CollapsibleContent
} from '@/components/ui/collapsible'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { TransactionTab } from './TransactionTab'
import {
  useGlobalCompilationResultStore,
  useGlobalContractStore
} from '@/lib/use-global-store'
import { FunctionItem } from '@/lib/types'
import { encodeFunctionSignature, flattenTypes } from 'web3-eth-abi'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger
} from '@/components/ui/tooltip'
import RemixClient from './RemixClient'

const walker = new AstWalker()

export const IndexComponent = () => {
  const [client, setClient] = useState<
    Client<Api, Readonly<IRemixApi>> | undefined | null
  >(null)
  const [functions, setFunctions] = useState<FunctionItem[]>([])
  const [network, setNetwork] = useState<string>(
    EthChainIds[EthChainName.ETHEREUM]
  )
  const [showSearchFunction, setShowSearchFunction] = useState('')
  const { addContractItem, allContracts } = useGlobalContractStore()
  const {
    fileName,
    setFileName,
    parseResult,
    addSource,
    addSources,
    sourceMapping,
    compilationResult,
    getMethodBySelector,
    beforeSimulate
  } = useGlobalCompilationResultStore()
  const sourceMappingRef = useRef(sourceMapping)
  sourceMappingRef.current = sourceMapping

  const [transactions, setTransactions] = useState<RemixTxEvent[]>([])

  useEffect(() => {
    if (process.env.NODE_ENV === 'development') {
      // setCompilationData(sampleCompilationData as any)
      parseResult(sampleCompilationData as any)
      setFileName(sampleFnName)
    }
  }, [])

  useEffect(() => {
    if (!fileName || !compilationResult) return
    const astData = compilationResult.ast
    const contracts = compilationResult.contracts
    if (!astData) setFunctions([])
    const functionList: FunctionItem[] = []
    walker.walkFull(astData as any, (node: any) => {
      switch (node.nodeType) {
        case 'ContractDefinition': {
          const childrenNodes = walker.getASTNodeChildren(node)
          childrenNodes.forEach((childNode) => {
            if (childNode.nodeType === 'FunctionDefinition') {
              if (
                childNode.visibility === 'public' &&
                childNode.kind !== 'constructor'
              ) {
                const functionSelector = childNode.functionSelector
                const name = childNode.name
                const functionAbi = contracts[node.name]?.abi.find(
                  (abiItem) => {
                    try {
                      const signature = encodeFunctionSignature(abiItem as any)
                      return signature === `0x${functionSelector}`
                    } catch {
                      return false
                    }
                  }
                )
                let functionName = ''
                if (functionAbi) {
                  try {
                    functionName = `${name}(${flattenTypes(true, functionAbi.inputs as any).join(', ')})`
                  } catch {
                    functionName = ''
                  }
                }
                functionList.push({
                  contractName: node.name,
                  name,
                  src: childNode.src,
                  kind: childNode.kind,
                  location: sourceLocationFromAstNode(childNode),
                  functionSelector,
                  functionName,
                  filePath: compilationResult.path
                })
              }
            }
          })
          break
        }
        case 'FunctionDefinition':
          // if (node.visibility === 'public') {
          //   functionList.push({
          //     name: node.name,
          //     src: node.src,
          //     kind: node.kind,
          //     location: sourceLocationFromAstNode(node)
          //   })
          // }
          break
      }
    })
    setFunctions(functionList)
  }, [fileName, compilationResult])

  useEffect(() => {
    async function load() {
      const client = RemixClient.client
      await RemixClient.onload()
      setClient(client)
      client.fileManager.on('currentFileChanged', (fn: string) => {
        setFileName(fn)
        client.fileManager.readFile(fn).then((content: string) => {
          addSource(fn, content)
        })
        return true
      })
      client.solidity.on(
        'compilationFinished',
        (
          fn: string,
          source: any,
          languageVersion: string,
          data: CompilationResultType
        ) => {
          parseResult(data)
          addSources(source.sources, source.target)
          setFileName(fn)
        }
      )
      client.udapp.on('newTransaction', (tx: RemixTxEvent) => {
        setTransactions((preValue) => {
          if (preValue.filter((item) => item.hash === tx.hash).length > 0)
            return preValue
          return [tx, ...preValue]
        })
      })
    }
    load()
  }, [])

  const isCurrentCompiled = Boolean(compilationResult)
  const fileInfo = useMemo(() => {
    if (!fileName)
      return {
        name: '',
        path: ''
      }
    return {
      name: fileName.split('/').pop() || '',
      path: '/' + fileName.split('/').slice(0, -1).join('/') + '/'
    }
  }, [fileName])

  return (
    <div className="h-screen overflow-auto">
      <Tabs defaultValue="overview">
        <div className="bg-light sticky top-0 z-[1] space-y-2 px-2 py-2 shadow-sm">
          <Collapsible className="space-y-2">
            <div className="flex w-full flex-wrap items-center gap-2">
              {fileName ? (
                <>
                  <div
                    className="flex flex-1 items-center gap-6 overflow-hidden py-2"
                    title={fileName}
                  >
                    <div className="flex-0 inline-flex items-center gap-1">
                      <i className="fa-regular fa-file-code text-dark h-4 w-4"></i>
                      <span className="text-dark text-sm font-semibold">
                        {fileInfo?.name}
                      </span>
                    </div>
                    <span className="shrink-1 ml-auto grow-0 truncate text-xs">
                      {fileInfo.path}
                    </span>
                  </div>
                </>
              ) : (
                <div className="flex flex-1" />
              )}
              <Separator orientation="vertical" className="flex-0 h-3" />
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <CollapsibleTrigger className="flex-0 inline-flex items-center">
                      <i className="fa-solid fa-gear h-4 w-4 cursor-pointer opacity-80 hover:opacity-100"></i>
                    </CollapsibleTrigger>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p className="max-w-[200px]">
                      Set your API keys, target project and setting chain
                      network
                    </p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
            <CollapsibleContent className="space-y-2">
              <SettingContent
                label="API Key"
                placeholder="Input your Sentio API Key"
                storageId="api-key"
              />
              <SettingContent
                label="Project Owner"
                placeholder="Input your project owner here"
                storageId="projectOwner"
              />
              <SettingContent
                label="Project Slug"
                placeholder="Input your project slug here"
                storageId="projectSlug"
              />
              <div className="flex items-center gap-2">
                <label className="w-20 shrink-0 pr-2 pt-2 text-right text-xs font-medium">
                  Network:{' '}
                </label>
                <Select onValueChange={setNetwork} value={network}>
                  <SelectTrigger className="h-8 w-full">
                    <SelectValue placeholder="Choose a network" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectLabel>Network</SelectLabel>
                      {[
                        EthChainName.ETHEREUM,
                        EthChainName.POLYGON,
                        EthChainName.MOONBEAM,
                        EthChainName.ASTAR
                      ].map((networkName) => (
                        <SelectItem
                          value={EthChainIds[networkName]}
                          key={networkName}
                        >
                          <span className="inline-flex items-center justify-between gap-2">
                            <Image
                              src={EthChainLogos[networkName]}
                              className="inline-block align-text-bottom"
                              height={16}
                              width={16}
                              alt={networkName}
                            />
                            {networkName}
                          </span>
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
            </CollapsibleContent>
          </Collapsible>
          <div className="flex w-full items-center justify-center">
            <TabsList>
              <TabsTrigger value="overview">Functions</TabsTrigger>
              <TabsTrigger value="transaction">Transactions</TabsTrigger>
            </TabsList>
          </div>
        </div>
        {isCurrentCompiled ? (
          functions.length === 0 ? (
            <div className="text-light">
              <div className="p-4 text-center">
                <div className="mb-4 text-center">
                  <div className="fa-solid fa-dove text-[50px]"></div>
                  {/* <FontAwesomeIcon icon={faDove} className="inline-block text-[50px]" /> */}
                </div>
                No public method found,
                <br /> try choose another contract
              </div>
            </div>
          ) : (
            <div>
              <TabsContent value="overview">
                <ul className="">
                  {functions.map((item, index) => {
                    return (
                      <li
                        key={index}
                        className="group border-b px-4 py-2 text-sm"
                      >
                        <div className="flex w-full flex-row items-start justify-between gap-x-6 gap-y-2">
                          <div className="inline-flex flex-1 flex-wrap items-center justify-between gap-2">
                            <div
                              className="inline-flex cursor-pointer flex-wrap items-center gap-0.5 hover:underline"
                              onClick={async () => {
                                await client?.editor.discardHighlight()
                                const lineRange =
                                  sourceMappingRef.current?.srcToLineColumnRange(
                                    item.src
                                  )
                                const source = sourceMappingRef.current?.source
                                // if (source && item.location) {
                                //   console.log(source.substr(item.location.start, item.location.length))
                                // }
                                if (lineRange) {
                                  await client?.editor.highlight(
                                    {
                                      start: {
                                        line: lineRange.start.line - 1,
                                        column: lineRange.start.character
                                      },
                                      end: {
                                        line: lineRange.end.line,
                                        column: lineRange.end.character
                                      }
                                    },
                                    fileName,
                                    '#ffd300'
                                  )
                                }
                              }}
                            >
                              <span className="text-dark font-semibold">
                                {item.functionName || `${item.name}()`}
                              </span>
                            </div>
                            <div className="inline-flex items-center gap-2">
                              <label className="mb-0 font-mono text-xs">
                                {item.functionSelector}
                              </label>
                              <CopyButton
                                text={item.functionSelector}
                                size={14}
                                className="invisible group-hover:!visible"
                              />
                            </div>
                          </div>
                          <button
                            className="btn-sm btn btn-secondary"
                            onClick={() => {
                              setShowSearchFunction((preValue) => {
                                if (preValue === item.functionSelector)
                                  return ''
                                return item.functionSelector || ''
                              })
                            }}
                          >
                            Transactions
                          </button>
                        </div>
                        {item.functionSelector &&
                        showSearchFunction === item.functionSelector ? (
                          <RelatedTransactions
                            open={true}
                            chainId={network}
                            methodSignature={item.functionSelector}
                          />
                        ) : null}
                      </li>
                    )
                  })}
                </ul>
              </TabsContent>
              <TabsContent value="transaction">
                <TransactionTab
                  data={allContracts}
                  txnData={transactions}
                  functions={functions}
                  getMethodBySelector={getMethodBySelector}
                  beforeSimulate={beforeSimulate}
                />
              </TabsContent>
            </div>
          )
        ) : (
          <div className="text-info">
            <div className="p-4 text-center">
              <div className="mb-4 text-center">
                <div className="fa-solid fa-dove text-[50px]"></div>
                {/* <FontAwesomeIcon icon={faDove} className="inline-block text-[50px]" /> */}
              </div>
              No compiled contract
            </div>
          </div>
        )}
      </Tabs>
    </div>
  )
}
