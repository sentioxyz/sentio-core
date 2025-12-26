import '../../styles.css'
import { LinkifyText } from './LinkifyText'

export const Default = () => (
  <div className="p-4">
    <LinkifyText text="This is a simple text without links" />
  </div>
)

export const WithUrl = () => (
  <div className="p-4">
    <LinkifyText text="Check out this website: https://www.example.com for more information" />
  </div>
)

export const WithMultipleUrls = () => (
  <div className="p-4">
    <LinkifyText text="Visit https://github.com and https://stackoverflow.com for developer resources" />
  </div>
)

export const WithNumbers = () => (
  <div className="p-4">
    <LinkifyText
      text="Transaction amount: 1000 tokens at price 25.50 USD"
      isHighlightNumbers={true}
    />
  </div>
)

export const WithHexAddress = () => (
  <div className="p-4">
    <LinkifyText
      text="Address 0x1234567890abcdef with balance 100.5"
      isHighlightNumbers={true}
    />
  </div>
)

export const WithUrlAndNumbers = () => (
  <div className="p-4">
    <LinkifyText
      text="Visit https://etherscan.io/address/0x123abc to see balance of 1000.25 ETH"
      isHighlightNumbers={true}
    />
  </div>
)

export const WithCustomClassName = () => (
  <div className="p-4">
    <LinkifyText
      text="Custom styled text with https://example.com"
      className="text-lg font-bold text-blue-600"
    />
  </div>
)

export const WithComplexContent = () => (
  <div className="space-y-4 p-4">
    <div>
      <h3 className="mb-2 font-semibold">Transaction Details:</h3>
      <LinkifyText
        text="Amount: 1234.56 ETH from 0xabcdef123456 to https://etherscan.io/address/0x987654321"
        isHighlightNumbers={true}
      />
    </div>
  </div>
)

export const NullAndUndefined = () => (
  <div className="space-y-4 p-4">
    <div>
      <strong>Null: </strong>
      <LinkifyText text={null} />
    </div>
    <div>
      <strong>Undefined: </strong>
      <LinkifyText text={undefined} />
    </div>
    <div>
      <strong>Number: </strong>
      <LinkifyText text={42} />
    </div>
  </div>
)
