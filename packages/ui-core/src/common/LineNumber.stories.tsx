import '../styles.css'
import type { Story } from '@ladle/react'
import { LineNumber } from './LineNumber'

export const Basic: Story = () => (
  <div className="relative h-16 w-16 p-8">
    <LineNumber className="bg-primary text-white">1</LineNumber>
  </div>
)
Basic.meta = {
  description: 'Absolutely positioned numbered badge used to index query lines'
}

export const Sequence: Story = () => (
  <div className="flex gap-8 p-8">
    {[1, 2, 3].map((n) => (
      <div key={n} className="relative h-6 w-6">
        <LineNumber className="bg-primary-100 text-primary-700">{n}</LineNumber>
      </div>
    ))}
  </div>
)
Sequence.meta = {
  description: 'Several line numbers in a row'
}
