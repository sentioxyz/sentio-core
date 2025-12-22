import '../styles.css'
import '@sentio/ui-core/dist/style.css'
import { SimulatorInfo } from './SimulatorInfo'
import type { SimulationData } from './SimulatorInfo'

// Mock simulation data
const mockSimulationWithBlockOverride: SimulationData = {
  blockOverride: {
    number: '0x1234567',
    timestamp: '0x65000000',
    baseFeePerGas: '0x3b9aca00',
    gasLimit: '0x1c9c380',
    coinbase: '0x0000000000000000000000000000000000000001'
  }
}

const mockSimulationWithSourceOverrides: SimulationData = {
  sourceOverrides: {
    '0x1234567890123456789012345678901234567890': 'compilation_abc123',
    '0xabcdefabcdefabcdefabcdefabcdefabcdefabcd': 'compilation_xyz789'
  }
}

const mockSimulationWithStateOverrides: SimulationData = {
  stateOverrides: {
    '0x1234567890123456789012345678901234567890': {
      balance: '5000000000000000000',
      nonce: '10'
    },
    '0xabcdefabcdefabcdefabcdefabcdefabcdefabcd': {
      balance: '1000000000000000000',
      state: {
        '0x0': '0x1',
        '0x1': '0x2'
      }
    }
  }
}

const mockSimulationWithBlockHashes: SimulationData = {
  blockOverride: {
    timestamp: '0x65000000',
    number: '0x112a880',
    blockHash: {
      '-1': '0xabc123456789def0abc123456789def0abc123456789def0abc123456789def0',
      '-2': '0xdef456789abc012def456789abc012def456789abc012def456789abc012def4'
    }
  }
}

const mockSimulationWithCodeOverride: SimulationData = {
  stateOverrides: {
    '0x1234567890123456789012345678901234567890': {
      balance: '5000000000000000000',
      nonce: '5',
      code: '0x608060405234801561001057600080fd5b50600436106100365760003560e01c806306fdde031461003b578063313ce567146100b9575b600080fd5b6100436100d7565b6040518080602001828103825283818151815260200191508051906020019080838360005b83811015610083578082015181840152602081019050610068565b50505050905090810190601f1680156100b05780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b6100c1610179565b6040518082815260200191505060405180910390f35b606060008054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561016f5780601f106101445761010080835404028352916020019161016f565b820191906000526020600020905b81548152906001019060200180831161015257829003601f168201915b5050505050905090565b6000600160009054906101000a900460ff16905090565b600080fd5b565b600080fd5b565b600080fd5b565b600080fd5b565b600080fd5b565b600080fd5b565b600080fd5b565b600080fd5b565b600080fd5b565b600080fd5b56fea264697066735822122089a4f6c4f3e5c1b8a2f5e5d6c1b8a2f5e5d6c1b8a2f5e5d6c1b8a2f5e5d664736f6c634300080f0033'
    }
  }
}

const mockSimulationAllTypes: SimulationData = {
  blockOverride: {
    number: '0x1234567',
    timestamp: '0x65000000',
    baseFeePerGas: '0x3b9aca00',
    blockHash: {
      '-1': '0xabc123456789def0abc123456789def0abc123456789def0abc123456789def0'
    }
  },
  sourceOverrides: {
    '0x1234567890123456789012345678901234567890': 'compilation_v1',
    '0xabcdefabcdefabcdefabcdefabcdefabcdefabcd': 'compilation_v2'
  },
  stateOverrides: {
    '0x9876543210987654321098765432109876543210': {
      balance: '10000000000000000000',
      nonce: '15',
      state: {
        '0x0': '0xffff'
      }
    }
  }
}

// Ladle stories - export simple React components

export const Default = () => {
  return (
    <div className="p-4">
      <SimulatorInfo simulationData={mockSimulationWithBlockOverride} />
    </div>
  )
}

Default.story = {
  name: 'Block Override Only'
}

export const WithSourceOverrides = () => {
  return (
    <div className="p-4">
      <SimulatorInfo simulationData={mockSimulationWithSourceOverrides} />
    </div>
  )
}

export const WithStateOverrides = () => {
  return (
    <div className="p-4">
      <SimulatorInfo simulationData={mockSimulationWithStateOverrides} />
    </div>
  )
}

export const WithBlockHashes = () => {
  return (
    <div className="p-4">
      <SimulatorInfo simulationData={mockSimulationWithBlockHashes} />
    </div>
  )
}

export const WithCodeOverride = () => {
  return (
    <div className="p-4">
      <SimulatorInfo simulationData={mockSimulationWithCodeOverride} />
    </div>
  )
}

export const AllOverrideTypes = () => {
  return (
    <div className="p-4">
      <SimulatorInfo simulationData={mockSimulationAllTypes} />
    </div>
  )
}

export const WithCustomCompilationTag = () => {
  return (
    <div className="p-4">
      <SimulatorInfo
        simulationData={mockSimulationWithSourceOverrides}
        renderCompilationTag={(id) => (
          <a
            href={`/compilations/${id}`}
            className="text-primary-600 hover:text-primary-800 hover:underline"
            onClick={(e) => {
              e.preventDefault()
              alert(`Navigate to compilation: ${id}`)
            }}
          >
            {id} →
          </a>
        )}
      />
    </div>
  )
}

export const NoOverrides = () => {
  return (
    <div className="p-4">
      <p className="mb-4 text-sm text-gray-500">
        When there are no overrides, the component returns null and nothing is
        rendered:
      </p>
      <SimulatorInfo simulationData={{}} />
      <p className="mt-4 text-sm text-gray-500">
        (Nothing should appear above this text)
      </p>
    </div>
  )
}

export const DefaultOpen = () => {
  return (
    <div className="p-4">
      <SimulatorInfo
        simulationData={{
          ...mockSimulationWithBlockOverride,
          ...mockSimulationWithSourceOverrides,
          ...mockSimulationWithStateOverrides
        }}
        className="border-primary-500 border-2"
      />
    </div>
  )
}

DefaultOpen.story = {
  name: 'With Custom className'
}

export const DarkMode = () => {
  return (
    <div className="dark min-h-screen bg-gray-900 p-4">
      <SimulatorInfo simulationData={mockSimulationAllTypes} />
    </div>
  )
}
