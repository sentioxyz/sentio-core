export function exportSVG(name: string, element?: HTMLDivElement | null) {
  if (!element) return

  // Check if dom-to-image is available
  if (typeof window !== 'undefined' && (window as any).domtoimage) {
    const domtoimage = (window as any).domtoimage
    domtoimage
      .toSvg(element, { bgcolor: 'white' })
      .then(function (dataUrl: string) {
        const link = document.createElement('a')
        link.download = `${name}.svg`
        link.href = dataUrl
        link.click()
      })
      .catch((error: any) => {
        console.error('Error exporting SVG:', error)
      })
  } else {
    console.warn(
      'dom-to-image library not found. Please include it in your project for SVG export functionality.'
    )
  }
}

export function exportPNG(name: string, element?: HTMLDivElement | null) {
  if (!element) return

  if (typeof window !== 'undefined' && (window as any).domtoimage) {
    const domtoimage = (window as any).domtoimage
    domtoimage
      .toPng(element, { bgcolor: 'white' })
      .then(function (dataUrl: string) {
        const link = document.createElement('a')
        link.download = `${name}.png`
        link.href = dataUrl
        link.click()
      })
      .catch((error: any) => {
        console.error('Error exporting PNG:', error)
      })
  } else {
    console.warn(
      'dom-to-image library not found. Please include it in your project for PNG export functionality.'
    )
  }
}
