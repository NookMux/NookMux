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
import { Logo } from '@/assets/logo'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

const LLM_ICONS = [
  'OpenAI',
  'Claude.Color',
  'Gemini.Color',
  'DeepSeek.Color',
  'Qwen.Color',
  'ChatGLM.Color',
  'Grok',
  'Mistral.Color',
  'Meta.Color',
  'Kimi.Mono',
  'Doubao.Color',
  'Minimax.Color',
] as const

const DEV_ICONS = [
  { name: 'Cursor.Mono', y: 39 },
  { name: 'GithubCopilot.Mono', y: 134 },
  { name: 'Trae.Mono', y: 229 },
  { name: 'Codex.Mono', y: 324 },
] as const

/** Right-side fan-out curve colors, one per dev tool icon. */
const CURVE_COLORS = ['#60a5fa', '#fb7185', '#fbbf24', '#2dd4bf']

// Stage geometry (px); the SVG overlay shares this 400x420 coordinate space.
const STAGE_WIDTH = 400
const STAGE_HEIGHT = 420
const PILL_RIGHT = 72
const HUB_LEFT = 168
const HUB_RIGHT = 232
const HUB_CENTER_Y = 210
const ICON_LEFT = 328

interface HubDiagramProps {
  className?: string
  /**
   * Turns the center hub into a control. Fires on hover, focus and tap, which
   * is what the showcase card listens to before flipping to the API example.
   */
  onHubActivate?: () => void
  /** Accessible name for the hub control; required with `onHubActivate`. */
  hubLabel?: string
}

/**
 * Zeabur-style routing diagram: an infinitely scrolling pill of LLM provider
 * icons on the left converges into the New API hub, then fans out through
 * colored bezier curves to developer tools on the right.
 *
 * Decorative unless `onHubActivate` is passed, which promotes the center hub
 * to a real button so keyboard and touch users get the same affordance.
 */
export function HubDiagram({
  className,
  onHubActivate,
  hubLabel,
}: HubDiagramProps) {
  const HubTag = onHubActivate ? 'button' : 'div'

  return (
    <div
      aria-hidden={onHubActivate ? undefined : 'true'}
      className={cn('relative overflow-hidden rounded-[2rem]', className)}
    >
      <div
        className='relative mx-auto'
        style={{ width: STAGE_WIDTH, height: STAGE_HEIGHT }}
      >
        {/* Connection overlay: straight in-feed + colored fan-out curves */}
        <svg
          className='absolute inset-0 h-full w-full'
          viewBox={`0 0 ${STAGE_WIDTH} ${STAGE_HEIGHT}`}
          fill='none'
        >
          <line
            x1={PILL_RIGHT}
            y1={HUB_CENTER_Y}
            x2={HUB_LEFT}
            y2={HUB_CENTER_Y}
            stroke='currentColor'
            strokeWidth={1.5}
            className='text-border'
          />
          {DEV_ICONS.map((icon, i) => {
            const yc = icon.y + 28
            return (
              <path
                key={icon.name}
                d={`M ${HUB_RIGHT} ${HUB_CENTER_Y} C ${HUB_RIGHT + 42} ${HUB_CENTER_Y} ${ICON_LEFT - 42} ${yc} ${ICON_LEFT} ${yc}`}
                stroke={CURVE_COLORS[i]}
                strokeOpacity={0.65}
                strokeWidth={1.5}
              />
            )
          })}
        </svg>

        {/* Left: infinitely scrolling LLM icon pill */}
        <div className='scroll-container border-border bg-background/80 absolute top-[50px] bottom-[50px] left-4 w-14 overflow-hidden rounded-full border [mask-image:linear-gradient(to_bottom,transparent,black_15%,black_85%,transparent)]'>
          <div className='animate-scroll-down flex flex-col items-center gap-6 py-3'>
            {[...LLM_ICONS, ...LLM_ICONS].map((iconName, i) => (
              // oxlint-disable-next-line react/no-array-index-key
              <span
                key={`${iconName}-${i}`}
                className='text-foreground shrink-0'
              >
                {getLobeIcon(iconName, 26)}
              </span>
            ))}
          </div>
        </div>

        {/* Center: New API hub */}
        <div className='absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2'>
          <div className='bg-primary/20 absolute -inset-5 rounded-full blur-2xl' />
          <HubTag
            {...(onHubActivate && {
              type: 'button' as const,
              'aria-label': hubLabel,
              onMouseEnter: onHubActivate,
              onFocus: onHubActivate,
              onClick: onHubActivate,
            })}
            className={cn(
              'border-border bg-card relative flex h-16 w-16 items-center justify-center rounded-2xl border shadow-lg',
              onHubActivate &&
                'focus-visible:ring-ring/50 cursor-pointer transition-transform duration-300 outline-none hover:scale-110 focus-visible:ring-3'
            )}
          >
            <Logo className='size-8' />
          </HubTag>
        </div>

        {/* Right: developer tool icons */}
        {DEV_ICONS.map((icon) => (
          <div
            key={icon.name}
            className='border-border bg-card text-foreground absolute flex h-14 w-14 items-center justify-center rounded-2xl border shadow-sm'
            style={{ left: ICON_LEFT, top: icon.y }}
          >
            {getLobeIcon(icon.name, 26)}
          </div>
        ))}
      </div>
    </div>
  )
}
