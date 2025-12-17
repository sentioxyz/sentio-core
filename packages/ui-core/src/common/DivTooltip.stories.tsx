import '../styles.css'
import { PopoverTooltip as DivTooltip } from './DivTooltip'

export const Default = () => (
  <div className="p-8">
    <DivTooltip text="This is a tooltip">
      <button className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
        Hover me
      </button>
    </DivTooltip>
  </div>
)

export const WithIcon = () => (
  <div className="p-8">
    <DivTooltip
      text="Tooltip with icon"
      icon={<span className="mr-2">ℹ️</span>}
    >
      <button className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
        Hover me
      </button>
    </DivTooltip>
  </div>
)

export const DifferentPlacements = () => (
  <div className="p-16 space-y-8">
    <div className="flex justify-center">
      <DivTooltip text="Top placement" placementOption="top">
        <button className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
          Top
        </button>
      </DivTooltip>
    </div>
    <div className="flex justify-between">
      <DivTooltip text="Left placement" placementOption="left">
        <button className="px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600">
          Left
        </button>
      </DivTooltip>
      <DivTooltip text="Right placement" placementOption="right">
        <button className="px-4 py-2 bg-red-500 text-white rounded hover:bg-red-600">
          Right
        </button>
      </DivTooltip>
    </div>
    <div className="flex justify-center">
      <DivTooltip text="Bottom placement" placementOption="bottom">
        <button className="px-4 py-2 bg-purple-500 text-white rounded hover:bg-purple-600">
          Bottom
        </button>
      </DivTooltip>
    </div>
  </div>
)

export const WithoutArrow = () => (
  <div className="p-8">
    <DivTooltip text="Tooltip without arrow" hideArrow>
      <button className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
        No Arrow
      </button>
    </DivTooltip>
  </div>
)

export const WithPortal = () => (
  <div className="p-8">
    <DivTooltip text="Tooltip using portal" usePortal>
      <button className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
        With Portal
      </button>
    </DivTooltip>
  </div>
)

export const WithFadeAnimation = () => (
  <div className="p-8">
    <DivTooltip
      text="Tooltip with fade animation"
      enableFadeAnimation
      animationDuration={300}
    >
      <button className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
        Fade Animation
      </button>
    </DivTooltip>
  </div>
)

export const LongText = () => (
  <div className="p-8">
    <DivTooltip
      text="This is a very long tooltip text that should wrap properly and demonstrate how the component handles longer content. It includes multiple sentences and should show proper text wrapping behavior."
      maxWidth="max-w-xs"
    >
      <button className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
        Long Text
      </button>
    </DivTooltip>
  </div>
)

export const CustomStyling = () => (
  <div className="p-8">
    <DivTooltip
      text="Custom styled tooltip"
      className="custom-tooltip-container"
      buttonClassName="custom-button"
      activeButtonClassName="custom-button-active"
    >
      <button className="px-4 py-2 bg-yellow-500 text-white rounded hover:bg-yellow-600">
        Custom Style
      </button>
    </DivTooltip>
  </div>
)

export const WithReactNode = () => (
  <div className="p-8">
    <DivTooltip
      text={
        <div>
          <strong>Rich content tooltip</strong>
          <br />
          <em>With multiple lines</em>
          <br />
          <span style={{ color: 'red' }}>And custom styling</span>
        </div>
      }
    >
      <button className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
        Rich Content
      </button>
    </DivTooltip>
  </div>
)