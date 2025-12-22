import '../styles.css'
import '@sentio/ui-core/dist/style.css'
import { MevInfo, MevType } from './MevInfo'
import type { MevData } from './MevInfo'
import { SvgFolderContext } from '@sentio/ui-core'

const StoryWrapper = ({ children }: { children: React.ReactNode }) => (
  <SvgFolderContext.Provider value="https://app.sentio.xyz">
    {children}
  </SvgFolderContext.Provider>
)

// Mock MEV data for different scenarios

const mockSandwichVictimData: MevData = {
  type: 'VICTIM',
  blockNumber: '18500000',
  txIndex: 150,
  sandwich: {
    revenues: {
      totalUsd: 5234.56
    },
    tokens: [
      {
        address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
        symbol: 'WETH'
      },
      {
        address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
        symbol: 'USDC'
      }
    ],
    traders: [
      {
        address: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
        protocol: 'Uniswap V2',
        tokens: [
          {
            address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
            symbol: 'WETH'
          },
          {
            address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
            symbol: 'USDC'
          }
        ]
      },
      {
        address: '0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F',
        protocol: 'Sushiswap',
        tokens: [
          {
            address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
            symbol: 'USDC'
          },
          {
            address: '0xdAC17F958D2ee523a2206206994597C13D831ec7',
            symbol: 'USDT'
          }
        ]
      }
    ],
    txs: [
      {
        txHash:
          '0xabc123456789def0abc123456789def0abc123456789def0abc123456789def0',
        txIndex: 149
      },
      {
        txHash:
          '0xdef456789abc012def456789abc012def456789abc012def456789abc012def4',
        txIndex: 151
      }
    ],
    victims: [
      {
        txHash:
          '0x1234567890123456789012345678901234567890123456789012345678901234',
        txIndex: 150
      }
    ]
  }
}

const mockSandwichAttackerData: MevData = {
  type: 'ATTACKER',
  blockNumber: '18500000',
  txIndex: 149,
  sandwich: {
    revenues: {
      totalUsd: -2345.67
    },
    tokens: [
      {
        address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
        symbol: 'WETH'
      },
      {
        address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
        symbol: 'USDC'
      }
    ],
    traders: [
      {
        address: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
        protocol: 'Uniswap V2',
        tokens: [
          {
            address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
            symbol: 'WETH'
          },
          {
            address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
            symbol: 'USDC'
          }
        ]
      }
    ],
    txs: [
      {
        txHash:
          '0xabc123456789def0abc123456789def0abc123456789def0abc123456789def0',
        txIndex: 149
      },
      {
        txHash:
          '0xdef456789abc012def456789abc012def456789abc012def456789abc012def4',
        txIndex: 151
      }
    ],
    victims: [
      {
        txHash:
          '0x1234567890123456789012345678901234567890123456789012345678901234',
        txIndex: 150
      }
    ]
  }
}

const mockArbitrageData: MevData = {
  type: 'VICTIM',
  blockNumber: '18500001',
  txIndex: 75,
  arbitrage: {
    txHash:
      '0xfedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210',
    txIndex: 76,
    revenues: {
      totalUsd: 1234.89
    },
    tokens: [
      {
        address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
        symbol: 'WETH'
      },
      {
        address: '0x6B175474E89094C44Da98b954EedeAC495271d0F',
        symbol: 'DAI'
      }
    ],
    traders: [
      {
        address: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
        protocol: 'Uniswap V2',
        tokens: [
          {
            address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
            symbol: 'WETH'
          },
          {
            address: '0x6B175474E89094C44Da98b954EedeAC495271d0F',
            symbol: 'DAI'
          }
        ]
      },
      {
        address: '0x8888888888888888888888888888888888888888',
        protocol: 'Curve Finance',
        tokens: [
          {
            address: '0x6B175474E89094C44Da98b954EedeAC495271d0F',
            symbol: 'DAI'
          },
          {
            address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
            symbol: 'USDC'
          }
        ]
      }
    ]
  }
}

