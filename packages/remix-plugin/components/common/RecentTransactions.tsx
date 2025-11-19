import { PropsWithChildren, useCallback, useMemo, useState } from 'react'
import { RemixTxEvent } from '@remixproject/plugin-api'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '../ui/badge'
import { getMethodSignature, trimTxnHash, uuid } from '@/lib/utils'
import { CopyButton } from './CopyButton'
import { API_HOST } from '@/lib/host'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { CollapsibleTrigger, Collapsible, CollapsibleContent } from '@/components/ui/collapsible'
import { FunctionItem, CompileSpecType } from '@/lib/types'
import { SignatureMapItem } from '@/lib/use-global-store'
import { uploadToSentioCompilation } from '@/lib/use-compilation'
import { simulateTransaction } from '@/lib/use-simulation'
import { useToast } from '@/components/ui/use-toast'
import { ToastAction } from '../ui/toast'
import { DEFAULT_SETTINGS } from '@/lib/default-keys'

interface Props {
  data: RemixTxEvent[]
  functions?: FunctionItem[]
  getMethodBySelector?: (selector: string) => SignatureMapItem[] | undefined
  beforeSimulate?: (targetPath: string, targetContract: string) => CompileSpecType
}

function DisplayItem({
  label,
  value,
  copyable,
  children
}: PropsWithChildren<{ label?: string; value?: string; copyable?: boolean }>) {
  return (
    <div className="grid grid-cols-4 items-center gap-x-4">
      <div className="text-info truncate whitespace-nowrap text-xs font-medium">{label}:</div>
      <div className="group col-span-3 flex items-center gap-2">
        <div className="overflow-hidden">
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger className="text-info w-full truncate font-mono text-xs">{value}</TooltipTrigger>
              <TooltipContent>
                <p className="max-w-[60vw] whitespace-pre-wrap break-words">{value}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
        {copyable && <CopyButton text={value} className="text-info invisible group-hover:!visible" />}
        {children}
      </div>
    </div>
  )
}

function DisplayMore({ children }: React.PropsWithChildren<{}>) {
  const [open, setOpen] = useState(false)
  return (
    <Collapsible onOpenChange={setOpen}>
      <CollapsibleTrigger className="w-full cursor-pointer text-right text-xs opacity-80 hover:underline hover:opacity-100">
        {open ? 'Show Less' : 'Show More'}
      </CollapsibleTrigger>
      <CollapsibleContent>{children}</CollapsibleContent>
    </Collapsible>
  )
}

function safeParseLocalStorage(key: string) {
  const value = window.localStorage.getItem(key)
  if (value) {
    try {
      return JSON.parse(value)
    } catch {
      return ''
    }
  }
  return ''
}

