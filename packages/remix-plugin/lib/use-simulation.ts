import { API_HOST } from './host'

export type SimulateTransactionRequest = {
  projectOwner?: string
  projectSlug?: string
  simulation?: {
    networkId?: string
    blockNumber?: string
    transactionIndex?: string
    from?: string
    to?: string
    value?: string
    gas?: string
    gasPrice?: string
    input?: string
    sourceOverrides?: Record<string, string>
    originTxHash?: string
  }
}

export const simulateTransaction = async (data: SimulateTransactionRequest, apiKey: string) => {
  const response = await fetch(`${API_HOST}/api/v1/solidity/simulate`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'api-key': apiKey
    },
    body: JSON.stringify(data)
  })
  if (!response.ok) {
    throw new Error('Failed to simulate transaction')
  }
  return response.json()
}
