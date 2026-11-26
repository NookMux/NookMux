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
import { useEffect, type ReactNode } from 'react'
import { Mail } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { SiGithub, SiLinux } from 'react-icons/si'
import { handleGitHubOAuth, handleLinuxDOOAuth } from '@/lib/oauth'
import { useDialogs } from '@/hooks/use-dialog'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import { OAUTH_BIND_STORAGE_KEY } from '@/features/auth/constants'
import { SettingsGroup } from '@/features/system-settings/components/settings-group'
import { SettingsNotice } from '@/features/system-settings/components/settings-notice'
import { SettingsPanel } from '@/features/system-settings/components/settings-panel'
import type { UserProfile } from '../../types'
import { ChangePasswordDialog } from '../dialogs/change-password-dialog'
import { EmailBindDialog } from '../dialogs/email-bind-dialog'
import { METHOD_GRID_CLASS, TilesSkeleton } from './section-skeleton'

interface LoginMethodsSectionProps {
  profile: UserProfile | null
  loading: boolean
  failed: boolean
  onRetry: () => void
  /** 绑定结果变化后刷新资料。 */
  onProfileUpdate: () => void
}

type DialogKey = 'password' | 'email'

/** 登录方式的标题：品牌图标、名称与「已绑定」状态。 */
function BindingLabel({
  icon,
  label,
  bound,
}: {
  icon: ReactNode
  label: string
  bound: boolean
}) {
  const { t } = useTranslation()

  return (
    <span className='inline-flex flex-wrap items-center gap-2'>
      <span
        className='text-muted-foreground flex size-4 shrink-0 items-center justify-center'
        aria-hidden='true'
      >
        {icon}
      </span>
      {label}
      {bound && (
        <StatusBadge
          label={t('profile.fields.bound')}
          variant='success'
          copyable={false}
        />
      )}
    </span>
  )
}

/**
 * 登录与认证：更改密码与站点启用的各类账号绑定，每种方式是一个等宽的操作面板，
 * 容器够宽时一行三个，方式增多时自动换行。
 */
export function LoginMethodsSection({
  profile,
  loading,
  failed,
  onRetry,
  onProfileUpdate,
}: LoginMethodsSectionProps) {
  const { t } = useTranslation()
  const dialogs = useDialogs<DialogKey>()
  const { status, loading: statusLoading } = useStatus()

  // 第三方授权在弹出的窗口里完成，结果经 localStorage 通知本页刷新资料。
  useEffect(() => {
    const handleStorage = (event: StorageEvent) => {
      if (event.key !== OAUTH_BIND_STORAGE_KEY || !event.newValue) return
      try {
        const payload = JSON.parse(event.newValue) as { status?: string }
        if (payload?.status === 'success') {
          onProfileUpdate()
        }
      } catch {
        // 无法解析的载荷不触发刷新。
      }
      try {
        window.localStorage.removeItem(OAUTH_BIND_STORAGE_KEY)
      } catch {
        // 存储不可用时无需清理。
      }
    }

    window.addEventListener('storage', handleStorage)
    return () => window.removeEventListener('storage', handleStorage)
  }, [onProfileUpdate])

  const iconClass = 'size-4'
  const bindings =
    profile && status
      ? [
          {
            id: 'email',
            label: t('auth.fields.email'),
            icon: <Mail className={iconClass} />,
            value: profile.email,
            bound: Boolean(profile.email),
            enabled: true,
            onBind: () => dialogs.open('email'),
          },
          {
            id: 'github',
            label: t('profile.fields.gitHub'),
            icon: <SiGithub className={iconClass} />,
            value: profile.github_id,
            bound: Boolean(profile.github_id),
            enabled: status.github_oauth === true,
            onBind: () => {
              if (status.github_client_id) {
                void handleGitHubOAuth(status.github_client_id)
              }
            },
          },
          {
            id: 'linuxdo',
            label: t('profile.fields.linuxDo'),
            icon: <SiLinux className={iconClass} />,
            value: profile.linux_do_id,
            bound: Boolean(profile.linux_do_id),
            enabled: status.linuxdo_oauth === true,
            onBind: () => {
              if (status.linuxdo_client_id) {
                void handleLinuxDOOAuth(status.linuxdo_client_id)
              }
            },
          },
        ].filter((binding) => binding.enabled)
      : []

  // 已绑定的第三方账号不能在此解绑，只有邮箱可以换成另一个地址。
  const bindingAction = (binding: (typeof bindings)[number]) => {
    if (binding.bound && binding.id !== 'email') return undefined

    return (
      <Button variant='outline' size='sm' onClick={binding.onBind}>
        {binding.bound ? t('profile.fields.change') : t('profile.actions.bind')}
      </Button>
    )
  }

  const ready =
    profile !== null && status !== null && !loading && !statusLoading
  const unboundDescription = (
    <span className='break-all'>{t('profile.fields.notBound')}</span>
  )

  return (
    <>
      <SettingsGroup
        title={t('profile.titles.loginAndAuthentication')}
        description={t('profile.tips.passwordAndBoundAccounts')}
      >
        {ready ? (
          <div className={METHOD_GRID_CLASS}>
            {/* 说明较长：操作与标题同行，说明另起一行占满面板宽度。 */}
            <SettingsPanel
              title={t('auth.fields.password')}
              description={t(
                'profile.tips.updateYourPasswordToKeepYourAccountSecure'
              )}
              actions={
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => dialogs.open('password')}
                >
                  {t('profile.fields.changePassword')}
                </Button>
              }
            />

            {bindings.map((binding) => (
              <SettingsPanel
                key={binding.id}
                inline
                title={
                  <BindingLabel
                    icon={binding.icon}
                    label={binding.label}
                    bound={binding.bound}
                  />
                }
                description={
                  binding.bound ? (
                    <span className='break-all'>
                      {binding.value || t('profile.fields.bound')}
                    </span>
                  ) : (
                    unboundDescription
                  )
                }
                actions={bindingAction(binding)}
              />
            ))}
          </div>
        ) : failed ? (
          <SettingsNotice
            tone='danger'
            title={t('profile.errors.failedToLoadProfile')}
          >
            <Button
              variant='link'
              size='sm'
              className='h-auto p-0 align-baseline'
              onClick={onRetry}
            >
              {t('common.actions.retry')}
            </Button>
          </SettingsNotice>
        ) : (
          <TilesSkeleton />
        )}
      </SettingsGroup>

      {profile && (
        <>
          <ChangePasswordDialog
            open={dialogs.isOpen('password')}
            onOpenChange={(open) =>
              open ? dialogs.open('password') : dialogs.close('password')
            }
            username={profile.username}
          />

          <EmailBindDialog
            open={dialogs.isOpen('email')}
            onOpenChange={(open) =>
              open ? dialogs.open('email') : dialogs.close('email')
            }
            currentEmail={profile.email}
            onSuccess={onProfileUpdate}
          />
        </>
      )}
    </>
  )
}
