import { CircleCheck, CircleDashed, CircleX } from 'lucide-react'

export const TransactionStatus = ({ status }: { status?: number }) => {
  if (status === undefined) return null
  if (status === 1) {
    return (
      <span className="text-success inline-flex items-center justify-end font-medium">
        <CircleCheck className="mr-2 inline-block h-4 w-4" />
        Success
      </span>
    )
  }
  if (status === 0) {
    return (
      <span className="text-danger inline-flex items-center justify-end font-medium">
        <CircleX className="mr-2 inline-block h-4 w-4" />
        Failed
      </span>
    )
  }

  return (
    <span className="text-info inline-flex items-center justify-end font-medium">
      <CircleDashed className="mr-2 inline-block h-4 w-4" />
      Pending
    </span>
  )
}
