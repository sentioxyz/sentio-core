import { wrappedSolidityService, wrappeedWebService } from './services'
import isString from 'lodash/isString'

let cachedPromise: {
  expireTime: number
  promise: Promise<any>
}
async function fetchConfig() {
  const time = new Date().getTime()
  if (cachedPromise && cachedPromise.expireTime > time) {
    return cachedPromise.promise
  }
  cachedPromise = {
    expireTime: time + 1000 * 60 * 60, // expire after 1 hour
    promise: fetch('https://api.npoint.io/0075d9580a46f78f6d65')
      .then((res) => res.json())
      .catch(() => ({}))
  }
  return cachedPromise.promise
}

async function callApi(
  path,
  params?: Record<string, any> | string,
  method = 'GET',
  apiKey?: string,
  host = 'https://app.sentio.xyz'
) {
  const url = new URL(host)
  const headers = new Headers()
  if (apiKey) {
    headers.set('api-key', apiKey)
  }
  url.pathname = path
  try {
    let res
    if (method === 'POST') {
      res = await fetch(url, {
        method,
        body: isString(params) ? (params as string) : JSON.stringify(params),
        headers
      })
    } else {
      if (params) {
        if (isString(params)) {
          url.search = params as string
        } else {
          const flattenParams = (obj, prefix = '') => {
            return Object.keys(obj).reduce((acc, k) => {
              const pre = prefix.length ? prefix + '.' : ''
              if (typeof obj[k] === 'object')
                Object.assign(acc, flattenParams(obj[k], pre + k))
              else acc[pre + k] = obj[k]
              return acc
            }, {})
          }
          url.search = new URLSearchParams(flattenParams(params)).toString()
        }
      }
      res = await fetch(url, { headers })
    }
    return res.json()
  } catch (e) {
    return {
      code: 2,
      message: e?.toString() || 'API error'
    }
  }
}

async function callPostApi(path, params) {
  return callApi(path, params, 'POST')
}

