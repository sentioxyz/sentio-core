import { startCase } from 'lodash'

// Curly-brace template token, e.g. "{{ method }}".
const TEMPLATE_TOKEN = /{{([\s\S]+?)}}/g

// Map a few proto label keys to friendlier template names, preserving the
// originals too. Pure — no proto / worker deps.
export function sanitizeLabels(labels: { [k: string]: string }): {
  [k: string]: string
} {
  const result: { [k: string]: string } = {}
  for (const k in labels) {
    switch (k) {
      case 'contract_name':
        result['contract'] = labels[k]
        break
      case 'contract_address':
        result['address'] = labels[k]
        break
    }
    result[k] = labels[k]
  }
  return result
}

// Resolve a `{{token}}` alias against a series' labels. Returns undefined when
// no alias is given so callers can fall back to a display name.
export function aliasTemplate(
  alias: string | undefined | null,
  labels: { [k: string]: string }
): string | undefined {
  if (alias) {
    try {
      const safe = sanitizeLabels(labels)
      return alias.replace(TEMPLATE_TOKEN, (_, m1) => {
        const value = safe[m1.trim()]
        return value == null ? `` : value
      })
    } catch (e) {
      return alias
    }
  }
}

function escapeColumnId(id: string): string {
  return id.replace(/[\W_.]+/g, '_')
}

// Derive a stable column id + human-readable name for a metrics series from
// its alias template (or display name fallback) and labels.
export function getColumnNameId(
  labels: { [p: string]: string },
  alias?: string,
  displayName?: string
): { columnName: string; columnId: string } {
  const s = aliasTemplate(alias, labels) || startCase(displayName)
  return { columnName: s, columnId: escapeColumnId(s) }
}
