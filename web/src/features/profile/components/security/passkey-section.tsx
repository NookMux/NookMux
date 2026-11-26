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
import { useCallback, useMemo, useState } from 'react'
import { Loader2, Pencil, Trash } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { StatusBadge } from '@/components/status-badge'
import { usePasskeyManagement } from '@/features/auth/passkey'
import { updatePasskey } from '@/features/auth/passkey/api'
import type { PasskeyCredential } from '@/features/auth/passkey/types'
import {
  SecureVerificationDialog,
  useSecureVerification,
  type VerificationMethod,
  type VerificationMethods,
} from '@/features/auth/secure-verification'
import {
  SettingsGroup,
  SettingsRow,
} from '@/features/system-settings/components/settings-group'
import { SettingsNotice } from '@/features/system-settings/components/settings-notice'
import { RenamePasskeyDialog } from '../dialogs/rename-passkey-dialog'
import { SectionSkeleton } from './section-skeleton'

const TITLE_BADGE_CLASS = 'inline-flex flex-wrap items-center gap-2'

/** Passkey 登录：已注册的 Passkey 每个一行，标题行右侧是添加按钮。 */
export function PasskeySection() {
  const { t } = useTranslation()
  const [removeTarget, setRemoveTarget] = useState<PasskeyCredential | null>(
    null
  )
  const [renameTarget, setRenameTarget] = useState<PasskeyCredential | null>(
    null
  )
  const [renameName, setRenameName] = useState('')
  const [renaming, setRenaming] = useState(false)
  const [restrictedMethod, setRestrictedMethod] =
    useState<VerificationMethod | null>(null)

  const {
    status,
    loading,
    registering,
    removing,
    supported,
    fetchStatus,
    register,
    remove,
  } = usePasskeyManagement()

  const {
    open: verificationOpen,
    setOpen: setVerificationOpen,
    methods: verificationMethods,
    state: verificationState,
    startVerification,
    executeVerification,
    cancel: cancelVerification,
    setCode,
    switchMethod,
    fetchVerificationMethods,
  } = useSecureVerification({
    onSuccess: () => {
      setRestrictedMethod(null)
    },
  })

  const dialogMethods = useMemo<VerificationMethods>(() => {
    if (!restrictedMethod) return verificationMethods
    return {
      ...verificationMethods,
      has2FA: restrictedMethod === '2fa' && verificationMethods.has2FA,
      hasPasskey:
        restrictedMethod === 'passkey' && verificationMethods.hasPasskey,
    }
  }, [restrictedMethod, verificationMethods])

  const handleRegister = useCallback(async () => {
    if (!supported) {
      toast.info(t('auth.tips.deviceDoesNotSupportPasskey'))
      return
    }

    const methods = await fetchVerificationMethods()
    if (!methods.has2FA) {
      await register()
      return
    }

    setRestrictedMethod('2fa')
    await startVerification(register, {
      preferredMethod: '2fa',
      title: t('profile.fields.securityVerification'),
      description: t(
        'profile.actions.confirmYourIdentityWithTwoFactorAuthenticationBeforeRegistering'
      ),
    })
  }, [fetchVerificationMethods, register, startVerification, supported, t])

  const handleRemove = useCallback(
    async (id?: number) => {
      const methods = await fetchVerificationMethods()
      const required: VerificationMethod | null = methods.has2FA
        ? '2fa'
        : methods.hasPasskey
          ? 'passkey'
          : null

      if (!required) {
        toast.error(
          t(
            'auth.errors.pleaseEnableTwoFactorAuthenticationOrPasskeyBeforeProceeding'
          )
        )
        return
      }

      if (required === 'passkey' && !methods.passkeySupported) {
        toast.info(t('auth.tips.deviceDoesNotSupportPasskey'))
        return
      }

      setRemoveTarget(null)
      setRestrictedMethod(required)
      await startVerification(() => remove(id), {
        preferredMethod: required,
        title: t('profile.fields.securityVerification'),
        description: t(
          'profile.actions.confirmYourIdentityBeforeRemovingThisPasskeyFromYour'
        ),
      })
    },
    [fetchVerificationMethods, remove, startVerification, t]
  )

  const openRename = useCallback((credential: PasskeyCredential) => {
    setRenameName(credential.device_name || '')
    setRenameTarget(credential)
  }, [])

  const closeRename = useCallback(() => {
    setRenameTarget(null)
  }, [])

  const handleRenameSubmit = useCallback(async () => {
    if (!renameTarget) return
    setRenaming(true)
    try {
      const res = await updatePasskey(renameTarget.id, renameName)
      if (!res.success) {
        toast.error(res.message || t('profile.errors.failedToUpdateDeviceName'))
        return
      }
      toast.success(t('profile.status.deviceNameUpdated'))
      setRenameTarget(null)
      await fetchStatus()
    } catch (_error) {
      toast.error(t('profile.errors.failedToUpdateDeviceName'))
    } finally {
      setRenaming(false)
    }
  }, [fetchStatus, renameName, renameTarget, t])

  const handleVerificationCancel = useCallback(() => {
    setRestrictedMethod(null)
    cancelVerification()
  }, [cancelVerification])

  const handleVerificationOpenChange = useCallback(
    (next: boolean) => {
      if (!next) {
        setRestrictedMethod(null)
      }
      setVerificationOpen(next)
    },
    [setVerificationOpen]
  )

  const handleDialogVerify = useCallback(
    async (method: VerificationMethod, code?: string) => {
      try {
        await executeVerification(method, code)
      } catch {
        // 验证失败由 useSecureVerification 弹出提示。
      }
    },
    [executeVerification]
  )

  const passkeys = status?.passkeys ?? []
  const count = status?.count ?? 0
  const maxPasskeys = status?.max_passkeys || 1
  const canAddMore = count < maxPasskeys
  const reachedLimit = !canAddMore && count > 0

  return (
    <>
      <SettingsGroup
        title={t('profile.fields.passkeyLogin')}
        description={t(
          'profile.actions.usePasskeyToSignInWithoutEnteringYourPassword'
        )}
        actions={
          canAddMore ? (
            <Button
              variant='outline'
              size='sm'
              onClick={() => void handleRegister()}
              disabled={!supported || registering}
            >
              {registering && (
                <Loader2 className='animate-spin' data-icon='inline-start' />
              )}
              {count === 0
                ? t('profile.actions.enablePasskey')
                : t('profile.actions.addAnotherPasskey')}
            </Button>
          ) : undefined
        }
      >
        {loading ? (
          <SectionSkeleton rows={1} />
        ) : passkeys.length === 0 ? (
          <p className='text-muted-foreground py-6 text-center text-sm'>
            {t('profile.status.noPasskeys')}
          </p>
        ) : (
          passkeys.map((credential) => (
            <SettingsRow
              key={credential.id}
              label={
                <span className={TITLE_BADGE_CLASS}>
                  {credential.device_name || t('profile.fields.unnamedDevice')}
                  <StatusBadge
                    label={
                      credential.attachment === 'platform'
                        ? t('profile.fields.builtIn')
                        : credential.attachment === 'cross-platform'
                          ? t('profile.fields.external')
                          : t('channels.fields.unknown')
                    }
                    variant='neutral'
                    copyable={false}
                  />
                  {credential.backup_eligible && (
                    <StatusBadge
                      label={
                        credential.backup_state
                          ? t('profile.fields.backedUp')
                          : t('profile.fields.notBackedUp')
                      }
                      variant={credential.backup_state ? 'success' : 'warning'}
                      copyable={false}
                    />
                  )}
                </span>
              }
              description={
                credential.last_used_at
                  ? t('profile.tips.lastUsedAt', {
                      time: dayjs(credential.last_used_at).fromNow(),
                    })
                  : t('profile.status.notUsedYet')
              }
              trailing
            >
              <div className='flex items-center gap-1'>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  aria-label={t('profile.fields.renameDevice')}
                  onClick={() => openRename(credential)}
                  disabled={removing}
                >
                  <Pencil />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  className='text-destructive hover:text-destructive'
                  aria-label={t('common.actions.remove')}
                  onClick={() => setRemoveTarget(credential)}
                  disabled={removing}
                >
                  <Trash />
                </Button>
              </div>
            </SettingsRow>
          ))
        )}

        {!loading && !supported && count === 0 && (
          <SettingsNotice
            tone='warning'
            title={t('profile.tips.passkeyNotSupportedOnThisDevice')}
          >
            {t(
              'profile.actions.useACompatibleBrowserOrDeviceWithBiometricAuthentication'
            )}
          </SettingsNotice>
        )}

        {!loading && reachedLimit && (
          <SettingsNotice>
            {t('profile.tips.haveReachedTheMaximumNumberOfPasskeys')} ({count}/
            {maxPasskeys})
          </SettingsNotice>
        )}
      </SettingsGroup>

      <RenamePasskeyDialog
        open={renameTarget !== null}
        onOpenChange={(open) => {
          if (!renaming && !open) closeRename()
        }}
        name={renameName}
        onNameChange={setRenameName}
        submitting={renaming}
        onSubmit={handleRenameSubmit}
      />

      <ConfirmDialog
        open={removeTarget !== null}
        onOpenChange={(open) => {
          if (!open && !removing) setRemoveTarget(null)
        }}
        title={t('profile.actions.removePasskey')}
        desc={t('profile.tips.thisPasskeyWillBeRemovedFromYourAccountYouCan')}
        confirmText={t('common.actions.remove')}
        destructive
        isLoading={removing}
        handleConfirm={() => {
          if (removeTarget) void handleRemove(removeTarget.id)
        }}
      />

      <SecureVerificationDialog
        open={verificationOpen}
        onOpenChange={handleVerificationOpenChange}
        methods={dialogMethods}
        state={verificationState}
        onVerify={handleDialogVerify}
        onCancel={handleVerificationCancel}
        onCodeChange={setCode}
        onMethodChange={switchMethod}
      />
    </>
  )
}
