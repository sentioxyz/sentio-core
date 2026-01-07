import '../styles.css'
import type { Story } from '@ladle/react'
import { useEffect, useState } from 'react'
import { ProgressBar } from './ProgressBar'

export const BasicProgressBar: Story = () => {
  const [progress, setProgress] = useState(0.3)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Basic Progress Bar</h3>
      <ProgressBar progress={progress} gradient />
      <div className="mt-4">
        <input
          type="range"
          min="0"
          max="1"
          step="0.01"
          value={progress}
          onChange={(e) => setProgress(parseFloat(e.target.value))}
          className="w-full"
        />
        <p className="mt-2 text-sm text-gray-600">
          Progress: {(progress * 100).toFixed(0)}%
        </p>
      </div>
    </div>
  )
}

BasicProgressBar.meta = {
  description: 'Basic progress bar with gradient colors'
}

export const WithUpperTicks: Story = () => {
  const [progress, setProgress] = useState(0.45)

  return (
    <div className="p-8">
      <h3 className="mb-8 text-lg font-semibold">
        Progress Bar with Upper Ticks
      </h3>
      <ProgressBar
        progress={progress}
        gradient
        upperTicks={{
          0: '0%',
          0.25: '25%',
          0.5: '50%',
          0.75: '75%',
          1: '100%'
        }}
      />
      <div className="mt-8">
        <input
          type="range"
          min="0"
          max="1"
          step="0.01"
          value={progress}
          onChange={(e) => setProgress(parseFloat(e.target.value))}
          className="w-full"
        />
        <p className="mt-2 text-sm text-gray-600">
          Progress: {(progress * 100).toFixed(0)}%
        </p>
      </div>
    </div>
  )
}

WithUpperTicks.meta = {
  description: 'Progress bar with tick marks and labels on top'
}

export const WithLowerTicks: Story = () => {
  const [progress, setProgress] = useState(0.6)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">
        Progress Bar with Lower Ticks
      </h3>
      <ProgressBar
        progress={progress}
        gradient
        lowerTicks={{
          0: 'Start',
          0.5: 'Halfway',
          1: 'Complete'
        }}
      />
      <div className="mt-8">
        <input
          type="range"
          min="0"
          max="1"
          step="0.01"
          value={progress}
          onChange={(e) => setProgress(parseFloat(e.target.value))}
          className="w-full"
        />
        <p className="mt-2 text-sm text-gray-600">
          Progress: {(progress * 100).toFixed(0)}%
        </p>
      </div>
    </div>
  )
}

WithLowerTicks.meta = {
  description: 'Progress bar with tick marks and labels on bottom'
}

export const WithBothTicks: Story = () => {
  const [progress, setProgress] = useState(0.35)

  return (
    <div className="p-8">
      <h3 className="mb-8 text-lg font-semibold">
        Progress Bar with Upper and Lower Ticks
      </h3>
      <ProgressBar
        progress={progress}
        gradient
        upperTicks={{
          0: '0',
          0.25: '250k',
          0.5: '500k',
          0.75: '750k',
          1: '1M'
        }}
        lowerTicks={{
          0: 'Min',
          0.5: 'Target',
          1: 'Max'
        }}
      />
      <div className="mt-12">
        <input
          type="range"
          min="0"
          max="1"
          step="0.01"
          value={progress}
          onChange={(e) => setProgress(parseFloat(e.target.value))}
          className="w-full"
        />
        <p className="mt-2 text-sm text-gray-600">
          Progress: {(progress * 100).toFixed(0)}%
        </p>
      </div>
    </div>
  )
}

WithBothTicks.meta = {
  description: 'Progress bar with tick marks on both top and bottom'
}

export const CustomSegments: Story = () => {
  const [progress, setProgress] = useState(0.7)

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Custom Color Segments</h3>
      <ProgressBar
        progress={progress}
        gradient
        segments={{
          0.33: 'from-green-600 to-green-500',
          0.66: 'from-green-500 to-yellow-500',
          1: 'from-yellow-500 to-red-600'
        }}
        upperTicks={{
          0: 'Low',
          0.33: 'Medium',
          0.66: 'High',
          1: 'Critical'
        }}
      />
      <div className="mt-8">
        <input
          type="range"
          min="0"
          max="1"
          step="0.01"
          value={progress}
          onChange={(e) => setProgress(parseFloat(e.target.value))}
          className="w-full"
        />
        <p className="mt-2 text-sm text-gray-600">
          Progress: {(progress * 100).toFixed(0)}%
        </p>
      </div>
    </div>
  )
}

CustomSegments.meta = {
  description: 'Progress bar with custom color segments'
}

export const DifferentStages: Story = () => {
  return (
    <div className="space-y-8 p-8">
      <h3 className="text-lg font-semibold">Different Progress Stages</h3>

      <div>
        <p className="mb-2 text-sm font-medium">Starting (10%)</p>
        <ProgressBar progress={0.1} gradient />
      </div>

      <div>
        <p className="mb-2 text-sm font-medium">Quarter (25%)</p>
        <ProgressBar progress={0.25} gradient />
      </div>

      <div>
        <p className="mb-2 text-sm font-medium">Halfway (50%)</p>
        <ProgressBar progress={0.5} gradient />
      </div>

      <div>
        <p className="mb-2 text-sm font-medium">Three Quarters (75%)</p>
        <ProgressBar progress={0.75} gradient />
      </div>

      <div>
        <p className="mb-2 text-sm font-medium">Almost Done (90%)</p>
        <ProgressBar progress={0.9} gradient />
      </div>

      <div>
        <p className="mb-2 text-sm font-medium">Complete (100%)</p>
        <ProgressBar progress={1} gradient />
      </div>
    </div>
  )
}

DifferentStages.meta = {
  description: 'Progress bar showing different completion stages'
}

export const AnimatedProgress: Story = () => {
  const [progress, setProgress] = useState(0)

  useEffect(() => {
    const interval = setInterval(() => {
      setProgress((prev) => {
        if (prev >= 1) {
          return 0
        }
        return prev + 0.01
      })
    }, 50)
    return () => clearInterval(interval)
  }, [])

  return (
    <div className="p-8">
      <h3 className="mb-4 text-lg font-semibold">Animated Progress Bar</h3>
      <ProgressBar progress={progress} gradient />
      <p className="mt-4 text-sm text-gray-600">
        Progress: {(progress * 100).toFixed(0)}%
      </p>
    </div>
  )
}

AnimatedProgress.meta = {
  description: 'Progress bar with automatic animation'
}
