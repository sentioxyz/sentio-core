import type { Story } from '@ladle/react'
import { sentioColors } from './sentio-colors'

function Swatches({ title, colors }: { title: string; colors: string[] }) {
  return (
    <div className="mb-4">
      <p className="text-text-foreground-secondary mb-1 text-xs font-medium">
        {title}
      </p>
      <div className="flex flex-wrap gap-1">
        {colors.map((c) => (
          <div key={c} className="flex flex-col items-center gap-1">
            <span
              className="border-light h-8 w-8 rounded border"
              style={{ backgroundColor: c }}
            />
            <span className="text-text-foreground-secondary text-[10px]">
              {c}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

export const Palettes: Story = () => (
  <div className="p-8">
    <Swatches title="light · classic" colors={sentioColors.light.classic} />
    <Swatches title="light · purple" colors={sentioColors.light.purple} />
    <Swatches title="dark · classic" colors={sentioColors.dark.classic} />
    <Swatches title="dark · purple" colors={sentioColors.dark.purple} />
  </div>
)
Palettes.meta = {
  description: 'Series color palettes (light/dark × classic/purple)'
}
