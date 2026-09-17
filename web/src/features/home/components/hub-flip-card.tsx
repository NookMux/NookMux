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
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Logo } from '@/assets/logo'
import { cn } from '@/lib/utils'
import { HeroTerminalDemo } from './hero-terminal-demo'
import { HubDiagram } from './hub-diagram'

const FLIP_MS = 1000

/**
 * Symmetric easing on purpose: the faces swap at the halfway point, so the turn
 * has to actually be edge-on at half the duration. An ease-out curve is already
 * ~85% through by then and the swap pops in plain view.
 */
const FLIP_EASING = 'cubic-bezier(0.65, 0, 0.35, 1)'

/**
 * Both faces sit in one grid cell, so they always share a size, and each is
 * swapped in at the half-turn where the card is edge-on and neither is legible.
 * `backface-visibility` alone is not enough: a flattened 3D context silently
 * ignores it and shows the far face straight through the near one.
 */
const FACE = cn(
  'col-start-1 row-start-1 transition-opacity duration-0 [backface-visibility:hidden] motion-reduce:delay-0'
)

interface HubFlipCardProps {
  className?: string
}

/**
 * Two faces of the same card: the routing diagram, and the request it stands
 * for. The hub logo turns it over and the corner logo turns it back — each
 * face stays put until the other one is asked for.
 */
export function HubFlipCard({ className }: HubFlipCardProps) {
  const { t } = useTranslation()
  const [showingApi, setShowingApi] = useState(false)
  const showApi = useCallback(() => setShowingApi(true), [])
  const showDiagram = useCallback(() => setShowingApi(false), [])
  const halfTurn = { transitionDelay: `${FLIP_MS / 2}ms` }

  return (
    <div className={cn('[perspective:2000px]', className)}>
      <div
        className={cn(
          'relative grid w-full transition-transform [transform-style:preserve-3d] motion-reduce:transition-none',
          showingApi && '[transform:rotateY(180deg)]'
        )}
        style={{
          transitionDuration: `${FLIP_MS}ms`,
          transitionTimingFunction: FLIP_EASING,
        }}
      >
        {/* Face A: how a request is routed */}
        <div
          className={cn(
            FACE,
            showingApi ? 'pointer-events-none opacity-0' : 'opacity-100'
          )}
          style={halfTurn}
        >
          <div className='mx-auto flex h-full w-full max-w-2xl items-center justify-center'>
            <HubDiagram
              onHubActivate={showApi}
              hubLabel={t('home.tips.showApiExample')}
            />
          </div>
        </div>

        {/* Face B: what that request looks like on the wire */}
        <div
          className={cn(
            FACE,
            'relative [transform:rotateY(180deg)]',
            showingApi ? 'opacity-100' : 'pointer-events-none opacity-0'
          )}
          style={halfTurn}
        >
          <HeroTerminalDemo />
          <button
            type='button'
            aria-label={t('home.tips.showRoutingDiagram')}
            onMouseEnter={showDiagram}
            onFocus={showDiagram}
            onClick={showDiagram}
            className='border-border bg-card text-foreground hover:bg-muted focus-visible:ring-ring/50 absolute -top-3 -right-3 flex size-10 cursor-pointer items-center justify-center rounded-2xl border shadow-md transition-transform duration-300 outline-none hover:scale-110 focus-visible:ring-3'
          >
            <Logo className='size-5' />
          </button>
        </div>
      </div>
    </div>
  )
}
