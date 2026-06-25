import type { GroupStyleLike } from '../types/enums'
import { sentioColors } from '../charts/theme/sentio-colors'

// Curated palette for the Group header highlight. Keys are persisted on the
// Panel as `highlight_color`; the visual color is resolved from the shared
// `sentioColors.classic` palette so groups always track the chart palette.
export type HighlightColorKey =
  | ''
  | 'blue'
  | 'cyan'
  | 'pink'
  | 'yellow'
  | 'green'
  | 'lightblue'
  | 'purple'
  | 'red'
  | 'orange'

// Each highlight key maps to an INDEX into sentioColors.{light,dark}.classic
// so EMPHASIS picks up the theme-appropriate hue automatically.
//   classic[0]=blue 1=cyan 2=pink 3=yellow 4=green 5=lightblue 6=purple 7=red 8=orange
const CLASSIC_INDEX: Record<Exclude<HighlightColorKey, ''>, number> = {
  blue: 0,
  cyan: 1,
  pink: 2,
  yellow: 3,
  green: 4,
  lightblue: 5,
  purple: 6,
  red: 7,
  orange: 8
}

export interface HighlightColorMeta {
  key: Exclude<HighlightColorKey, ''>
  name: string
}

export const HIGHLIGHT_COLORS: HighlightColorMeta[] = [
  { key: 'green', name: 'Sentio Green' },
  { key: 'blue', name: 'Sentio Blue' },
  { key: 'cyan', name: 'Sentio Cyan' },
  { key: 'lightblue', name: 'Sentio Light Blue' },
  { key: 'purple', name: 'Sentio Purple' },
  { key: 'pink', name: 'Sentio Pink' },
  { key: 'red', name: 'Sentio Red' },
  { key: 'orange', name: 'Sentio Orange' },
  { key: 'yellow', name: 'Sentio Yellow' }
]

// Default key used when the user picks EMPHASIS without choosing a color yet —
// keeps the preview from rendering an invisible white bar.
export const DEFAULT_HIGHLIGHT_KEY: Exclude<HighlightColorKey, ''> = 'green'

// Resolve the base CSS color (hex) for a highlight key + theme. Returns
// undefined for an unknown key so callers can fall back to the default.
export function getHighlightHex(
  key: string | undefined,
  isDark: boolean
): string | undefined {
  if (!key) return undefined
  const idx = CLASSIC_INDEX[key as Exclude<HighlightColorKey, ''>]
  if (idx === undefined) return undefined
  return sentioColors[isDark ? 'dark' : 'light'].classic[idx]
}

// Compute a readable foreground (#000 or #fff) for a given hex bg using the
// W3C relative-luminance formula. Avoids hardcoding a per-color text color so
// the palette can change without re-tuning contrast.
function readableForeground(hex: string): string {
  const m = hex.replace('#', '')
  const r = parseInt(m.slice(0, 2), 16) / 255
  const g = parseInt(m.slice(2, 4), 16) / 255
  const b = parseInt(m.slice(4, 6), 16) / 255
  const lin = (v: number) =>
    v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4)
  const L = 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
  return L > 0.5 ? '#1f2937' : '#ffffff'
}

export interface ResolvedHighlight {
  solid: string
  foreground: string
}

export function resolveHighlight(
  colorKey: string | undefined,
  isDark: boolean
): ResolvedHighlight {
  const hex =
    getHighlightHex(colorKey, isDark) ??
    getHighlightHex(DEFAULT_HIGHLIGHT_KEY, isDark)!
  return { solid: hex, foreground: readableForeground(hex) }
}

// Resolve the CSS styles to apply to the GroupPanel HEADER row based on the
// configured style + color + theme. Returns an empty object for DEFAULT.
export function resolveHeaderStyle(
  style: GroupStyleLike | undefined,
  colorKey: string | undefined,
  isDark: boolean
): React.CSSProperties {
  if (!style || style === 'DEFAULT') return {}
  const color = resolveHighlight(colorKey, isDark)
  return { backgroundColor: color.solid, color: color.foreground }
}
