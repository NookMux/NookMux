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
import { useMemo } from 'react'
import { Loader2, Send, Shield, UserRound, type LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { SiGithub, SiLinux, SiWechat } from 'react-icons/si'
import { AuthLayout } from '../auth-layout'

type OAuthCallbackScreenProps = {
  provider: string
  mode: 'login' | 'bind'
}

type ProviderMeta = {
  label: string
  Icon: LucideIcon | ((props: { className?: string }) => React.JSX.Element)
}

const providerDictionary: Record<string, ProviderMeta> = {
  github: {
    label: 'profile.fields.gitHub',
    Icon: (props: { className?: string }) => (
      <SiGithub className={props.className} focusable='false' />
    ),
  },
  oidc: { label: 'profile.fields.oidc', Icon: Shield },
  linuxdo: {
    label: 'profile.fields.linuxDo',
    Icon: (props: { className?: string }) => (
      <SiLinux className={props.className} focusable='false' />
    ),
  },
  telegram: { label: 'profile.fields.telegram', Icon: Send },
  discord: { label: 'profile.fields.discord', Icon: Shield },
  wechat: {
    label: 'profile.fields.chat',
    Icon: (props: { className?: string }) => (
      <SiWechat className={props.className} focusable='false' />
    ),
  },
}

export function OAuthCallbackScreen({
  provider,
  mode,
}: OAuthCallbackScreenProps) {
  const { t } = useTranslation()
  const { label, Icon } = useMemo(() => {
    const normalized = provider?.toLowerCase() ?? ''
    return (
      providerDictionary[normalized] || {
        label: 'auth.fields.account',
        Icon: UserRound,
      }
    )
  }, [provider])

  const providerLabel = t(label)
  const isBindMode = mode === 'bind'

  const headline = isBindMode
    ? t('auth.tips.bindingYourProviderAccount', { provider: providerLabel })
    : t('auth.tips.signingYouInWithProvider', { provider: providerLabel })

  const description = isBindMode
    ? t('auth.tips.hangTightWhileWeSecurelyLinkThisAccountTo')
    : t('auth.tips.hangTightWhileWeFinishConnectingYourAccount')

  const secondaryNote = isBindMode
    ? t('auth.status.closeThisTabOnceTheBindingCompletesOrA')
    : t('common.tips.llBeRedirectedAutomaticallyYouCanReturnToThe')

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <div className='flex flex-col items-center space-y-4 text-center'>
          <div className='bg-muted flex h-16 w-16 items-center justify-center rounded-2xl'>
            <Icon className='h-8 w-8' />
          </div>
          <div className='space-y-2'>
            <h2 className='text-center text-2xl font-semibold tracking-tight'>
              {headline}
            </h2>
            <p className='text-muted-foreground text-sm sm:text-base'>
              {description}
            </p>
          </div>
        </div>

        <div className='space-y-4 text-center'>
          <div className='flex items-center justify-center gap-2 text-sm font-medium'>
            <Loader2 className='h-4 w-4 animate-spin' />
            <span>{t('auth.status.processingOauthResponse')}</span>
          </div>
          <p className='text-muted-foreground text-sm'>{secondaryNote}</p>
          <p className='text-muted-foreground text-xs'>
            {t('auth.tips.mayTakeAFewMomentsWhileWeValidateThe')}
          </p>
        </div>
      </div>
    </AuthLayout>
  )
}
