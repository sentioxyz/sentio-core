import { createContext, useContext } from 'react'

export const SvgFolderContext = createContext('')
export const useDetectExtenstion = () => {
  const folderPath = useContext(SvgFolderContext)
  return Boolean(folderPath)
}

export const DarkModeContext = createContext(false)
export const useDarkMode = () => {
  return useContext(DarkModeContext)
}

type Absent<T, K extends keyof T> = { [k in Exclude<keyof T, K>]?: undefined };
type OneOf<T> =
  | { [k in keyof T]?: undefined }
  | (
    keyof T extends infer K ?
      (K extends string & keyof T ? { [k in K]: T[K] } & Absent<T, K>
        : never)
    : never);

type BaseTokenTag = {
}

export type TokenTag = BaseTokenTag
  & OneOf<{ erc20: ERC20Token; erc721: ERC721Token; suiCoin: SuiCoin }>

export type ERC20Token = {
  contractAddress?: string
  name?: string
  symbol?: string
  decimals?: number
  logo?: string
  website?: string
}

export type ERC721Token = {
  contractAddress?: string
  name?: string
  symbol?: string
  logo?: string
  website?: string
}

export type SuiCoin = {
  coinType?: string
  objectId?: string
  symbol?: string
  decimals?: number
  logo?: string
  bridge?: boolean
  verified?: boolean
}

export type NameTag = {
  label?: string
  dataSource?: string
  updatedAt?: string
  expiresAt?: string
}


type TagData = {
  address?: string
  primaryName?: string
  token?: TokenTag
  names?: NameTag[]
}

export const TagsContext = createContext<Map<string, TagData>>(new Map())

type openContractFn = (address: string, chain: string) => void
export const OpenContractContext = createContext<openContractFn | undefined>(undefined)

type openCompilationFn = (id: string) => void
export const OpenCompilationContext = createContext<openCompilationFn | undefined>(undefined)