const mockMultiplePoolsData: MevData = {
  type: 'VICTIM',
  blockNumber: '18500100',
  txIndex: 200,
  sandwich: {
    revenues: {
      totalUsd: 8765.43
    },
    tokens: [
      {
        address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
        symbol: 'WETH'
      },
      {
        address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599',
        symbol: 'WBTC'
      },
      {
        address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
        symbol: 'USDC'
      }
    ],
    traders: [
      {
        address: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D',
        protocol: 'Uniswap V2',
        tokens: [
          {
            address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
            symbol: 'WETH'
          },
          {
            address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599',
            symbol: 'WBTC'
          }
        ]
      },
      {
        address: '0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F',
        protocol: 'Sushiswap',
        tokens: [
          {
            address: '0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599',
            symbol: 'WBTC'
          },
          {
            address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
            symbol: 'USDC'
          }
        ]
      },
      {
        address: '0x1111111254fb6c44bAC0beD2854e76F90643097d',
        protocol: 'Pancakeswap',
        tokens: [
          {
            address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
            symbol: 'USDC'
          },
          {
            address: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2',
            symbol: 'WETH'
          }
        ]
      }
    ],
    txs: [
      {
        txHash:
          '0xaaa111222333444555666777888999000aaabbbcccdddeeefffaaa111222333',
        txIndex: 199
      },
      {
        txHash:
          '0xbbb222333444555666777888999000aaabbbcccdddeeefffaaa111222333444',
        txIndex: 201
      }
    ],
    victims: [
      {
        txHash:
          '0xccc333444555666777888999000aaabbbcccdddeeefffaaa111222333444555',
        txIndex: 200
      }
    ]
  }
}

// Stories

export const SandwichVictim = () => {
  return (
    <StoryWrapper>
      <div className="p-4">
        <MevInfo
          hash="0x1234567890123456789012345678901234567890123456789012345678901234"
          chainId="1"
          data={mockSandwichVictimData}
          loading={false}
        />
      </div>
    </StoryWrapper>
  )
}

export const SandwichAttacker = () => {
  return (
    <StoryWrapper>
      <div className="p-4">
        <MevInfo
          hash="0xabc123456789def0abc123456789def0abc123456789def0abc123456789def0"
          chainId="1"
          data={mockSandwichAttackerData}
          loading={false}
        />
      </div>
    </StoryWrapper>
  )
}

export const ArbitrageVictim = () => {
  return (
    <StoryWrapper>
      <div className="p-4">
        <p className="mb-4 text-sm text-gray-500">
          By default, arbitrage MEV is hidden (hideArbitrage=true). Set
          hideArbitrage=false to show it.
        </p>
        <MevInfo
          hash="0x1234567890123456789012345678901234567890123456789012345678901234"
          chainId="1"
          data={mockArbitrageData}
          loading={false}
          hideArbitrage={false}
        />
      </div>
    </StoryWrapper>
  )
}

export const MultiplePools = () => {
  return (
    <StoryWrapper>
      <div className="p-4">
        <MevInfo
          hash="0xccc333444555666777888999000aaabbbcccdddeeefffaaa111222333444555"
          chainId="1"
          data={mockMultiplePoolsData}
          loading={false}
        />
      </div>
    </StoryWrapper>
  )
}

export const WithCustomTokenRenderer = () => {
  const renderToken = (address: string, symbol?: string) => (
    <div className="bg-primary-100 inline-flex items-center gap-1 rounded-full px-2 py-1">
      <div className="bg-primary-600 h-4 w-4 rounded-full" />
      <span className="text-primary-800 text-xs font-semibold">
        {symbol || address.slice(0, 6)}
      </span>
    </div>
  )

  return (
    <StoryWrapper>
      <div className="p-4">
        <MevInfo
          hash="0x1234567890123456789012345678901234567890123456789012345678901234"
          chainId="1"
          data={mockSandwichVictimData}
          loading={false}
          renderToken={renderToken}
        />
      </div>
    </StoryWrapper>
  )
}

export const WithCustomCurrencyFormat = () => {
  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2
    }).format(value)
  }

  return (
    <StoryWrapper>
      <div className="p-4">
        <MevInfo
          hash="0x1234567890123456789012345678901234567890123456789012345678901234"
          chainId="1"
          data={mockSandwichVictimData}
          loading={false}
          formatCurrency={formatCurrency}
        />
      </div>
    </StoryWrapper>
  )
}

