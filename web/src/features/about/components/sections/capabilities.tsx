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
import { PlugZap, Waypoints, ReceiptText, KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'

interface CapabilitiesProps {
  className?: string
}

export function Capabilities(_props: CapabilitiesProps) {
  const { t } = useTranslation()

  const capabilities = [
    {
      id: 'unified',
      num: '01',
      title: t('about.fields.capabilityUnifiedTitle'),
      desc: t('about.tips.capabilityUnifiedDesc'),
      span: 'md:col-span-2',
      icon: <PlugZap className='size-4 text-blue-400' />,
      visual: (
        <div className='mt-4 grid grid-cols-2 gap-2 md:grid-cols-3'>
          {[
            '/v1/chat/completions',
            '/v1/messages',
            '/v1/embeddings',
            '/v1/images/generations',
            '/v1/audio/speech',
            '/v1/rerank',
          ].map((path) => (
            <div
              key={path}
              className='border-border/30 bg-muted/20 text-muted-foreground flex items-center justify-center rounded-lg border px-3 py-2 font-mono text-xs transition-colors duration-300 hover:border-blue-500/30 hover:bg-blue-500/5'
            >
              {path}
            </div>
          ))}
        </div>
      ),
    },
    {
      id: 'routing',
      num: '02',
      title: t('about.fields.capabilityRoutingTitle'),
      desc: t('about.tips.capabilityRoutingDesc'),
      span: 'md:col-span-1',
      icon: <Waypoints className='size-4 text-violet-400' />,
      visual: (
        <div className='mt-4 space-y-2'>
          {[
            t('about.fields.loadBalancing'),
            t('about.fields.autoRetry'),
            t('about.fields.failover'),
          ].map((step, i) => (
            <div key={step} className='flex items-center gap-2'>
              <div
                className={`flex size-6 items-center justify-center rounded-full text-[10px] font-bold ${
                  i === 1
                    ? 'border border-blue-500/30 bg-blue-500/20 text-blue-500'
                    : 'border-border/40 bg-muted text-muted-foreground border'
                }`}
              >
                {i + 1}
              </div>
              <div className='bg-border/40 h-px flex-1' />
              <span className='text-muted-foreground text-xs'>{step}</span>
            </div>
          ))}
        </div>
      ),
    },
    {
      id: 'billing',
      num: '03',
      title: t('about.fields.capabilityBillingTitle'),
      desc: t('about.tips.capabilityBillingDesc'),
      span: 'md:col-span-1',
      icon: <ReceiptText className='size-4 text-emerald-400' />,
      visual: (
        <div className='mt-4 flex items-center justify-center'>
          <div className='relative'>
            <div className='flex size-16 items-center justify-center rounded-2xl border border-emerald-500/20 bg-emerald-500/5'>
              <ReceiptText
                className='size-7 text-emerald-500/70'
                strokeWidth={1.5}
              />
            </div>
            <div className='absolute -top-1 -right-1 flex size-4 items-center justify-center rounded-full bg-emerald-500'>
              <svg
                className='size-2.5 text-white'
                fill='none'
                viewBox='0 0 24 24'
                stroke='currentColor'
                strokeWidth={3}
              >
                <path
                  strokeLinecap='round'
                  strokeLinejoin='round'
                  d='m4.5 12.75 6 6 9-13.5'
                />
              </svg>
            </div>
          </div>
        </div>
      ),
    },
    {
      id: 'keys',
      num: '04',
      title: t('about.fields.capabilityKeysTitle'),
      desc: t('about.tips.capabilityKeysDesc'),
      span: 'md:col-span-2',
      icon: <KeyRound className='size-4 text-amber-400' />,
      visual: (
        <div className='mt-4 flex items-center gap-3'>
          <div className='flex -space-x-2'>
            {['Key', 'Group', 'Quota', 'Expiry'].map((n) => (
              <div
                key={n}
                className='border-background from-muted to-muted/60 text-muted-foreground flex size-8 items-center justify-center rounded-full border-2 bg-gradient-to-br text-[9px] font-bold'
              >
                {n}
              </div>
            ))}
          </div>
          <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
            <KeyRound className='size-3.5 text-amber-500' />
            {t('about.fields.fineGrainedControl')}
          </div>
        </div>
      ),
    },
  ]

  return (
    <section className='relative z-10 px-6 py-24 md:py-32'>
      <div className='mx-auto max-w-6xl'>
        <AnimateInView className='mb-16 max-w-lg'>
          <p className='text-muted-foreground mb-3 text-xs font-medium tracking-widest uppercase'>
            {t('about.fields.capabilitiesEyebrow')}
          </p>
          <h2 className='text-2xl leading-tight font-bold tracking-tight md:text-3xl'>
            {t('about.titles.capabilitiesTitle')}
          </h2>
        </AnimateInView>

        {/* Bento grid */}
        <div className='border-border/40 bg-border/40 grid gap-px overflow-hidden rounded-xl border md:grid-cols-3'>
          {capabilities.map((c, i) => (
            <AnimateInView
              key={c.id}
              delay={i * 100}
              animation='scale-in'
              className={`bg-background group hover:bg-muted/20 p-7 transition-colors duration-300 md:p-8 ${c.span}`}
            >
              <div className='mb-3 flex items-center gap-3'>
                <span className='border-border/40 bg-muted text-muted-foreground flex size-7 items-center justify-center rounded-md border text-[10px] font-semibold tabular-nums'>
                  {c.num}
                </span>
                <h3 className='text-sm font-semibold'>{c.title}</h3>
              </div>
              <p className='text-muted-foreground text-sm leading-relaxed'>
                {c.desc}
              </p>
              {c.visual}
            </AnimateInView>
          ))}
        </div>
      </div>
    </section>
  )
}