function TransactionCard({
  data,
  beforeSimulate
}: {
  data: any
  beforeSimulate?: (targetPath: string, targetContract: string) => CompileSpecType
}) {
  const { envMode } = data
  const isVm = envMode.startsWith('vm')
  const { toast } = useToast()
  const onSimulate = useCallback(
    async (data: any) => {
      const compileSpec = beforeSimulate?.(data.methodPath, data.methodContract)
      let apiKey = safeParseLocalStorage('api-key')
      let projectOwner = safeParseLocalStorage('projectOwner')
      let projectSlug = safeParseLocalStorage('projectSlug')

      if (!apiKey && !projectOwner && !projectSlug) {
        apiKey = DEFAULT_SETTINGS['api-key']
        projectOwner = DEFAULT_SETTINGS['projectOwner']
        projectSlug = DEFAULT_SETTINGS['projectSlug']
      }

      const chainId = data.chainId.toString()

      if (!apiKey) {
        toast({
          title: 'API Key is missing',
          description: 'Please set the API Key in the settings panel'
        })
        return
      }

      if (!projectOwner || !projectSlug) {
        toast({
          title: 'Project information is missing',
          description: 'Please set the project information in the setting panel'
        })
        return
      }

      try {
        if (compileSpec && Object.keys(compileSpec)) {
          toast({
            title: 'Start simulation',
            description: 'Please wait for the simulation to finish'
          })

          let res: any = null
          res = await uploadToSentioCompilation(
            {
              name: `remix-${chainId}-${data.methodContract}-${uuid()}`,
              projectOwner,
              projectSlug,
              compileSpec
            },
            apiKey
          )
          if (!res || !res.userCompilationId) {
            toast({
              title: 'Upload file failed'
            })
            return
          }
          const contractAddress = data.to as string
          res = await simulateTransaction(
            {
              projectOwner,
              projectSlug,
              simulation: {
                networkId: chainId,
                blockNumber: data.blockNumber.toString(),
                transactionIndex: data.transactionIndex.toString(),
                from: data.from,
                to: contractAddress,
                gas: data.gas.toString(),
                gasPrice: data.gasPrice.toString(),
                value: data.value.toString(),
                input: data.data,
                originTxHash: data.hash,
                sourceOverrides: {
                  [contractAddress]: res.userCompilationId
                }
              }
            },
            apiKey
          )
          if (res.simulation?.id) {
            toast({
              title: 'Simulation finished',
              description: 'Please check the result in Sentio',
              action: (
                <ToastAction
                  altText="view"
                  onClick={() => {
                    window.open(
                      `${API_HOST}/${projectOwner}/${projectSlug}/simulator/${chainId}/${res.simulation.id}`,
                      '_blank'
                    )
                  }}
                >
                  View
                </ToastAction>
              )
            })
          }
        }
      } catch {
        toast({
          title: 'Simulation failed',
          description: 'Please check the API Key in the settings page'
        })
      }
    },
    [beforeSimulate, toast]
  )

  if (isVm) {
    return (
      <Card className="bg-light">
        <CardHeader>
          <CardTitle className="truncate">
            <div className="flex w-full items-center justify-between">
              <span className="text-primary group font-mono">
                {trimTxnHash(data.hash)}
                <CopyButton text={data.hash} ml={8} className="invisible group-hover:!visible" />
              </span>
              <span className="inline-flex gap-2">
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger>
                      <Badge variant="outline">{data.chainId.toString()}</Badge>
                    </TooltipTrigger>
                    <TooltipContent>
                      <p className="max-w-[60vw] whitespace-pre-wrap break-words">
                        Chain ID: {data.chainId.toString()}
                      </p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger>
                      <Badge variant="secondary">VM</Badge>
                    </TooltipTrigger>
                    <TooltipContent>
                      <p className="max-w-[60vw] whitespace-pre-wrap break-words">
                        This is a transaction in Remix Virtual Machine, it is not on the real blockchain.
                      </p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </span>
            </div>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div>
            <DisplayItem label="Status" value={data.status == 0 ? 'Failed' : 'Succeed'} />
            <DisplayItem label="From" value={data.from} copyable />
            {data.to && <DisplayItem label="To" value={data.to} copyable />}
            {data.to && data.data && (
              <>
                <DisplayItem label="Method" value={data.methodName || getMethodSignature(data.data)} copyable>
                  {data.ambiguity && <Badge variant="secondary">Ambiguity</Badge>}
                </DisplayItem>
              </>
            )}
            <DisplayMore>
              <>
                {data.methodName && getMethodSignature(data.data) && (
                  <DisplayItem label="Method Signature" value={getMethodSignature(data.data)} copyable />
                )}
                {data.data && <DisplayItem label="Input" value={data.data} copyable />}
                <DisplayItem label="Block" value={data.blockNumber.toString()} copyable />
                <DisplayItem label="Gas" value={data.gas.toString()} copyable />
                <DisplayItem label="Gas Price" value={data.gasPrice.toString()} copyable />
              </>
            </DisplayMore>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="truncate">
          <div className="flex w-full items-center justify-between">
            <span className="text-primary group font-mono">
              {trimTxnHash(data.hash)}
              <CopyButton text={data.hash} ml={8} className="invisible group-hover:!visible" />
            </span>
            <span className="inline-flex flex-wrap justify-end gap-2">
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger>
                    <Badge variant="outline">{data.chainId.toString()}</Badge>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p className="max-w-[60vw] whitespace-pre-wrap break-words">Chain ID: {data.chainId.toString()}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
              {data.to ? null : <Badge variant="secondary">Contract Creation</Badge>}
            </span>
          </div>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div>
          <DisplayItem label="Status" value={data.status == 0 ? 'Failed' : 'Succeed'} />
          <DisplayItem label="From" value={data.from} copyable />
          {data.to && <DisplayItem label="To" value={data.to} copyable />}
          {data.to && data.data && (
            <DisplayItem label="Method" value={data.methodName || getMethodSignature(data.data)} copyable>
              {data.ambiguity && <Badge variant="secondary">Ambiguity</Badge>}
            </DisplayItem>
          )}
          <DisplayMore>
            <>
              {data.methodName && getMethodSignature(data.data) && (
                <DisplayItem label="Method Signature" value={getMethodSignature(data.data)} copyable />
              )}
              {data.data && <DisplayItem label="Input" value={data.data} copyable />}
              <DisplayItem label="Block Number" value={data.blockNumber.toString()} copyable />
              <DisplayItem label="Gas" value={data.gas.toString()} copyable />
              <DisplayItem label="Gas Price" value={data.gasPrice.toString()} copyable />
            </>
          </DisplayMore>
        </div>
      </CardContent>
      <CardFooter className="space-x-4">
        <button
          className="btn-sm btn btn-secondary"
          onClick={() => {
            window.open(`${API_HOST}/tx/${data.chainId.toString()}/${data.hash}`, '_blank')
          }}
        >
          View in Sentio
        </button>
        {data.to && (
          <button
            className="btn-sm btn btn-primary"
            onClick={() => {
              onSimulate(data)
            }}
          >
            Simulate in Sentio
          </button>
        )}
      </CardFooter>
    </Card>
  )
}

export const RecentTransactions = ({ data, functions, getMethodBySelector, beforeSimulate }: Props) => {
  const transactions = useMemo(() => {
    return data?.map((tx) => {
      const signature = getMethodSignature((tx as any).data)
      const methodABI = getMethodBySelector?.(signature || '')
      if (methodABI?.length) {
        const method = methodABI[methodABI.length - 1]
        return {
          ...tx,
          methodName: method.name,
          methodContract: method.contractName,
          methodPath: method.file,
          ambiguity: methodABI.length > 1
        }
      }
      return tx
    })
  }, [getMethodBySelector, data])
  return (
    <div className="pt-2">
      <ul className="space-y-2 p-2">
        {transactions.map((tx) => {
          return (
            <li key={tx.hash}>
              <TransactionCard data={tx} beforeSimulate={beforeSimulate} />
            </li>
          )
        })}
        {data.length === 0 && (
          <div>
            <Card>
              <CardHeader>
                <CardTitle>No transactions found</CardTitle>
                <CardDescription>There are no transactions to display, use Remix to transact first.</CardDescription>
              </CardHeader>
            </Card>
          </div>
        )}
      </ul>
    </div>
  )
}