export const WithCustomMetamaskButton = () => {
  return (
    <StoryWrapper>
      <div className="p-4">
        <MevInfo
          hash="0x1234567890123456789012345678901234567890123456789012345678901234"
          chainId="1"
          data={mockSandwichVictimData}
          loading={false}
          metamaskBtn={
            <button
              className="rounded-md bg-orange-500 px-3 py-1 text-sm text-white hover:bg-orange-600"
              onClick={() => alert('Add to MetaMask!')}
            >
              🦊 Add Custom RPC
            </button>
          }
        />
      </div>
    </StoryWrapper>
  )
}

export const WithCallback = () => {
  const handleMevCallback = (mevType: MevType, role: string, value: string) => {
    console.log('MEV Detected:', { mevType, role, value })
    // You could send analytics, show notifications, etc.
  }

  return (
    <StoryWrapper>
      <div className="p-4">
        <p className="mb-4 text-sm text-gray-500">
          Open browser console to see callback output when component mounts.
        </p>
        <MevInfo
          hash="0x1234567890123456789012345678901234567890123456789012345678901234"
          chainId="1"
          data={mockSandwichVictimData}
          loading={false}
          mevCallback={handleMevCallback}
        />
      </div>
    </StoryWrapper>
  )
}

export const LoadingState = () => {
  return (
    <StoryWrapper>
      <div className="p-4">
        <MevInfo
          hash="0x1234567890123456789012345678901234567890123456789012345678901234"
          chainId="1"
          data={null}
          loading={true}
        />
      </div>
    </StoryWrapper>
  )
}

export const NoMevData = () => {
  return (
    <StoryWrapper>
      <div className="p-4">
        <p className="mb-4 text-sm text-gray-500">
          When there's no MEV data or type is 'NONE', the component returns
          null:
        </p>
        <MevInfo
          hash="0x1234567890123456789012345678901234567890123456789012345678901234"
          chainId="1"
          data={{ type: 'NONE' }}
          loading={false}
        />
        <p className="mt-4 text-sm text-gray-500">
          (Nothing should appear above this text)
        </p>
      </div>
    </StoryWrapper>
  )
}

export const NonEthereumChain = () => {
  return (
    <StoryWrapper>
      <div className="p-4">
        <p className="mb-4 text-sm text-gray-500">
          MEV info only works on Ethereum mainnet (chainId='1'). Other chains
          return null:
        </p>
        <MevInfo
          hash="0x1234567890123456789012345678901234567890123456789012345678901234"
          chainId="56"
          data={mockSandwichVictimData}
          loading={false}
        />
        <p className="mt-4 text-sm text-gray-500">
          (Nothing should appear above this text because chainId is '56' not
          '1')
        </p>
      </div>
    </StoryWrapper>
  )
}

export const ExtensionModeHidesAttacker = () => {
  return (
    <StoryWrapper>
      <div className="p-4">
        <p className="mb-4 text-sm text-gray-500">
          When isExtension=true and role=Attacker, the component is hidden:
        </p>
        <MevInfo
          hash="0xabc123456789def0abc123456789def0abc123456789def0abc123456789def0"
          chainId="1"
          data={mockSandwichAttackerData}
          loading={false}
          isExtension={true}
        />
        <p className="mt-4 text-sm text-gray-500">
          (Nothing should appear above this text)
        </p>
      </div>
    </StoryWrapper>
  )
}

export const SmallWidth = () => {
  return (
    <StoryWrapper>
      <div className="p-4">
        <div className="max-w-2xl">
          <p className="mb-4 text-sm text-gray-500">
            Responsive layout for smaller widths (less than 1080px):
          </p>
          <MevInfo
            hash="0x1234567890123456789012345678901234567890123456789012345678901234"
            chainId="1"
            data={mockSandwichVictimData}
            loading={false}
          />
        </div>
      </div>
    </StoryWrapper>
  )
}

export const DarkMode = () => {
  return (
    <StoryWrapper>
      <div className="bg-sentio-gray-50 dark min-h-screen p-4">
        <MevInfo
          hash="0x1234567890123456789012345678901234567890123456789012345678901234"
          chainId="1"
          data={mockSandwichVictimData}
          loading={false}
        />
      </div>
    </StoryWrapper>
  )
}

export const WithCustomClassName = () => {
  return (
    <StoryWrapper>
      <div className="p-4">
        <MevInfo
          hash="0x1234567890123456789012345678901234567890123456789012345678901234"
          chainId="1"
          data={mockSandwichVictimData}
          loading={false}
          className="border-4 border-red-500 bg-red-50 p-4"
        />
      </div>
    </StoryWrapper>
  )
}
