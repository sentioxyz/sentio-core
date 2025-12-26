import {
  createContext,
  useContext,
  useState,
  useCallback,
  ReactNode
} from 'react'

type Absent<T, K extends keyof T> = { [k in Exclude<keyof T, K>]?: undefined }
type OneOf<T> =
  | { [k in keyof T]?: undefined }
  | (keyof T extends infer K
      ? K extends string & keyof T
        ? { [k in K]: T[K] } & Absent<T, K>
        : never
      : never)

type BaseTokenTag = {}

export type TokenTag = BaseTokenTag &
  OneOf<{ erc20: ERC20Token; erc721: ERC721Token; suiCoin: SuiCoin }>

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
export const OpenContractContext = createContext<openContractFn | undefined>(
  undefined
)

type openCompilationFn = (id: string) => void
export const OpenCompilationContext = createContext<
  openCompilationFn | undefined
>(undefined)

// Tag Cache Context and Types
export type GetTagByAddressResponse = {
  address?: string
  primaryName?: string
  token?: TokenTag
  names?: NameTag[]
}

type TagCacheContextType = {
  tagCache: Map<string, GetTagByAddressResponse>
  setTagCache: (passedMap: Map<string, GetTagByAddressResponse>) => void
  clearTagCache: () => void
}

export const TagCacheContext = createContext<TagCacheContextType>({
  tagCache: new Map(),
  setTagCache: () => {},
  clearTagCache: () => {}
})

// TagCache Provider Component
export const TagCacheProvider = ({ children }: { children: ReactNode }) => {
  const [tagCache, setTagCacheState] = useState<
    Map<string, GetTagByAddressResponse>
  >(new Map())

  const setTagCache = useCallback(
    (passedMap: Map<string, GetTagByAddressResponse>) => {
      setTagCacheState((preMap) => {
        let isDiff = false
        passedMap.forEach((v, k) => {
          const preValue = preMap.get(k)
          // Check if values are different (simple comparison, can be enhanced with deep equality check)
          if (JSON.stringify(preValue) !== JSON.stringify(v)) {
            isDiff = true
          }
        })

        if (isDiff) {
          return new Map([...preMap, ...passedMap])
        }
        return preMap
      })
    },
    []
  )

  const clearTagCache = useCallback(() => {
    setTagCacheState(new Map())
  }, [])

  return (
    <TagCacheContext.Provider value={{ tagCache, setTagCache, clearTagCache }}>
      {children}
    </TagCacheContext.Provider>
  )
}

// Custom hook to use TagCache
export const useTagCache = () => {
  const context = useContext(TagCacheContext)
  if (!context) {
    throw new Error('useTagCache must be used within TagCacheProvider')
  }
  return context
}
