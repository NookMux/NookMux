/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Component, useMemo, useState, type ReactNode } from 'react'
import { GrainGradient } from '@paper-design/shaders-react'
import { cn } from '@/lib/utils'
import { useTheme } from '@/context/theme-provider'

/**
 * Static glow used when WebGL is unavailable — keeps the showcase card
 * readable without the grainy gradient.
 */
function GlowFallback() {
  return (
    <div
      aria-hidden
      className='pointer-events-none absolute inset-0 opacity-40'
      style={{
        background: [
          'radial-gradient(ellipse 45% 55% at 12% 8%, rgba(115, 0, 255, 0.35) 0%, transparent 70%)',
          'radial-gradient(ellipse 45% 55% at 88% 92%, rgba(235, 168, 255, 0.25) 0%, transparent 70%)',
        ].join(', '),
      }}
    />
  )
}

interface ShaderBoundaryState {
  failed: boolean
}

/** Reverts to the glow fallback if the shader crashes at runtime. */
class ShaderBoundary extends Component<
  { children: ReactNode },
  ShaderBoundaryState
> {
  state: ShaderBoundaryState = { failed: false }

  static getDerivedStateFromError(): ShaderBoundaryState {
    return { failed: true }
  }

  render() {
    if (this.state.failed) {
      return <GlowFallback />
    }
    return this.props.children
  }
}

function isWebGLAvailable(): boolean {
  if (typeof window === 'undefined') return false
  try {
    const canvas = document.createElement('canvas')
    return !!(canvas.getContext('webgl2') || canvas.getContext('webgl'))
  } catch {
    return false
  }
}

interface GrainFieldProps {
  /** Merged onto the absolutely-positioned wrapper — opacity, masks, insets. */
  className?: string
}

/**
 * Grainy gradient backdrop, absolutely filling its nearest positioned parent.
 * Colors adapt to the current theme (light/dark).
 * Animation freezes for users who prefer reduced motion;
 * missing WebGL falls back to a static glow.
 */
export function GrainField({ className }: GrainFieldProps) {
  const [supported] = useState(isWebGLAvailable)
  const { resolvedTheme } = useTheme()
  const isDark = resolvedTheme === 'dark'

  const reducedMotion = useMemo(() => {
    if (typeof window === 'undefined') return false
    return window.matchMedia('(prefers-reduced-motion: reduce)').matches
  }, [])

  return (
    <div
      aria-hidden
      className={cn(
        'pointer-events-none absolute inset-0 opacity-60',
        className
      )}
    >
      {supported ? (
        <ShaderBoundary>
          <GrainGradient
            style={{
              position: 'absolute',
              inset: 0,
              width: '100%',
              height: '100%',
            }}
            colors={isDark ? ['#7300ff', '#eba8ff'] : ['#7300ff', '#c9a0ff']}
            colorBack={isDark ? '#0a0a0a' : '#f8f8f8'}
            softness={0.5}
            intensity={0.5}
            noise={0.25}
            shape='corners'
            speed={reducedMotion ? 0 : 0.2}
          />
        </ShaderBoundary>
      ) : (
        <GlowFallback />
      )}
    </div>
  )
}
