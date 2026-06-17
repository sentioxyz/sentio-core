// Pure data module — kept free of `next/font` (and any other non-worker-safe
// imports) so it can be safely pulled into the web-worker bundle via
// `lib/metrics/series.ts`. Do not add side-effectful imports here.

export const sentioColors = {
  light: {
    classic: [
      '#5470f0',
      '#47c9d9',
      '#de5f94',
      '#e4bc4f',
      '#4cb275',
      '#77aeef',
      '#9368dd',
      '#e46d6d',
      '#f1904e'
    ],
    purple: [
      '#5b0fa6',
      '#6d11c9',
      '#8617e8',
      '#9b35e9',
      '#a855f7',
      '#b67af2',
      '#7a6bff',
      '#5b7cff',
      '#3e82f6'
    ]
  },
  dark: {
    classic: [
      '#6c8aff',
      '#74dfe6',
      '#ff75b0',
      '#f1cf66',
      '#67c88f',
      '#95c6ff',
      '#b189ff',
      '#f28787',
      '#ffad67'
    ],
    purple: [
      '#3f0a78',
      '#5310a0',
      '#6816c7',
      '#7c2ee6',
      '#9451f4',
      '#a874f8',
      '#6d63f6',
      '#5b7cff',
      '#4794ff'
    ]
  }
}
