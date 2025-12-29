import domtoimage from 'dom-to-image-more'

export function exportPNG(name: string, element?: HTMLDivElement | null) {
  if (element) {
    domtoimage.toPng(element, { bgcolor: 'white' }).then(function (
      dataUrl: string
    ) {
      const link = document.createElement('a')
      link.download = `${name}.png`
      link.href = dataUrl
      link.click()
    })
  }
}

// todo: find a way to export a real svg render by echarts
export function exportSVG(name: string, element?: HTMLDivElement | null) {
  if (element) {
    domtoimage.toSvg(element, { bgcolor: 'white' }).then(function (
      dataUrl: string
    ) {
      const link = document.createElement('a')
      link.download = `${name}.svg`
      link.href = dataUrl
      link.click()
    })
  }
}
