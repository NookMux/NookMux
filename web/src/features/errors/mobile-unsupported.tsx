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
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ProjectAttribution } from '@/components/layout/components/footer'

function PhoneSlashIcon(props: React.ComponentProps<'svg'>) {
  return (
    <svg
      viewBox='0 0 24 24'
      fill='none'
      stroke='currentColor'
      strokeWidth={2}
      strokeLinecap='round'
      strokeLinejoin='round'
      aria-hidden='true'
      {...props}
    >
      <rect x='7' y='2' width='10' height='20' rx='3' />
      <line x1='12' y1='18.4' x2='12.01' y2='18.4' />
      <line x1='3.5' y1='20.5' x2='20.5' y2='3.5' />
    </svg>
  )
}

export function MobileUnsupported() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { systemName } = useSystemConfig()
  const currentYear = new Date().getFullYear()

  return (
    <div className='flex min-h-svh flex-col'>
      <div className='m-auto flex w-full flex-col items-center justify-center gap-2 px-6 text-center'>
        <PhoneSlashIcon className='text-foreground mb-3 size-20' />
        <span className='font-medium'>
          {t('common.tips.consoleUnavailableOnPhone')}
        </span>
        <p className='text-muted-foreground'>
          {t('common.tips.consoleRequiresLargeScreen')}
        </p>
        <Badge
          variant='outline'
          className='border-info/30 bg-info/10 text-info'
        >
          <span className='bg-info size-1.5 animate-pulse rounded-full' />
          {t('common.tips.mobileOptimizationInProgress')}
        </Badge>
        <div className='mt-6 flex gap-4'>
          <Button
            variant='outline'
            onClick={() => navigate({ to: '/pricing' })}
          >
            {t('home.actions.viewPricing')}
          </Button>
          <Button onClick={() => navigate({ to: '/' })}>
            {t('layout.actions.backToHome')}
          </Button>
        </div>
      </div>
      <footer className='border-border/40 border-t'>
        <div className='flex flex-col items-center justify-between gap-x-3 gap-y-2 px-6 py-4 sm:flex-row'>
          <span className='text-muted-foreground/40 text-xs'>
            &copy; {currentYear} {systemName || 'NookMux'}.{' '}
            {t('layout.tips.footerDefaultCopyright')}
          </span>
          <ProjectAttribution currentYear={currentYear} />
        </div>
      </footer>
    </div>
  )
}
