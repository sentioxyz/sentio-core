import '../styles.css'
import '@sentio/ui-core/dist/style.css'
import { HexNumber } from './HexNumber'
import { SvgFolderContext } from '@sentio/ui-core'

const address = '0x742d35Cc6634C0532925a3b844Bc454e4438f44e'
const txHash = '0x8ba1f109551bd432803012645ac136ddd64dba72e3a22cb58b4f2e0a7d4b3c5f1'

export const Addresses = () => (
  <div className="p-4 space-y-4">
    <div>
      <div className="text-sm text-gray-500 mb-1">Default</div>
      <HexNumber data={address} />
    </div>
    <div>
      <div className="text-sm text-gray-500 mb-1">With Copy</div>
      <HexNumber data={address} copyable />
    </div>
    <div>
      <div className="text-sm text-gray-500 mb-1">With Avatar</div>
      <HexNumber data={address} avatar copyable />
    </div>
    <div>
      <div className="text-sm text-gray-500 mb-1">Truncated</div>
      <HexNumber data={address} truncate={20} copyable />
    </div>
  </div>
)

export const Transactions = () => (
  <SvgFolderContext.Provider value="https://app.sentio.xyz">
    <div className="p-4 space-y-4">
      <div>
        <div className="text-sm text-gray-500 mb-1">Transaction Hash</div>
        <HexNumber data={txHash} type="tx" chainId="1" copyable />
      </div>
      <div>
        <div className="text-sm text-gray-500 mb-1">Block Number</div>
        <HexNumber data="0x1234567890abcdef" type="block" copyable />
      </div>
      <div>
        <div className="text-sm text-gray-500 mb-1">Large Size</div>
        <HexNumber data={txHash} type="tx" chainId="1" size="lg" copyable />
      </div>
    </div>
  </SvgFolderContext.Provider>
)

export const Variants = () => (
  <SvgFolderContext.Provider value="https://app.sentio.xyz">
    <div className="p-4 space-y-4">
      <div>
        <div className="text-sm text-gray-500 mb-1">With Chain ID</div>
        <HexNumber data={address} chainId="1" type="address" copyable />
      </div>
      <div>
        <div className="text-sm text-gray-500 mb-1">No Link</div>
        <HexNumber data={address} noLink copyable />
      </div>
      <div>
        <div className="text-sm text-gray-500 mb-1">Static Trigger</div>
        <HexNumber data={address} copyable trigger="static" />
      </div>
    </div>
  </SvgFolderContext.Provider>
)
