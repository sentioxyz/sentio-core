import { useCallback, useEffect, useMemo, useState, useContext } from 'react'
import Drawer from '@mui/material/Drawer'
import { useTransactionInfo } from '~/content/lib/debug/use-transaction-info'
import { sentioProjectSimulatorUrl } from '~/utils/url'
import { useSetAtom } from 'jotai'
import {
  OpenSimulationContext,
  IsSimulationContext
} from '~/content/lib/context/transaction'
import { useSimulator } from '~/content/lib/debug/use-simulator'
import { BigDecimal } from '@sentio/bigdecimal'
import map from 'lodash/map'
import {
  NewSimulation,
  SvgFolderContext,
  chainIdToNumber,
  parseHex,
  getNumberWithDecimal,
  getNativeToken,
  contractAddress,
  contractNetwork
} from '@sentio/ui-web3'
import { SimulateProvider } from './SimulateProvider'

const logo = new URL(chrome.runtime.getURL('images/logo.png')).toString()

const BD = BigDecimal.clone({
  EXPONENTIAL_AT: [-30, 30]
})

interface Props {
  hash: string
  chainId: string
}

export const SimulateSlideOver = ({ hash, chainId }: Props) => {
  const [open, setOpen] = useState(false)
  const setAddress = useSetAtom(contractAddress)
  const setNetwork = useSetAtom(contractNetwork)
  const [simId, setSimId] = useState('')
  const [simHref, setSimHref] = useState('')
  const [defaultProject, setDefaultProject] = useState<string>('')
  const [render, setRender] = useState(false)
  const isSimulation = useContext(IsSimulationContext)
  const { data: simulator } = useSimulator(isSimulation ? hash : undefined)

  useEffect(() => {
    if (chainId) {
      setNetwork(chainId)
      chrome.runtime
        .sendMessage({
          api: 'GetChainConfig'
        })
        .then((config) => {
          setRender(config[chainId] && config[chainId].simulator)
        })
    }
  }, [chainId, setNetwork])

  const { data, loading: transactionLoading } = useTransactionInfo(
    hash,
    chainId
  )
  const defaultValue = useMemo(() => {
    if (!data) {
      return undefined
    }
    const chainId = chainIdToNumber(data.transaction?.chainId)?.toString()
    const nativeToken = getNativeToken(chainId)
    const res: any = {
      blockNumber: Number(parseHex(data.block?.number)),
      txIndex: Number(parseHex(data.transaction?.transactionIndex)),
      from: data.transaction?.from,
      input: data.transaction?.input,
      value:
        (getNumberWithDecimal(
          data.transaction?.value,
          nativeToken?.tokenDecimals || 18
        ) as string) ?? '0',
      gas: parseHex(data.transaction?.gas).toString(),
      gasPrice:
        (getNumberWithDecimal(data.transaction?.gasPrice, 9) as string) ?? '0',
      contract: {
        address: data.transaction?.to,
        chainId: chainId
      },
      accessList: data.transaction?.accessList
    }

    const sourceOverrides: any[] = []
    const simData = simulator?.simulation
    if (simData?.sourceOverrides) {
      for (const [key, value] of Object.entries(simData.sourceOverrides)) {
        sourceOverrides.push({
          compilationId: value,
          address: key
        })
      }
      res.sourceOverrides = sourceOverrides
    }
    if (simData?.stateOverrides) {
      const stateOverrides: any[] = []
      for (const [contract, params] of Object.entries(simData.stateOverrides)) {
        const { balance, state } = params as any
        stateOverrides.push({
          contract,
          balance: balance ? BD(balance).toString() : undefined,
          storage: map(state, (value, key) => ({
            key,
            value
          })),
          customBalance: balance ? true : false,
          customContract: true
        })
      }
      res.stateOverride = stateOverrides
    }
    return res
  }, [data, simulator])

  useEffect(() => {
    if (data?.transaction?.to) {
      setAddress(data.transaction.to)
    }
  }, [data?.transaction?.to, setAddress])

  useEffect(() => {
    chrome.storage.sync.get('project').then((data) => {
      setDefaultProject((data.project as string) || '')
    })
    chrome.storage.onChanged.addListener((changes, namespace) => {
      if (namespace === 'sync' && changes.project) {
        setDefaultProject((changes.project.newValue as string) || '')
      }
    })
  }, [])

  const onRequestAPI = useCallback(async (request) => {
    // inject apiKey, project owner and slug
    const res = await chrome.runtime.sendMessage({
      api: 'SimulateTransaction',
      data: request
    })
    if (res.code && res.message) {
      throw {
        body: {
          ...res
        }
      }
    }
    return res
  }, [])

  const onOpenSimulation = useContext(OpenSimulationContext)

  if (!render) {
    return null
  }

  return (
    <SimulateProvider>
      <SvgFolderContext.Provider value="https://app.sentio.xyz/">
        <div>
          <a
            className="text-icontent text-gray hover:border-primary hover:text-primary inline-block rounded border px-2 py-0.5"
            role="button"
            onClick={() => {
              setOpen(true)
            }}
          >
            <i className="far fa-plus mr-1" />
            Simulate in
            <img
              style={{ marginRight: 4 }}
              src={logo}
              alt="sentio-logo"
              width="14"
              height="14"
              className="mx-1 inline-block"
            />
            Sentio
          </a>
          {simHref ? (
            <a
              className="ml-4 rounded border border-gray-100 bg-gray-50 px-2 py-1"
              href={simHref}
              target="_blank"
              rel="noreferrer"
            >
              Last simulation: #{simId}
            </a>
          ) : null}
          <Drawer
            anchor="right"
            open={open}
            onClose={() => setOpen(false)}
            className="_sentio_"
            hideBackdrop
            classes={{
              paper: 'text-gray-600'
            }}
          >
            <div
              className="dark:bg-sentio-gray-100 flex h-full flex-col bg-white sm:min-w-[600px]"
              key={open ? hash + '_open' : hash + '_hidden'}
            >
              <div className="flex-0 flex w-full justify-between border-b border-gray-100 px-4 py-2">
                <div className="inline-flex items-center gap-2">
                  <span className="text-base font-bold">New Simulation</span>
                  {defaultProject && (
                    <span className="text-gray space-x-1 text-xs">
                      <span>{'(under your project'}</span>
                      <a
                        href={sentioProjectSimulatorUrl(defaultProject)}
                        target="_blank"
                        rel="noreferrer"
                        className="hover:underline"
                      >
                        {defaultProject}
                      </a>
                      <span>{')'}</span>
                    </span>
                  )}
                </div>
                <div
                  className="hover:text-primary cursor-pointer text-lg"
                  onClick={() => {
                    setOpen(false)
                  }}
                >
                  <i className="fa-solid fa-xmark fal fa-times"></i>
                </div>
              </div>
              <div className="flex-1 basis-0 overflow-auto">
                {open ? (
                  <NewSimulation
                    defaultValue={defaultValue}
                    onClose={() => {
                      setOpen(false)
                    }}
                    onSuccess={(res) => {
                      if (res.simulation?.id) {
                        setSimId(res.simulation.id)
                        const link = onOpenSimulation(res as any)
                        if (link) {
                          setSimHref(link)
                        }
                      }
                    }}
                    onRequestAPI={onRequestAPI}
                    hideProjectSelect
                  />
                ) : null}
              </div>
            </div>
          </Drawer>
        </div>
      </SvgFolderContext.Provider>
    </SimulateProvider>
  )
}
