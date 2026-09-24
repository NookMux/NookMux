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
import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useSystemConfig } from '@/hooks/use-system-config'
import { GrainField } from '@/components/landing/grain-field'

interface AboutHeroProps {
  isAuthenticated?: boolean
  className?: string
}

/**
 * About page hero: mirrors the home hero's editorial type and grainy purple
 * field, with a declarative accent line in place of the rotating words.
 */
export function AboutHero(props: AboutHeroProps) {
  const { t } = useTranslation()
  const { systemName } = useSystemConfig()

  return (
    <section className='relative flex min-h-[calc(100svh-var(--app-header-height,3rem))] flex-col justify-center overflow-hidden px-6'>
      <GrainField className='[mask-image:linear-gradient(to_bottom,black_55%,transparent)] opacity-45' />
      <div className='relative mx-auto w-full max-w-6xl'>
        <div className='max-w-3xl'>
          <p
            className='landing-animate-fade-up text-muted-foreground text-xs font-medium tracking-[0.2em] uppercase opacity-0'
            style={{ animationDelay: '0ms' }}
          >
            {t('about.fields.heroEyebrow', { name: systemName })}
          </p>

          <h1
            className='landing-animate-fade-up mt-8 text-4xl font-medium tracking-tight opacity-0 sm:text-5xl lg:text-6xl'
            style={{ animationDelay: '60ms' }}
          >
            {t('about.titles.heroHeadline')}
            <span className='block h-4' />
            <span className='text-[#7300ff] dark:text-[#cf9fff]'>
              {t('about.titles.heroHeadlineAccent')}
            </span>
          </h1>

          <p
            className='landing-animate-fade-up text-muted-foreground mt-6 max-w-xl text-lg leading-relaxed opacity-0 md:text-xl'
            style={{ animationDelay: '120ms' }}
          >
            {t('about.tips.heroDescription')}
          </p>

          <div
            className='landing-animate-fade-up mt-10 flex flex-wrap items-center gap-4 opacity-0'
            style={{ animationDelay: '180ms' }}
          >
            {props.isAuthenticated ? (
              <Link
                to='/dashboard'
                className='inline-flex h-11 items-center gap-1.5 rounded-full bg-neutral-950 px-6 text-sm font-semibold text-white transition-colors hover:bg-neutral-800 dark:bg-white dark:text-neutral-950 dark:hover:bg-neutral-200'
              >
                {t('layout.titles.goToDashboard')}
                <ArrowRight className='size-4' />
              </Link>
            ) : (
              <>
                <Link
                  to='/sign-up'
                  className='inline-flex h-11 items-center gap-1.5 rounded-full bg-neutral-950 px-6 text-sm font-semibold text-white transition-colors hover:bg-neutral-800 dark:bg-white dark:text-neutral-950 dark:hover:bg-neutral-200'
                >
                  {t('dashboard.fields.getStarted')}
                  <ArrowRight className='size-4' />
                </Link>
                <Link
                  to='/pricing'
                  className='ring-border hover:bg-muted/60 inline-flex h-11 items-center rounded-full px-6 text-sm font-semibold ring-1 transition-colors'
                >
                  {t('home.actions.viewPricing')}
                </Link>
              </>
            )}
          </div>
        </div>
      </div>
    </section>
  )
}
