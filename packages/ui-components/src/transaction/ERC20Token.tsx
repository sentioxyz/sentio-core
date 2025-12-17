import { EthChainInfo, ChainId, getChainExternalUrl } from "@sentio/chain"
import { chainIdToNumber, toChecksumAddress, useAddressTag } from "../utils/use-tag"
import { ChainIcon } from "./ChainIcons"
import { ChainIdContext } from "./transaction-context"
import { useContext } from "react"
import { PopoverTooltip } from "../common/DivTooltip"
import { CopyButton } from "../common/CopyButton"
import EtherLink from "./EtherLink"
import { HexNumber } from "./HexNumber"

export const NetworkNode = ({ id }: { id: string }) => {
  if (id === undefined) {
    return null
  }
  const chainId = chainIdToNumber(id)?.toString() || '1'
  const nativeToken = EthChainInfo[chainId]
  return (
    <span className="text-ilabel inline-flex items-center gap-2 rounded-full border px-2 py-0.5">
      <ChainIcon chainId={chainId} className="text-gray h-4 w-4" />
      {nativeToken?.name || 'Unknown'}
    </span>
  )
}

const defaultChain = EthChainInfo[ChainId.ETHEREUM]

export const getNativeToken = (chainId?: string) => {
  if (!chainId) {
    return defaultChain
  }
  return EthChainInfo[chainId] || defaultChain
}

export const TokenLabel = ({
  address,
  symbol,
  link,
  logo
}: {
  address: string
  symbol?: string
  link?: string
  logo?: string
}) => {
  const chainId = chainIdToNumber(useContext(ChainIdContext))?.toString() || '1'
  const checksumAddress = toChecksumAddress(address)
  return (
    <PopoverTooltip
      offsetOptions={2}
      hideArrow
      strategy="fixed"
      maxWidth="max-w-[500px]"
      placementOption="bottom-start"
      text={
        <div
          className="text-ilabel text-gray overflow-hidden px-2 py-1"
          onClick={(evt) => {
            evt.stopPropagation()
          }}
        >
          <div className="flex w-full justify-between">
            <div className="flex items-center gap-2 ">
              {symbol && (
                <CopyButton text={symbol} size={16}>
                  <span>{symbol}</span>
                </CopyButton>
              )}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <CopyButton text={address}>
              <span className="font-mono">{checksumAddress}</span>
            </CopyButton>
            <EtherLink address={checksumAddress} chainId={chainId} trigger="static" type="token" />
          </div>
        </div>
      }
    >
      <a href={link || '#'} target="_blank" rel="noreferrer">
        <section className="group inline-flex items-center gap-1">
          {logo ? (
            <img src={logo} className="flex-0 h-4 w-4 rounded-full" alt="token logo" />
          ) : (
            <ChainIcon chainId={chainId} className="text-gray h-4 w-4" />
          )}
          {symbol ? (
            <span className="group-hover:text-primary dark:text-text-foreground font-medium text-gray-800">
              {symbol}
            </span>
          ) : (
            <span className="font-medium text-gray-800">
              <HexNumber data={checksumAddress} truncate={8} />
            </span>
          )}
        </section>
      </a>
    </PopoverTooltip>
  )
}

interface Props {
  address: string
  tokenName?: string
}

export const ERC20Token = ({ address, tokenName }: Props) => {
  const chainId = chainIdToNumber(useContext(ChainIdContext))?.toString()
  const { data } = useAddressTag(address)
  const externalLink = getChainExternalUrl(chainId, address, 'token')
  const tokenData = data?.token?.erc20

  // ETH
  if (address === '0x0000000000000000000000000000000000000000') {
    const nativeToken = getNativeToken(chainId)
    return <TokenLabel address={address} symbol={nativeToken.tokenSymbol} link={externalLink} />
  }

  return (
    <TokenLabel
      address={address}
      symbol={tokenName || tokenData?.symbol?.toUpperCase()}
      link={externalLink}
      logo={tokenData?.logo}
    />
  )
}