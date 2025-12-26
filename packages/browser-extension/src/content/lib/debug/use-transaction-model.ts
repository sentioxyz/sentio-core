import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useContext
} from 'react'
import * as monaco from 'monaco-editor'
import { loader } from '@monaco-editor/react'
import { IsSimulationContext } from '../context/transaction'
import {
  GlobalQueryContext,
  getSourcePathKey,
  SourceStore
} from '@sentio/ui-web3'

type Source = {
  compilationId: string
  filePath: string
}

const useTxnSource = (
  hash: string,
  chainId: string,
  onError?: (error: any) => void
) => {
  const { owner, slug } = useContext(GlobalQueryContext) as any
  const [loading, setLoading] = useState(true)
  const [data, setData] = useState<any>()
  const isSimulation = useContext(IsSimulationContext)
  useEffect(() => {
    ;(async () => {
      const data = await chrome.runtime.sendMessage({
        api: isSimulation ? 'FetchAndCompileWithSimulation' : 'FetchAndCompile',
        hash,
        chainId,
        projectOwner: owner,
        projectSlug: slug
      })
      setData(data)
      setLoading(false)
    })()
  }, [isSimulation, hash, chainId])

  return {
    data,
    loading
  }
}

export const useTxnModel = (hash: string, chainId: string) => {
  const [monacoInstance, setMonacoInstance] = useState<any>()
  useEffect(() => {
    loader.config({ monaco })
    loader
      .init()
      .then((monaco) => setMonacoInstance(monaco))
      .catch((e) => console.error('Failed to load monaco', e))
  }, [])
  const [fetchError, setFetchError] = useState('')
  const setError = useCallback((error) => {
    if (error) {
      setFetchError(error.message)
    }
  }, [])
  const { data, loading } = useTxnSource(hash, chainId, setError)
  const registerModelsRef = useRef<Record<string, monaco.editor.ITextModel>>({})

  useEffect(() => {
    const addModel = (source: string, sourcePath: string, id: string) => {
      const key = getSourcePathKey({ compilationId: id, filePath: sourcePath })
      if (!monacoInstance) {
        return
      }
      try {
        const model = monacoInstance?.editor.createModel(
          source,
          'sentio-solidity',
          monaco.Uri.parse(key)
        )
        registerModelsRef.current[key] = model
      } catch {
        console.error('model already exist', key)
      }
    }
    data?.result?.forEach(({ sources, id }) => {
      sources.forEach(({ source, sourcePath }) => {
        addModel(source, sourcePath, id)
      })
    })
  }, [data, monacoInstance])

  useEffect(() => {
    return () => {
      Object.values(registerModelsRef.current).forEach((model) => {
        model.dispose()
      })
    }
  }, [])

  const getModel = useCallback(
    (source: Source) => {
      if (loading) {
        return undefined
      }
      const modelKey = getSourcePathKey(source)
      const modelUri = monaco.Uri.parse(modelKey)
      const model = monacoInstance?.editor.getModel(modelUri)
      if (model) {
        return model
      }
      return null
    },
    [monacoInstance, loading]
  )

  const store = useMemo(() => {
    if (chainId && data) {
      return new SourceStore(data, chainId, undefined)
    }
  }, [data, chainId])

  return {
    getModel,
    store,
    error: fetchError,
    loading
  }
}
