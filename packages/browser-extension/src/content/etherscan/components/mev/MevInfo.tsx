import { createExternalExtensionProvider } from '@metamask/providers'
import { useEffect, useRef, useState } from 'react'
import {
  MevInfo as SentioMevInfo,
  Button,
  FlashbotIcon,
  MevType
} from '@sentio/ui-web3'

const metaMaskIds = {
  stable: 'nkbihfbeogaeaoehlefnkodbefgpgknn',
  beta: 'pbbkamfgmaedccnfkmjcofcecjhfgldn',
  flask: 'ljfoeinjpaedjfecbmggjgodbgkmjkjk'
}

interface Props {
  hash?: string
  chainId?: string
}

export const MevInfo = ({ hash, chainId }: Props) => {
  const [isMetaMask, setIsMetaMask] = useState(false)
  const providerRef = useRef<any>(null)
  const [loading, setLoading] = useState(false)
  const [data, setData] = useState<any>(undefined)
  useEffect(() => {
    async function fetchMevData() {
      if (!hash || !chainId) {
        return
      }
      try {
        setLoading(true)
        const mevData = await chrome.runtime.sendMessage({
          api: 'GetMEVInfo',
          data: {
            chainSpec: {
              chainId: chainId?.toString()
            },
            txHash: hash
          }
        })
        setData(mevData)
      } catch (e) {
        console.error('Failed to fetch MEV data', e)
      } finally {
        setLoading(false)
      }
    }
    fetchMevData()
  }, [hash, chainId])
  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }
    ;(async () => {
      const idList = ['stable', 'flask', 'beta']
      for (let i = 0; i < idList.length; i++) {
        const extensionType = idList[i]
        if (providerRef.current) {
          setIsMetaMask(true)
          return
        }
        try {
          const metamaskPort = chrome.runtime.connect(
            metaMaskIds[extensionType]
          )
          await new Promise<void>((resolve, reject) => {
            const resolveTimeout = setTimeout(() => {
              resolve()
            }, 500)
            metamaskPort.onDisconnect.addListener(() => {
              reject()
              clearTimeout(resolveTimeout)
            })
          })
            .then(
              () => {
                providerRef.current =
                  createExternalExtensionProvider(extensionType)
              },
              () => {
                metamaskPort.disconnect()
              }
            )
            .finally(() => {
              metamaskPort.disconnect()
            })
        } catch {
          //ignore
        }
      }
      if (providerRef.current) {
        setIsMetaMask(true)
      }
    })()
  }, [])
  return (
    <div className="text-sm" id="mev-card">
      <SentioMevInfo
        hash={hash}
        metamaskBtn={
          isMetaMask ? (
            <Button
              size="sm"
              icon={<FlashbotIcon />}
              onClick={async () => {
                try {
                  const provider = providerRef.current
                  if (!provider) {
                    return
                  }
                  await provider.request({
                    method: 'wallet_addEthereumChain',
                    params: [
                      {
                        chainId: '0x1',
                        chainName: 'Flashbots Protect',
                        nativeCurrency: {
                          name: 'ETH',
                          symbol: 'ETH',
                          decimals: 18
                        },
                        rpcUrls: ['https://rpc.flashbots.net'],
                        blockExplorerUrls: ['https://etherscan.io/']
                      }
                    ]
                  })
                } catch (addError) {
                  // handle "add" error
                  console.log(addError)
                }
              }}
            >
              Flashbot RPC
            </Button>
          ) : null
        }
        mevCallback={(type: MevType, role: string, value?: string) => {
          const fundflowTag = document.querySelector(
            '#view-sentio-fundflow'
          ) as HTMLDivElement
          const mevTag = document.querySelector(
            '#view-sentio-mev'
          ) as HTMLDivElement
          if (
            mevTag ||
            !fundflowTag ||
            (role === 'Attacker' && type === 'sandwich') ||
            type === 'arbitrage'
          ) {
            return
          }
          const mevButton = document.createElement('a')
          const icon = document.createElement('i')
          icon.style.marginRight = '0.25rem'
          icon.className = 'fa fa-mask'
          mevButton.appendChild(icon)
          mevButton.innerHTML += 'MEV'
          mevButton.id = 'view-sentio-mev'
          mevButton.onclick = function () {
            const element = document.getElementById('mev-card')
            if (element) {
              const rect = element.getBoundingClientRect()
              const absoluteTop = rect.top + window.scrollY
              const middle = absoluteTop - 60
              window.scrollTo(0, middle)
            }
          }
          mevButton.className = 'mr-2 text-sm'
          mevButton.style.cursor = 'pointer'
          if (role === 'Victim') {
            mevButton.title = `${value ?? 'You'} has been ${type} attacked. Click to view details`
          } else if (role === 'Attacker') {
            mevButton.title = `This is the ${type} attacker. Click to view details`
          }
          fundflowTag.parentNode?.insertBefore(
            mevButton,
            fundflowTag.nextSibling
          )
        }}
        chainId={chainId}
        data={data}
        loading={loading}
        isExtension={true}
      />
    </div>
  )
}
