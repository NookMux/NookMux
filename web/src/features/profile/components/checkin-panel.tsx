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
import { useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuotaWithCurrency } from '@/lib/currency'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { StatusBadge } from '@/components/status-badge'
import { Turnstile } from '@/components/turnstile'
import { SettingsNotice } from '@/features/system-settings/components/settings-notice'
import {
  SettingsPanel,
  SettingsStat,
  SettingsStatGrid,
} from '@/features/system-settings/components/settings-panel'
import { getCheckinStatus, performCheckin } from '../api'
import { buildCalendarDays, getWeekdayLabels, toDateString } from '../lib'
import type { CheckinRecord } from '../types'

interface CheckinPanelProps {
  turnstileEnabled: boolean
  turnstileSiteKey: string
}

/** 每日签到：签到是立即执行的操作，下方是只读的签到统计与月历。 */
export function CheckinPanel({
  turnstileEnabled,
  turnstileSiteKey,
}: CheckinPanelProps) {
  const { t, i18n } = useTranslation()
  const [currentMonth, setCurrentMonth] = useState(
    () => new Date(new Date().getFullYear(), new Date().getMonth(), 1)
  )
  const [checkinLoading, setCheckinLoading] = useState(false)
  const [turnstileOpen, setTurnstileOpen] = useState(false)
  const [turnstileWidgetKey, setTurnstileWidgetKey] = useState(0)

  const currentMonthStr = `${currentMonth.getFullYear()}-${String(currentMonth.getMonth() + 1).padStart(2, '0')}`

  const {
    data: checkinData,
    isLoading,
    refetch,
  } = useQuery({
    queryKey: ['checkin-status', currentMonthStr],
    queryFn: async () => {
      const res = await getCheckinStatus(currentMonthStr)
      if (res.success && res.data) {
        return res.data
      }
      throw new Error(
        res.message || t('profile.errors.failedToFetchCheckinStatus')
      )
    },
    staleTime: 30000,
  })

  const records = checkinData?.stats?.records
  const checkinRecordsMap = useMemo(() => {
    const map: Record<string, number> = {}
    for (const record of records ?? []) {
      map[record.checkin_date] = record.quota_awarded
    }
    return map
  }, [records])

  const monthlyQuota = useMemo(
    () =>
      (records ?? []).reduce(
        (sum: number, record: CheckinRecord) =>
          sum + (record.quota_awarded || 0),
        0
      ),
    [records]
  )

  const [todayString] = useState(() => toDateString(new Date()))
  const checkedToday = checkinData?.stats?.checked_in_today === true
  const todayAward = checkinRecordsMap[todayString]

  const calendarDays = useMemo(
    () =>
      buildCalendarDays(currentMonth.getFullYear(), currentMonth.getMonth()),
    [currentMonth]
  )
  const weekdayLabels = useMemo(
    () => getWeekdayLabels(i18n.language),
    [i18n.language]
  )

  const shouldTriggerTurnstile = useCallback(
    (message?: string) => {
      if (!turnstileEnabled) return false
      if (typeof message !== 'string') return true
      return message.includes('Turnstile')
    },
    [turnstileEnabled]
  )

  const doCheckin = useCallback(
    async (token?: string) => {
      setCheckinLoading(true)
      try {
        const res = await performCheckin(token)
        if (res.success && res.data) {
          toast.success(
            `${t('profile.status.checkInSuccessfulReceived')} ${formatQuotaWithCurrency(res.data.quota_awarded)}`
          )
          refetch()
          setTurnstileOpen(false)
        } else {
          if (!token && shouldTriggerTurnstile(res.message)) {
            if (!turnstileSiteKey) {
              toast.error(
                t('profile.status.turnstileIsEnabledButSiteKeyIsEmpty')
              )
              return
            }
            setTurnstileOpen(true)
            return
          }
          if (token && shouldTriggerTurnstile(res.message)) {
            setTurnstileWidgetKey((value) => value + 1)
          }
          toast.error(res.message || t('profile.status.checkInFailed'))
        }
      } catch (_error) {
        toast.error(t('profile.status.checkInFailed'))
      } finally {
        setCheckinLoading(false)
      }
    },
    [refetch, shouldTriggerTurnstile, t, turnstileSiteKey]
  )

  const shiftMonth = (offset: number) => {
    setCurrentMonth(
      new Date(currentMonth.getFullYear(), currentMonth.getMonth() + offset, 1)
    )
  }

  return (
    <>
      <TooltipProvider delay={100}>
        <SettingsPanel
          title={
            <span className='inline-flex flex-wrap items-center gap-2'>
              {t('profile.fields.dailyCheckIn')}
              {checkedToday && (
                <StatusBadge
                  label={t('profile.fields.checkedIn')}
                  variant='success'
                  copyable={false}
                />
              )}
            </span>
          }
          description={
            checkedToday && todayAward !== undefined
              ? `${t('profile.fields.today')} +${formatQuotaWithCurrency(todayAward)}`
              : t('profile.tips.checkInDailyToReceiveRandomQuotaRewards')
          }
          actions={
            <Button
              size='sm'
              variant={checkedToday ? 'outline' : 'default'}
              onClick={() => void doCheckin()}
              disabled={checkinLoading || checkedToday}
            >
              {checkinLoading
                ? t('common.tips.loading')
                : checkedToday
                  ? t('profile.fields.checkedIn')
                  : t('profile.fields.checkInNow')}
            </Button>
          }
          divided
        >
          {isLoading ? (
            <div className='flex justify-center py-3'>
              <Skeleton className='h-56 w-full max-w-md' />
            </div>
          ) : (
            <>
              <SettingsStatGrid columns={3} className='py-3'>
                <SettingsStat
                  label={t('profile.fields.totalCheckIns')}
                  value={checkinData?.stats?.total_checkins ?? 0}
                />
                <SettingsStat
                  label={t('profile.fields.month')}
                  value={formatQuotaWithCurrency(monthlyQuota, {
                    digitsLarge: 0,
                  })}
                />
                <SettingsStat
                  label={t('profile.fields.totalEarned')}
                  value={formatQuotaWithCurrency(
                    checkinData?.stats?.total_quota ?? 0,
                    { digitsLarge: 0 }
                  )}
                />
              </SettingsStatGrid>

              <div className='mx-auto w-full max-w-2xl py-3'>
                <div className='flex items-center justify-between'>
                  <span className='text-sm font-medium tabular-nums'>
                    {dayjs(currentMonth).format('YYYY-MM')}
                  </span>
                  <div className='flex items-center gap-1'>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon-sm'
                      aria-label={t('common.actions.previous')}
                      onClick={() => shiftMonth(-1)}
                    >
                      <ChevronLeft />
                    </Button>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon-sm'
                      aria-label={t('common.actions.next')}
                      onClick={() => shiftMonth(1)}
                    >
                      <ChevronRight />
                    </Button>
                  </div>
                </div>
                <div className='mt-3 grid grid-cols-7 gap-1'>
                  {weekdayLabels.map((label) => (
                    <div
                      key={label}
                      className='text-muted-foreground flex h-8 items-center justify-center text-xs font-medium'
                    >
                      {label}
                    </div>
                  ))}
                  {calendarDays.map((day, index) => {
                    const dateStr = toDateString(day.date)
                    if (!day.inMonth) {
                      return (
                        <div
                          key={index}
                          className='text-muted-foreground flex h-10 items-center justify-center text-sm tabular-nums'
                        >
                          {day.date.getDate()}
                        </div>
                      )
                    }

                    const isToday = dateStr === todayString
                    const awarded = checkinRecordsMap[dateStr]
                    const checkedIn = awarded !== undefined
                    const dayButton = (
                      <Button
                        key={index}
                        type='button'
                        variant={isToday ? 'default' : 'ghost'}
                        className={cn(
                          'relative flex h-10 w-full flex-col items-center justify-center rounded-lg px-0 text-sm',
                          checkedIn && 'font-semibold'
                        )}
                      >
                        <span className='tabular-nums'>
                          {day.date.getDate()}
                        </span>
                        {checkedIn && !isToday && (
                          <span className='bg-success absolute bottom-1 size-1 rounded-full' />
                        )}
                      </Button>
                    )

                    if (!checkedIn) return dayButton

                    return (
                      <Tooltip key={index}>
                        <TooltipTrigger render={dayButton} />
                        <TooltipContent>
                          <div className='text-xs'>
                            <div className='font-medium'>
                              {t('profile.fields.checkedIn')}
                            </div>
                            <div className='text-muted-foreground mt-0.5'>
                              +{formatQuotaWithCurrency(awarded)}
                            </div>
                          </div>
                        </TooltipContent>
                      </Tooltip>
                    )
                  })}
                </div>
              </div>

              <SettingsNotice>
                {t('profile.tips.rewardsWillBeAddedDirectlyToYourBalance')}
              </SettingsNotice>
              <SettingsNotice>
                {t('profile.tips.onlyCheckInOncePerDay')}
              </SettingsNotice>
            </>
          )}
        </SettingsPanel>
      </TooltipProvider>

      <Dialog
        open={turnstileOpen}
        onOpenChange={(open) => {
          setTurnstileOpen(open)
          if (!open) {
            setTurnstileWidgetKey((value) => value + 1)
          }
        }}
      >
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>
              {t('profile.fields.securityVerification')}
            </DialogTitle>
          </DialogHeader>
          <div className='text-muted-foreground text-sm'>
            {t('profile.tips.pleaseCompleteTheSecurityCheckToContinue')}
          </div>
          <div className='flex justify-center py-4'>
            <Turnstile
              key={turnstileWidgetKey}
              siteKey={turnstileSiteKey}
              onVerify={(token) => {
                void doCheckin(token)
              }}
              onExpire={() => {
                setTurnstileWidgetKey((value) => value + 1)
              }}
            />
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