async function apiHandler(request, sendResponse) {
  switch (request.api) {
    case 'GetChainConfig': {
      const res = await fetchConfig()
      sendResponse(res)
      break
    }
    case 'FetchAndCompileWithSimulation':
    case 'FetchAndCompile': {
      const apiKey = (await chrome.storage.sync.get('apiKey')).apiKey
      const payload: Record<string, any> = {
        networkId: request.chainId,
        sourceOnly: true
      }
      if (request.address) {
        payload.addresses = request.address
      } else if (request.hash) {
        payload[
          request.api === 'FetchAndCompile'
            ? 'txId.txHash'
            : 'txId.simulationId'
        ] = request.hash
      } else {
        return
      }

      if (request.projectOwner) {
        payload.projectOwner = request.projectOwner
      }
      if (request.projectSlug) {
        payload.projectSlug = request.projectSlug
      }
      const res = await callApi(
        '/api/v1/solidity/fetch_and_compile',
        payload,
        'GET',
        apiKey as string
      )
      sendResponse(res)
      break
    }
    case 'GetCallTraceWithSimulation':
    case 'GetCallTrace': {
      const apiKey = (await chrome.storage.sync.get('apiKey')).apiKey
      const req = {
        networkId: request.chainId,
        withInternalCalls: request.withInternalCalls
      }
      if (request.projectOwner) {
        req['projectOwner'] = request.projectOwner
      }
      if (request.projectSlug) {
        req['projectSlug'] = request.projectSlug
      }
      if (request.api === 'GetCallTrace') {
        req['txId.txHash'] = request.hash
      } else {
        req['txId.simulationId'] = request.hash
      }
      const res = await callApi(
        '/api/v1/solidity/call_trace',
        req,
        'GET',
        apiKey as string
      )
      sendResponse(res)
      break
    }
    case 'GetContractIndex': {
      const { data } = request
      let res: any = undefined
      if (data) {
        res = await callApi('/api/v1/solidity/index', data)
      } else {
        res = await callApi('/api/v1/solidity/index', {
          address: request.address,
          'chainSpec.chainId': request.chainId
        })
      }
      sendResponse(res)
      break
    }
    case 'GetTransactionInfoWithSimulation':
    case 'GetTransactionInfo': {
      const res = await callApi(
        '/api/v1/solidity/transaction_info',
        Object.assign(
          {
            networkId: request.chainId,
            withStateDiff: !!request.withStateDiff
          },
          request.api === 'GetTransactionInfo'
            ? {
                'txId.txHash': request.hash
              }
            : {
                'txId.simulationId': request.hash
              }
        )
      )
      sendResponse(res)
      break
    }
    case 'GetTransactions': {
      const txHashStr = request.txHashList
        .map((hash) => `txHash=${hash}`)
        .join('&')
      const res = await callApi(
        '/api/v1/solidity/transactions',
        `${txHashStr}&networkId=${request.networkId}`
      )
      sendResponse(res)
      break
    }
    case 'MultiGetTagByAddress': {
      const { data } = request
      let res: any = undefined
      if (data) {
        res = await callPostApi('/api/v1/tag/multi', {
          requests: data.requests
        })
      } else {
        res = await callPostApi('/api/v1/tag/multi', {
          requests: request.requests
        })
      }
      sendResponse(res)
      break
    }
    case 'SearchTransaction': {
      const res = await callApi('/api/v1/solidity/search_transactions', {
        ...request.requests
      })
      sendResponse(res)
      break
    }
    case 'SimulateTransaction': {
      try {
        const storageKeys = await chrome.storage.sync.get(['apiKey', 'project'])
        const { apiKey, project } = storageKeys
        const [projectOwner, projectSlug] = (project as string).split('/')
        const res = await callApi(
          '/api/v1/solidity/simulate',
          {
            ...request.data,
            projectOwner,
            projectSlug
          },
          'POST',
          apiKey as string
        )
        sendResponse({
          ...res,
          projectOwner,
          projectSlug
        })
        break
      } catch {
        // ignore
      }
      sendResponse(
        await callApi(
          '/api/v1/solidity/simulate',
          {
            ...request.data
          },
          'POST'
        )
      )
      break
    }
    case 'GetPrice': {
      const res = await callApi('/api/v1/prices', {
        ...request.data
      })
      sendResponse(res)
      break
    }
    case 'GetSimulation': {
      const { simulationId, headers, initReq, ...otherParam } = request.data
      const apiKey = (await chrome.storage.sync.get('apiKey')).apiKey
      const res = await callApi(
        `/api/v1/solidity/simulate/${request.data.simulationId}`,
        otherParam,
        'GET',
        apiKey as string
      )
      sendResponse(res)
      break
    }
    case 'GetInscription': {
      const res = await callApi(
        `/api/ethscriptions/${request.id}`,
        undefined,
        'GET',
        undefined,
        'https://api.ethscriptions.com'
      )
      sendResponse(res)
      break
    }
    case 'GetMevBatch': {
      const { hashList, chainId } = request
      const hashStr = hashList.map((hash) => `txHash=${hash}`).join('&')
      const res = await callApi(
        '/api/v1/solidity/mev_info/batch',
        `networkId=${chainId}&${hashStr}`
      )
      sendResponse(res)
      break
    }
    default: {
      const api =
        wrappedSolidityService[request.api] || wrappeedWebService[request.api]
      if (!api) {
        return
      }
      const { path, method, useAPIKey } = api
      const apiKey = useAPIKey
        ? ((await chrome.storage.sync.get('apiKey')).apiKey as string)
        : undefined
      switch (method) {
        case 'GET': {
          const res = await callApi(
            api.path,
            {
              ...request.data
            },
            'GET',
            apiKey
          )
          sendResponse(res)
          break
        }
        case 'POST': {
          const res = await callApi(
            api.path,
            {
              ...request.data
            },
            'POST',
            apiKey
          )
          sendResponse(res)
          break
        }
      }
    }
  }
}

chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.api) {
    try {
      apiHandler(request, sendResponse)
    } catch (e: any) {
      // ignore
    }
    return true
  }
})

chrome.runtime.onMessageExternal.addListener(
  (request, sender, sendResponse) => {
    if (request.type === 'save') {
      chrome.storage.sync
        .set(request.data)
        .then(() => {
          sendResponse({ success: true })
        })
        .catch(() => {
          sendResponse({ success: false })
        })
      return true
    }
  }
)
