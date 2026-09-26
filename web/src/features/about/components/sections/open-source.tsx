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
import { useTranslation } from 'react-i18next'
import { SiGithub } from 'react-icons/si'
import { AnimateInView } from '@/components/animate-in-view'

interface OpenSourceProps {
  className?: string
}

export function OpenSource(_props: OpenSourceProps) {
  const { t } = useTranslation()
  const currentYear = new Date().getFullYear()

  return (
    <section className='border-border/40 relative z-10 border-t px-6 py-24 md:py-32'>
      <div className='mx-auto max-w-6xl'>
        <AnimateInView className='mb-16 max-w-lg'>
          <p className='text-muted-foreground mb-3 text-xs font-medium tracking-widest uppercase'>
            {t('about.fields.openSourceEyebrow')}
          </p>
          <h2 className='text-2xl leading-tight font-bold tracking-tight md:text-3xl'>
            {t('about.titles.openSourceTitle')}
          </h2>
        </AnimateInView>

        <div className='grid gap-10 md:grid-cols-[1.1fr_1fr] md:gap-16'>
          <AnimateInView animation='fade-up'>
            <p className='text-muted-foreground max-w-md text-base leading-relaxed'>
              {t('about.tips.openSourceDescription')}
            </p>
            <div className='mt-8'>
              <a
                href='https://github.com/NookMux/NookMux'
                target='_blank'
                rel='noopener noreferrer'
                className='ring-border hover:bg-muted/60 inline-flex h-11 items-center gap-2 rounded-full px-6 text-sm font-semibold ring-1 transition-colors'
              >
                <SiGithub className='size-4' />
                {t('about.actions.viewSource')}
              </a>
            </div>
          </AnimateInView>

          <AnimateInView
            delay={100}
            animation='fade-up'
            className='border-border/40 border-t pt-6 md:border-t-0 md:border-l md:pl-8'
          >
            <div className='space-y-4 text-sm'>
              <p>
                {t('about.fields.newApiProjectRepository')}{' '}
                <a
                  href='https://github.com/NookMux/NookMux'
                  target='_blank'
                  rel='noopener noreferrer'
                  className='text-primary hover:underline'
                >
                  {t('about.placeholders.urlGithubComNookMuxNookMux')}
                </a>
              </p>
              <p className='text-muted-foreground'>
                <a
                  href='https://github.com/NookMux/NookMux'
                  target='_blank'
                  rel='noopener noreferrer'
                  className='text-primary hover:underline'
                >
                  {t('about.fields.newApi')}
                </a>{' '}
                © {currentYear}{' '}
                <a
                  href='https://github.com/NookMux'
                  target='_blank'
                  rel='noopener noreferrer'
                  className='text-primary hover:underline'
                >
                  {t('about.fields.nookMux')}
                </a>{' '}
                {t('about.fields.basedOn')}{' '}
                <a
                  href='https://github.com/songquanpeng/one-api'
                  target='_blank'
                  rel='noopener noreferrer'
                  className='text-primary hover:underline'
                >
                  {t('about.fields.oneApi')}
                </a>{' '}
                © 2023{' '}
                <a
                  href='https://github.com/songquanpeng'
                  target='_blank'
                  rel='noopener noreferrer'
                  className='text-primary hover:underline'
                >
                  {t('about.fields.justSong')}
                </a>
              </p>
              <p className='text-muted-foreground'>
                {t('about.errors.projectMustBeUsedInComplianceWithThe')}{' '}
                <a
                  href='https://github.com/NookMux/NookMux/blob/main/LICENSE'
                  target='_blank'
                  rel='noopener noreferrer'
                  className='text-primary hover:underline'
                >
                  {t('about.tips.agplV30License')}
                </a>
                .
              </p>
            </div>
          </AnimateInView>
        </div>
      </div>
    </section>
  )
}
