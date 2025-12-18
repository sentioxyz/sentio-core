import { getChainLogo } from '@sentio/chain'

type CommonProperties<T, U> = {
  [K in keyof T & keyof U]: T[K] extends U[K] ? U[K] : never
}

export type ChainIconProps = Partial<
  CommonProperties<
    React.ImgHTMLAttributes<HTMLImageElement>,
    React.SVGProps<SVGSVGElement>
  >
>

interface Props {
  chainId: string | number
}

export function ChainIcon({ chainId, ...rest }: Props & ChainIconProps) {
  const logo = getChainLogo(
    typeof chainId === 'number' ? chainId.toString() : chainId
  )
  if (!logo) {
    return null
  }
  return <img src={logo} alt={`Logo of chain ${chainId}`} {...rest} />
}


export function getChainIconFactory(chainId: string | number) {
  return (props: ChainIconProps) => <ChainIcon chainId={chainId} {...props} />
}