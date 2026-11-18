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
import { cn } from '@/lib/utils'
import type { SystemStatus } from '../types'

interface TermsFooterProps {
  variant?: 'sign-in' | 'sign-up'
  className?: string
  status?: SystemStatus | null
}

export function TermsFooter({
  variant = 'sign-in',
  className,
  status,
}: TermsFooterProps) {
  const { t } = useTranslation()
  const leadInText =
    variant === 'sign-in'
      ? t('auth.tips.byClickingSignInYouAgreeToOur')
      : t('auth.tips.byCreatingAnAccountYouAgreeToOur')

  const hasUserAgreement = Boolean(status?.user_agreement_enabled)
  const hasPrivacyPolicy = Boolean(status?.privacy_policy_enabled)

  if (!hasUserAgreement && !hasPrivacyPolicy) {
    return null
  }

  const agreementLink = {
    label: 'layout.fields.userAgreement',
    href: '/user-agreement',
  }
  const privacyLink = {
    label: 'layout.fields.privacyPolicy',
    href: '/privacy-policy',
  }

  const activeLinks =
    hasUserAgreement || hasPrivacyPolicy
      ? ([
          hasUserAgreement ? agreementLink : null,
          hasPrivacyPolicy ? privacyLink : null,
        ].filter(Boolean) as Array<{ label: string; href: string }>)
      : [agreementLink, privacyLink]

  const [firstLink, secondLink] = activeLinks

  return (
    <p className={cn('text-muted-foreground text-center text-xs', className)}>
      {leadInText}{' '}
      {firstLink && (
        <a
          href={firstLink.href}
          className='hover:text-primary underline underline-offset-4'
        >
          {t(firstLink.label)}
        </a>
      )}
      {secondLink && (
        <>
          {' '}
          {t('auth.fields.value')}{' '}
          <a
            href={secondLink.href}
            className='hover:text-primary underline underline-offset-4'
          >
            {t(secondLink.label)}
          </a>
        </>
      )}
      {t('auth.fields.sentencePeriod')}
    </p>
  )
}
