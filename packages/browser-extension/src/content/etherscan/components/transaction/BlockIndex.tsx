import { useTransactionInfo } from '~/content/lib/debug/use-transaction-info'

interface Props {
  hash: string
  chainId: string
}

export const BlockIndex = ({ hash, chainId }: Props) => {
  const { data: transaction } = useTransactionInfo(hash, chainId)
  if (transaction?.transaction?.transactionIndex) {
    try {
      const position = Number.parseInt(
        transaction.transaction.transactionIndex,
        16
      )
      return (
        <span
          style={{
            marginLeft: '4px',
            fontSize: '0.75rem',
            color: 'var(--bs-secondary-color)'
          }}
        >
          (index: {position})
        </span>
      )
    } catch {
      return null
    }
  }
  return null
}
