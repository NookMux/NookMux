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
import { useEffect } from 'react'
import { useNavigate, useRouter } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  claimChunkReload,
  resolveHeadingStatus,
  type ReloadMarkerStorage,
} from './lib/general-error'

const FEEDBACK_URL = 'https://github.com/NookMux/NookMux/issues'

type GeneralErrorProps = React.HTMLAttributes<HTMLDivElement> & {
  minimal?: boolean
  error?: unknown
}

// 浏览器禁用存储时读取会话存储会抛错，此时无法留下刷新标记。
function sessionStorageOrNull(): ReloadMarkerStorage | undefined {
  try {
    return window.sessionStorage
  } catch {
    return undefined
  }
}

export function GeneralError({
  className,
  minimal = false,
  error,
}: GeneralErrorProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { history } = useRouter()
  const status = resolveHeadingStatus(error)
  const isRateLimited = status === 429

  // 路由分块加载失败多发于发版后旧分块被替换，当前页面无法原地恢复：同一失败在
  // 标签页内首次出现时自动刷新一次以拉取新资源，再次出现则停留在本页。
  useEffect(() => {
    if (claimChunkReload(sessionStorageOrNull(), error)) {
      window.location.reload()
    }
  }, [error])

  const title = isRateLimited
    ? t('common.fields.tooManyRequests')
    : `${t('common.tips.oopsSomethingWentWrong')} ${`:')`}`
  const description = isRateLimited
    ? t('common.errors.pleaseWaitAMomentBeforeTryingAgain')
    : t('common.tips.pleaseTryAgainLater')

  return (
    <div className={cn('h-svh w-full', className)}>
      <div className='m-auto flex h-full w-full flex-col items-center justify-center gap-2'>
        {!minimal && status !== undefined && (
          <h1 className='text-[7rem] leading-tight font-bold'>{status}</h1>
        )}
        <span className='font-medium'>{title}</span>
        <p className='text-muted-foreground text-center'>
          {t('common.tips.apologizeForTheInconvenience')} <br /> {description}
        </p>
        {!minimal && (
          <p className='text-muted-foreground text-center text-sm'>
            {t('common.tips.ifThisKeepsHappeningPleaseReportItOnGit')}
          </p>
        )}
        {!minimal && (
          <div className='mt-6 flex flex-wrap justify-center gap-4'>
            <Button variant='outline' onClick={() => history.go(-1)}>
              {t('common.fields.goBack')}
            </Button>
            <Button
              variant='outline'
              render={
                <a
                  href={FEEDBACK_URL}
                  target='_blank'
                  rel='noopener noreferrer'
                />
              }
            >
              {t('common.fields.reportAnIssue')}
            </Button>
            <Button onClick={() => navigate({ to: '/' })}>
              {t('layout.actions.backToHome')}
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}
