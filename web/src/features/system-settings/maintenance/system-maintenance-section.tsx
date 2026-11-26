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
import { Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { useSystemInfo } from '@/hooks/use-system-info'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { SettingsGroup, SettingsRow } from '../components/settings-group'
import { SettingsSection } from '../components/settings-section'

export function SystemMaintenanceSection() {
  const { t } = useTranslation()
  const { version, loading } = useSystemInfo()
  const { copyToClipboard } = useCopyToClipboard()

  return (
    <SettingsSection title={t('systemSettings.titles.maintenance')}>
      <SettingsGroup>
        <SettingsRow label={t('systemSettings.fields.buildIdHash')}>
          {loading && !version ? (
            <Skeleton className='h-5 w-64' />
          ) : (
            <span className='flex min-w-0 items-center gap-1'>
              <code className='font-mono text-sm break-all'>
                {version || t('layout.fields.unknownVersion')}
              </code>
              {version && (
                <Button
                  variant='ghost'
                  size='icon-sm'
                  aria-label={t('common.actions.copyToClipboard')}
                  onClick={() => void copyToClipboard(version)}
                >
                  <Copy />
                </Button>
              )}
            </span>
          )}
        </SettingsRow>
      </SettingsGroup>
    </SettingsSection>
  )
}
