type WrappedServiceItem = {
  path: string
  method: 'GET' | 'POST'
  useAPIKey?: boolean
}

/**
 * Method key should be unique in wrappedSolidityService and wrappeedWebService
 */

export const wrappedSolidityService: Record<string, WrappedServiceItem> = {
  GetContractName: {
    path: '/api/v1/solidity/contract_name',
    method: 'GET'
  },
  GetABI: {
    path: '/api/v1/solidity/abi',
    method: 'GET'
  },
  GetLatestBlockNumber: {
    path: '/api/v1/solidity/block_number',
    method: 'GET'
  },
  GetBlockSummary: {
    path: '/api/v1/solidity/block_summary',
    method: 'GET'
  },
  SimulateTransaction: {
    path: '/api/v1/solidity/simulate',
    method: 'POST'
  },
  GetContractIndex: {
    path: '/api/v1/solidity/index',
    method: 'GET'
  },
  // GetUserCompilations: {
  //   path: '/api/v1/solidity/user_compilation',
  //   method: 'GET',
  //   useAPIKey: true
  // },
  GetSimulation: {
    path: '/api/v1/solidity/simulate/',
    method: 'GET',
    useAPIKey: true
  },
  GetMEVInfo: {
    path: '/api/v1/solidity/mev_info',
    method: 'GET'
  }
}

export const wrappeedWebService: Record<string, WrappedServiceItem> = {
  GetProjectList: {
    path: '/api/v1/projects',
    method: 'GET',
    useAPIKey: true
  },
  GetChainsStatusSimple: {
    path: '/api/v1/sysstatus/chains',
    method: 'GET'
  }
}

export const wrappedTagService: Record<string, WrappedServiceItem> = {
  MultiGetTagByAddress: {
    path: '/api/v1/tag/multi',
    method: 'POST'
  }
}
