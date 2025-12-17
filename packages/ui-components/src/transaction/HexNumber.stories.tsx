import '../styles.css'
import { HexNumber } from './HexNumber'
import { SvgFolderContext } from '../utils/extension-context'

// Ladle works by rendering exported React components from *.stories.* files.
// Export simple React components (no Storybook APIs) so Ladle can pick them up.

export const Default = () => (
  <div className="p-4">
    <HexNumber data="0x742d35Cc6634C0532925a3b844Bc454e4438f44e" />
  </div>
)

export const WithCopy = () => (
  <div className="p-4">
    <HexNumber data="0x742d35Cc6634C0532925a3b844Bc454e4438f44e" copyable />
  </div>
)

export const WithAvatar = () => (
  <div className="p-4">
    <HexNumber data="0x742d35Cc6634C0532925a3b844Bc454e4438f44e" avatar />
  </div>
)

export const WithAvatarAndCopy = () => (
  <div className="p-4">
    <HexNumber
      data="0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
      avatar
      copyable
    />
  </div>
)

export const TransactionHash = () => (
  <SvgFolderContext.Provider value="https://app.sentio.xyz">
    <div className="p-4">
      <HexNumber
        data="0x8ba1f109551bd432803012645ac136ddd64dba72e3a22cb58b4f2e0a7d4b3c5f1"
        type="tx"
        chainId="1"
        copyable
      />
    </div>
  </SvgFolderContext.Provider>
)

export const BlockNumber = () => (
  <div className="p-4">
    <HexNumber data="0x1234567890abcdef" type="block" copyable />
  </div>
)

export const WithTruncate = () => (
  <div className="p-4">
    <HexNumber
      data="0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
      truncate={20}
    />
  </div>
)

export const LargeSize = () => (
  <SvgFolderContext.Provider value="https://app.sentio.xyz">
    <div className="p-4">
      <HexNumber
        data="0x8ba1f109551bd432803012645ac136ddd64dba72e3a22cb58b4f2e0a7d4b3c5f1"
        type="tx"
        chainId="1"
        size="lg"
        copyable
      />
    </div>
  </SvgFolderContext.Provider>
)

export const StaticTrigger = () => (
  <div className="p-4">
    <HexNumber
      data="0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
      copyable
      trigger="static"
    />
  </div>
)

export const WithChainId = () => (
  <SvgFolderContext.Provider value="https://app.sentio.xyz">
    <div className="p-4">
      <HexNumber
        data="0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
        chainId="1"
        type="address"
        copyable
      />
    </div>
  </SvgFolderContext.Provider>
)

export const NoLink = () => (
  <div className="p-4">
    <HexNumber
      data="0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
      noLink
      copyable
    />
  </div>
)
