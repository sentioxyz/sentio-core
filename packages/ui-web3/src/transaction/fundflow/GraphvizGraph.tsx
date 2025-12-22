import { instance } from '@viz-js/viz'
import {
  TransformWrapper,
  TransformComponent,
  useControls
} from 'react-zoom-pan-pinch'
import { useEffect, useRef, memo, useState } from 'react'
import { SpinLoading } from '@sentio/ui-core'
import { cx as classNames } from 'class-variance-authority'

interface Props {
  zoomable?: boolean
  graphString?: string
}

const GraphMemo = memo(function _Graph({
  graphString
}: {
  graphString?: string
}) {
  const { resetTransform } = useControls()
  const [loading, setLoading] = useState(true)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    instance()
      .then((viz) => {
        if (!containerRef.current || !graphString) return

        // Clear previous content
        while (containerRef.current.firstChild) {
          containerRef.current.removeChild(containerRef.current.firstChild)
        }

        // Render the graph
        const node = viz.renderSVGElement(graphString)
        node.setAttribute('style', 'max-width: 100% !important; height: auto;')
        containerRef.current?.appendChild(node)

        // Add hover effects to edges
        containerRef.current
          ?.querySelectorAll('.flow-chart-edge')
          .forEach((el) => {
            const id = el.getAttribute('id')
            const onHover = () => {
              containerRef.current
                ?.querySelectorAll('.flow-chart-edge')
                .forEach((edge) => {
                  if (edge.getAttribute('id') !== id) {
                    edge.classList.add('opacity-20')
                  }
                })
            }
            const unHover = () => {
              containerRef.current
                ?.querySelectorAll('.flow-chart-edge')
                .forEach((edge) => {
                  edge.classList.remove('opacity-20')
                })
            }
            if (id) {
              el.addEventListener('mouseover', onHover)
              el.addEventListener('mouseout', unHover)
            }
          })

        resetTransform()
        setLoading(false)
      })
      .catch((error) => {
        console.error('Error rendering graph:', error)
        setLoading(false)
      })
  }, [graphString, resetTransform])

  return (
    <TransformComponent
      wrapperStyle={{
        height: '100%'
      }}
    >
      <SpinLoading
        loading={loading}
        className={classNames('h-full w-full', loading ? 'min-h-[300px]' : '')}
      >
        <div ref={containerRef} />
      </SpinLoading>
    </TransformComponent>
  )
})

export const VizGraph = ({ zoomable, graphString }: Props) => {
  return (
    <div className="fundflow-container h-full w-full">
      <TransformWrapper
        minScale={0.1}
        doubleClick={{
          disabled: true
        }}
        panning={{
          velocityDisabled: true
        }}
        smooth={false}
        zoomAnimation={{
          disabled: true
        }}
        alignmentAnimation={{
          disabled: true
        }}
        wheel={{
          step: 0.05
        }}
        pinch={{
          step: 0.05
        }}
        disabled={!zoomable}
        centerOnInit={true}
      >
        <GraphMemo graphString={graphString} />
      </TransformWrapper>
    </div>
  )
}
